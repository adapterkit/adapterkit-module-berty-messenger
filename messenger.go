package messenger

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"berty.tech/berty/v2/go/pkg/messengertypes"
	"berty.tech/berty/v2/go/pkg/protocoltypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func New(nodeAddr string) MessengerSvcServer {
	conn, err := grpc.Dial(nodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	client := messengertypes.NewMessengerServiceClient(conn)
	get, err := client.AccountGet(context.Background(), &messengertypes.AccountGet_Request{})
	if err != nil {
		panic(err)
	}

	log.Println("pubkey:", get.Account.GetPublicKey())
	return &service{
		NodeAddr: nodeAddr,
		pubKey:   get.GetAccount().GetPublicKey(),
	}
}

type service struct {
	UnimplementedMessengerSvcServer

	NodeAddr string
	pubKey   string
}

func (s *service) GetInvitationLink(_ context.Context, req *GetInvitationLinkReq) (*GetInvitationLinkRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	client := messengertypes.NewMessengerServiceClient(conn)
	infos, err := client.InstanceShareableBertyID(context.Background(), &messengertypes.InstanceShareableBertyID_Request{})
	if err != nil {
		return nil, err
	}

	return &GetInvitationLinkRes{Link: infos.WebURL}, nil
}

func (s *service) GetContactRequests(req *GetContactRequestsReq, stream MessengerSvc_GetContactRequestsServer) error {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	protocolClient := protocoltypes.NewProtocolServiceClient(conn)
	config, err := protocolClient.InstanceGetConfiguration(context.Background(), &protocoltypes.InstanceGetConfiguration_Request{})
	if err != nil {
		return fmt.Errorf("get config error: %w", err)
	}

	cl, err := protocolClient.GroupMetadataList(context.Background(), &protocoltypes.GroupMetadataList_Request{
		GroupPK: config.AccountGroupPK,
	})
	if err != nil {
		return fmt.Errorf("list error: %w", err)
	}

	var contactRequests []*GetContactRequestsRes_ContactRequest

	for {
		meta, err := cl.Recv()
		if err == io.EOF {
			break
		}
		if meta != nil && meta.Metadata != nil {
			switch meta.Metadata.EventType {
			case protocoltypes.EventTypeAccountContactRequestIncomingReceived:
				casted := &protocoltypes.AccountContactRequestReceived{}
				if err := casted.Unmarshal(meta.Event); err != nil {
					return fmt.Errorf("unmarshal error: %w", err)
				}

				if req.Store {
					path := "."
					if req.StoreDir != "" {
						path = req.StoreDir
					}

					strBuf := base64.StdEncoding.EncodeToString(casted.ContactPK)

					err := os.WriteFile(filepath.Join(path, fmt.Sprintf("contact-request-%s.berty", casted.ContactMetadata)), []byte(strBuf), 0644)
					if err != nil {
						return err
					}
				}
				contactRequests = append(contactRequests, &GetContactRequestsRes_ContactRequest{
					PublicKey: casted.ContactPK,
					Name:      string(casted.ContactMetadata),
				})
				err := stream.Send(&GetContactRequestsRes{
					ContactRequests: contactRequests,
				})
				if err != nil {
					return err
				}
			case protocoltypes.EventTypeAccountContactRequestIncomingAccepted:
				casted := &protocoltypes.AccountContactRequestAccepted{}
				if err := casted.Unmarshal(meta.Event); err != nil {
					return fmt.Errorf("unmarshal error: %w", err)
				}

				contactRequests = RemovePartialOccurrence(contactRequests, func(request *GetContactRequestsRes_ContactRequest) bool {
					return bytes.Compare(request.PublicKey, casted.ContactPK) == 0
				})
				err := stream.Send(&GetContactRequestsRes{
					ContactRequests: contactRequests,
				})
				if err != nil {
					return err
				}
			case protocoltypes.EventTypeAccountContactRequestIncomingDiscarded:
				casted := &protocoltypes.AccountContactRequestDiscarded{}
				if err := casted.Unmarshal(meta.Event); err != nil {
					return fmt.Errorf("unmarshal error: %w", err)
				}
			}
		}
	}

	return nil
}

func (s *service) AcceptContactRequest(_ context.Context, req *AcceptContactRequestReq) (*AcceptContactRequestRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	pubkey, err := os.ReadFile(req.PathToPubkey)
	if err != nil {
		return nil, err
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(string(pubkey))
	if err != nil {
		return nil, err
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	_, err = client.ContactRequestAccept(context.Background(), &protocoltypes.ContactRequestAccept_Request{
		ContactPK: decodedPubkey,
	})
	if err != nil {
		return nil, err
	}

	err = os.Remove(req.PathToPubkey)
	if err != nil {
		return nil, err
	}
	return &AcceptContactRequestRes{Success: true}, nil
}

func (s *service) SendMessage(_ context.Context, req *SendMessageReq) (*SendMessageRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	pubkey, err := os.ReadFile(req.PathToPubkey)
	if err != nil {
		return nil, err
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(string(pubkey))
	if err != nil {
		return nil, err
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	group, err := client.GroupInfo(context.Background(), &protocoltypes.GroupInfo_Request{
		ContactPK: decodedPubkey,
	})
	if err != nil {
		return nil, fmt.Errorf("group info error: %w", err)
	}

	_, err = client.ActivateGroup(context.Background(), &protocoltypes.ActivateGroup_Request{
		GroupPK: group.Group.PublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("activate group error: %w", err)
	}

	_, err = client.AppMessageSend(context.Background(), &protocoltypes.AppMessageSend_Request{
		GroupPK: group.Group.PublicKey,
		Payload: []byte(req.Message),
	})
	if err != nil {
		return nil, fmt.Errorf("send message error: %w", err)
	}
	return &SendMessageRes{Success: true}, nil
}

func (s *service) ListMessages(req *ListMessagesReq, stream MessengerSvc_ListMessagesServer) error {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	pubkey, err := os.ReadFile(req.PathToPubkey)
	if err != nil {
		return fmt.Errorf("read file error: %w", err)
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(string(pubkey))
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	group, err := client.GroupInfo(context.Background(), &protocoltypes.GroupInfo_Request{
		ContactPK: decodedPubkey,
	})
	if err != nil {
		return fmt.Errorf("group info error: %w", err)
	}
	list, err := client.GroupMessageList(context.Background(), &protocoltypes.GroupMessageList_Request{
		GroupPK: group.Group.PublicKey,
	})
	if err != nil {
		return err
	}

	for {
		msg, err := list.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("recv error: %w", err)
		}

		err = stream.Send(&ListMessagesRes{
			Message: string(msg.GetMessage()),
		})
	}
}

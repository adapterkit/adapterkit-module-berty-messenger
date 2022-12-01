package messenger

import (
	"bytes"
	"context"
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
		return nil
	}
	client := messengertypes.NewMessengerServiceClient(conn)
	get, err := client.AccountGet(context.Background(), &messengertypes.AccountGet_Request{})
	if err != nil {
		return nil
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

					err := os.WriteFile(filepath.Join(path, fmt.Sprintf("contact-request-%s.berty", casted.ContactMetadata)), casted.ContactPK, 0644)
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

	client := messengertypes.NewMessengerServiceClient(conn)
	_, err = client.ContactAccept(context.Background(), &messengertypes.ContactAccept_Request{
		PublicKey: string(pubkey),
	})
	if err != nil {
		return nil, err
	}

	return &AcceptContactRequestRes{Success: true}, nil
}

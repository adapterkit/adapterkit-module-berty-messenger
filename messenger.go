package messenger

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"

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

func (s *service) GetContactPubkey(_ context.Context, _ *GetContactPubkeyReq) (*GetContactPubkeyRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	config, err := client.InstanceGetConfiguration(context.Background(), &protocoltypes.InstanceGetConfiguration_Request{})
	if err != nil {
		return nil, err
	}

	ref, err := client.ContactRequestReference(context.Background(), &protocoltypes.ContactRequestReference_Request{})
	if err != nil {
		return nil, fmt.Errorf("ref error: %w", err)
	}

	b64PK := base64.StdEncoding.EncodeToString(config.AccountPK)
	b64RdvSeed := base64.StdEncoding.EncodeToString(ref.PublicRendezvousSeed)

	return &GetContactPubkeyRes{
		Pubkey:  b64PK,
		RdvSeed: b64RdvSeed,
	}, nil
}

func (s *service) GetContactRequests(_ context.Context, _ *GetContactRequestsReq) (*GetContactRequestsRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	client := protocoltypes.NewProtocolServiceClient(conn)
	config, err := client.InstanceGetConfiguration(context.Background(), &protocoltypes.InstanceGetConfiguration_Request{})
	if err != nil {
		return nil, fmt.Errorf("get config error: %w", err)
	}

	cl, err := client.GroupMetadataList(context.Background(), &protocoltypes.GroupMetadataList_Request{
		GroupPK:  config.AccountGroupPK,
		UntilNow: true,
	})
	if err != nil {
		return nil, fmt.Errorf("list error: %w", err)
	}

	var contactRequests []*GetContactRequestsRes_ContactRequest

	for {
		meta, err := cl.Recv()
		if err == io.EOF {
			return &GetContactRequestsRes{ContactRequests: contactRequests}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("recv error: %w", err)
		}

		if meta == nil || meta.Metadata == nil {
			continue
		}
		switch meta.Metadata.EventType {
		case protocoltypes.EventTypeAccountContactRequestIncomingReceived:
			casted := &protocoltypes.AccountContactRequestReceived{}
			if err := casted.Unmarshal(meta.Event); err != nil {
				return nil, fmt.Errorf("unmarshal error: %w", err)
			}
			contactRequests = append(contactRequests, &GetContactRequestsRes_ContactRequest{
				PublicKey: base64.StdEncoding.EncodeToString(casted.ContactPK),
				Name:      string(casted.ContactMetadata),
			})
		case protocoltypes.EventTypeAccountContactRequestIncomingAccepted:
			casted := &protocoltypes.AccountContactRequestAccepted{}
			if err := casted.Unmarshal(meta.Event); err != nil {
				return nil, fmt.Errorf("unmarshal error: %w", err)
			}

			contactRequests = RemoveMatch(contactRequests, func(request *GetContactRequestsRes_ContactRequest) bool {
				return request.PublicKey == base64.StdEncoding.EncodeToString(casted.ContactPK)
			})
		case protocoltypes.EventTypeAccountContactRequestIncomingDiscarded:
			casted := &protocoltypes.AccountContactRequestDiscarded{}
			if err := casted.Unmarshal(meta.Event); err != nil {
				return nil, fmt.Errorf("unmarshal error: %w", err)
			}

			contactRequests = RemoveMatch(contactRequests, func(request *GetContactRequestsRes_ContactRequest) bool {
				return request.PublicKey == base64.StdEncoding.EncodeToString(casted.ContactPK)
			})
		}
	}
}

func (s *service) SendContactRequest(_ context.Context, req *SendContactRequestReq) (*SendContactRequestRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	contactPK, err := base64.StdEncoding.DecodeString(req.Pubkey)
	if err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	publicRdvSeed, err := base64.StdEncoding.DecodeString(req.RdvSeed)
	if err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	if req.Name == "" {
		req.Name = "Anonymous"
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	_, err = client.ContactRequestSend(context.Background(), &protocoltypes.ContactRequestSend_Request{
		Contact: &protocoltypes.ShareableContact{
			PK:                   contactPK,
			PublicRendezvousSeed: publicRdvSeed,
		},
		OwnMetadata: []byte(req.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("send error: %w", err)
	}

	return &SendContactRequestRes{Success: true}, nil
}

func (s *service) AcceptContactRequest(_ context.Context, req *AcceptContactRequestReq) (*AcceptContactRequestRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(req.Pubkey)
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

	return &AcceptContactRequestRes{Success: true}, nil
}

func (s *service) SendMessage(_ context.Context, req *SendMessageReq) (*SendMessageRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(req.Pubkey)
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

	//_, err = client.ActivateGroup(context.Background(), &protocoltypes.ActivateGroup_Request{
	//	GroupPK: group.Group.PublicKey,
	//})
	//if err != nil {
	//	return nil, fmt.Errorf("activate group error: %w", err)
	//}

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

	decodedPubkey, err := base64.StdEncoding.DecodeString(req.Pubkey)
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
		GroupPK:  group.Group.PublicKey,
		UntilNow: true,
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
			Message: string(msg.GetMessage()), // doesn't work with messenger layer cause non-utf8 chars are not supported
		})
	}
}

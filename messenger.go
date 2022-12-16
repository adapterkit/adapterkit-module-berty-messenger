package messenger

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"berty.tech/berty/v2/go/pkg/protocoltypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func New(nodeAddr string) MessengerSvcServer {
	_, err := grpc.Dial(nodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	return &service{
		NodeAddr: nodeAddr,
	}
}

type service struct {
	UnimplementedMessengerSvcServer

	NodeAddr string
}

func (s *service) GetContactPubkey(ctx context.Context, _ *GetContactPubkeyReq) (*GetContactPubkeyRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	config, err := client.InstanceGetConfiguration(ctx, &protocoltypes.InstanceGetConfiguration_Request{})
	if err != nil {
		return nil, err
	}

	ref, err := client.ContactRequestReference(ctx, &protocoltypes.ContactRequestReference_Request{})
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

func (s *service) GetContactRequests(ctx context.Context, _ *GetContactRequestsReq) (*GetContactRequestsRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	client := protocoltypes.NewProtocolServiceClient(conn)
	config, err := client.InstanceGetConfiguration(ctx, &protocoltypes.InstanceGetConfiguration_Request{})
	if err != nil {
		return nil, fmt.Errorf("get config error: %w", err)
	}

	cl, err := client.GroupMetadataList(ctx, &protocoltypes.GroupMetadataList_Request{
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

func (s *service) SendContactRequest(ctx context.Context, req *SendContactRequestReq) (*SendContactRequestRes, error) {
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
	_, err = client.ContactRequestSend(ctx, &protocoltypes.ContactRequestSend_Request{
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

func (s *service) AcceptContactRequest(ctx context.Context, req *AcceptContactRequestReq) (*AcceptContactRequestRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(req.Pubkey)
	if err != nil {
		return nil, err
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	_, err = client.ContactRequestAccept(ctx, &protocoltypes.ContactRequestAccept_Request{
		ContactPK: decodedPubkey,
	})
	if err != nil {
		return nil, err
	}

	return &AcceptContactRequestRes{Success: true}, nil
}

func (s *service) SendMessage(ctx context.Context, req *SendMessageReq) (*SendMessageRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(req.Pubkey)
	if err != nil {
		return nil, err
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	group := &protocoltypes.GroupInfo_Reply{}

	if !req.IsContact {
		group, err = client.GroupInfo(ctx, &protocoltypes.GroupInfo_Request{
			GroupPK: decodedPubkey,
		})
	} else {
		group, err = client.GroupInfo(ctx, &protocoltypes.GroupInfo_Request{
			ContactPK: decodedPubkey,
		})
		if err != nil {
			return nil, fmt.Errorf("contact group info error: %w", err)
		}
	}
	//_, err = client.ActivateGroup(context.Background(), &protocoltypes.ActivateGroup_Request{
	//	GroupPK: group.Group.PublicKey,
	//})
	//if err != nil {
	//	return nil, fmt.Errorf("activate group error: %w", err)
	//}

	_, err = client.AppMessageSend(ctx, &protocoltypes.AppMessageSend_Request{
		GroupPK: group.Group.PublicKey,
		Payload: []byte(req.Message),
	})
	if err != nil {
		return nil, fmt.Errorf("send message error: %w", err)
	}
	return &SendMessageRes{Success: true}, nil
}

func (s *service) ListMessages(req *ListMessagesReq, stream MessengerSvc_ListMessagesServer) error {
	ctx := stream.Context()
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	decodedPubkey, err := base64.StdEncoding.DecodeString(req.Pubkey)
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	group := &protocoltypes.GroupInfo_Reply{}

	if !req.IsContact {
		group, err = client.GroupInfo(ctx, &protocoltypes.GroupInfo_Request{
			GroupPK: decodedPubkey,
		})
	} else {
		group, err = client.GroupInfo(ctx, &protocoltypes.GroupInfo_Request{
			ContactPK: decodedPubkey,
		})
		if err != nil {
			return fmt.Errorf("contact group info error: %w", err)
		}
	}

	if err != nil {
		return fmt.Errorf("group info error: %w", err)
	}
	list, err := client.GroupMessageList(context.Background(), &protocoltypes.GroupMessageList_Request{
		GroupPK:      group.Group.PublicKey,
		UntilNow:     true,
		ReverseOrder: true,
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

		var id = "invalid"
		//if len(msg.Headers.DevicePK) > 6 {
		//	id = string(msg.Headers.DevicePK)[:5]
		//}

		fmt.Println(string(msg.Headers.DevicePK))
		err = stream.Send(&ListMessagesRes{
			Id:      id,
			Message: string(msg.GetMessage()), // doesn't work with messenger layer cause non-utf8 chars are not supported
		})
		if err != nil {
			return fmt.Errorf("send error: %w", err)
		}
	}
}

func (s *service) CreateGroup(ctx context.Context, req *CreateGroupReq) (*CreateGroupRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	gpk, err := client.MultiMemberGroupCreate(ctx, &protocoltypes.MultiMemberGroupCreate_Request{})
	if err != nil {
		return nil, fmt.Errorf("create g error: %w", err)
	}

	{
		_, err := client.ActivateGroup(ctx, &protocoltypes.ActivateGroup_Request{
			GroupPK: gpk.GroupPK,
		})
		if err != nil {
			return nil, fmt.Errorf("activate group error: %w", err)
		}
	}

	g, err := client.MultiMemberGroupInvitationCreate(ctx, &protocoltypes.MultiMemberGroupInvitationCreate_Request{
		GroupPK: gpk.GroupPK,
	})
	if err != nil {
		return nil, fmt.Errorf("create invite error: %w", err)
	}

	group := g.Group

	inv, err := group.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	b64Inv := base64.StdEncoding.EncodeToString(inv)
	b64Gpk := base64.StdEncoding.EncodeToString(group.PublicKey)

	return &CreateGroupRes{
		GroupPk:         b64Gpk,
		GroupInvitation: b64Inv,
	}, nil
}

func (s *service) JoinGroup(ctx context.Context, req *JoinGroupReq) (*JoinGroupRes, error) {
	conn, err := grpc.Dial(s.NodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	decodedInv, err := base64.StdEncoding.DecodeString(req.GroupInvitation)
	if err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	group := &protocoltypes.Group{}
	err = group.Unmarshal(decodedInv)
	if err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	client := protocoltypes.NewProtocolServiceClient(conn)
	_, err = client.MultiMemberGroupJoin(ctx, &protocoltypes.MultiMemberGroupJoin_Request{
		Group: group,
	})
	if err != nil {
		return nil, fmt.Errorf("join error: %w", err)
	}

	_, err = client.ActivateGroup(ctx, &protocoltypes.ActivateGroup_Request{
		GroupPK: group.PublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("activate group error: %w", err)
	}

	return &JoinGroupRes{Success: true}, nil
}

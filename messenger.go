package messenger

import (
	"context"
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
	messengertypes.UnimplementedMessengerServiceServer

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
	conn, err := grpc.Dial(req.NodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	for {
		meta, err := cl.Recv()
		if err == io.EOF {
			break
		}
		if meta == nil {
			//fmt.Println(i)
			//i++
			continue
			//log.Println(fmt.Errorf("recv error: %w", err))
		} else {
			if meta.Metadata.EventType == protocoltypes.EventTypeAccountContactRequestIncomingReceived {
				casted := &protocoltypes.AccountContactRequestReceived{}
				if err := casted.Unmarshal(meta.Event); err != nil {
					return fmt.Errorf("unmarshal error: %w", err)
				}
				err := stream.Send(&GetContactRequestsRes{
					ContactRequests: &GetContactRequestsRes_ContactRequest{
						Name:      string(casted.ContactMetadata),
						PublicKey: casted.ContactPK,
					},
				})
				if err != nil {
					return err
				}
			}
			//log.Println("meta:", meta)
		}
	}

	return nil
}

func (s *service) mustEmbedUnimplementedMessengerSvcServer() {
	//TODO implement me
	panic("implement me")
}

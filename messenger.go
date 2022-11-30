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

func (s *service) GetContactRequests(ctx context.Context, req *GetContactRequestsReq) (*GetContactRequestsRes, error) {
	conn, err := grpc.Dial(req.NodeAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	p := protocoltypes.NewProtocolServiceClient(conn)
	cl, err := p.GroupMetadataList(context.Background(), &protocoltypes.GroupMetadataList_Request{
		GroupPK:  []byte(s.pubKey),
		UntilNow: true,
	})
	if err != nil {
		return nil, err
	}

	var res []*GetContactRequestsRes_ContactRequest

	for {
		meta, err := cl.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		res = append(res, &GetContactRequestsRes_ContactRequest{
			Name: string(meta.Metadata.Payload),
		})
		log.Println("meta:", meta)
	}
	return &GetContactRequestsRes{ContactRequests: res}, nil
}

func (s *service) mustEmbedUnimplementedMessengerSvcServer() {
	//TODO implement me
	panic("implement me")
}

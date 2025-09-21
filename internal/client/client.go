package client

import (
	"context"
	"fmt"

	pb "github.com/nlewo/comin/internal/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	conn        *grpc.ClientConn
	cominClient pb.CominClient
}

type ClientOpts struct {
	Port int
	Host string
}

func New(clientOpts ClientOpts) (c Client, err error) {
	serverAddr := fmt.Sprintf("%s:%d", clientOpts.Host, clientOpts.Port)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	c.conn, err = grpc.NewClient(serverAddr, opts...)
	if err != nil {
		return
	}
	c.cominClient = pb.NewCominClient(c.conn)
	return
}

func (c Client) Close() {
	c.conn.Close()
}

func (c Client) GetManagerState() (state *pb.State, err error) {
	return c.cominClient.GetState(context.Background(), &emptypb.Empty{})
}

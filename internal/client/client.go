package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	conn        *grpc.ClientConn
	cominClient protobuf.CominClient
}

type ClientOpts struct {
	UnixSocketPath string
}

func New(clientOpts ClientOpts) (c Client, err error) {
	serverAddr := fmt.Sprintf("unix://%s", clientOpts.UnixSocketPath)
	logrus.Infof("client: connection to %s", serverAddr)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	c.conn, err = grpc.NewClient(serverAddr, opts...)
	if err != nil {
		return
	}
	c.cominClient = protobuf.NewCominClient(c.conn)
	return
}

func (c Client) Close() {
	c.conn.Close() // nolint: errcheck
}

func (c Client) GetManagerState() (state *protobuf.State, err error) {
	return c.cominClient.GetState(context.Background(), &emptypb.Empty{})
}

func (c Client) Events(handler func(*protobuf.Event) error) error {
	for {
		stream, err := c.cominClient.Events(context.Background(), &emptypb.Empty{})
		if err != nil {
			logrus.Infof("failed to connect to the stream: %s", err)
			time.Sleep(time.Second)
			continue
		}
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				logrus.Infof("server closed stream: %s", err)
				break
			}
			if err != nil {
				logrus.Infof("failed to receive from the stream: %s", err)
				break
			}
			if err := handler(event); err != nil {
				return err
			}
		}
	}
}

func (c Client) Fetch() {
	c.cominClient.Fetch(context.Background(), &emptypb.Empty{}) // nolint: errcheck
}
func (c Client) Suspend() error {
	_, err := c.cominClient.Suspend(context.Background(), &emptypb.Empty{})
	return err
}
func (c Client) Resume() error {
	_, err := c.cominClient.Resume(context.Background(), &emptypb.Empty{})
	return err
}
func (c Client) SwitchDeploymentLatest() error {
	_, err := c.cominClient.SwitchDeploymentLatest(context.Background(), &emptypb.Empty{})
	return err
}

func (c Client) Confirm(generationUUID, for_ string) error {
	_, err := c.cominClient.Confirm(context.Background(), &protobuf.ConfirmRequest{
		GenerationUuid: generationUUID, For: for_})
	return err
}

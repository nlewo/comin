package server

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
)

type cominServer struct {
	protobuf.CominServer
	manager        *manager.Manager
	broker         *broker.Broker
	unixSocketPath string
}

func (s *cominServer) Events(_ *emptypb.Empty, stream grpc.ServerStreamingServer[protobuf.Event]) error {
	logrus.Infof("server: start to stream events")

	subscriber := s.broker.Subscribe()
	for {
		event := <-subscriber
		if err := stream.Send(event); err != nil {
			logrus.Infof("server: failed to send stream: %s", err)
			s.broker.Unsubscribe(subscriber)
			return err
		}
	}
}

func (s *cominServer) GetState(ctx context.Context, empty *emptypb.Empty) (*protobuf.State, error) {
	return s.manager.GetState(), nil
}

func (s *cominServer) Fetch(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	fetcher := s.manager.GetState().Fetcher
	remotes := make([]string, 0)
	for _, r := range fetcher.RepositoryStatus.Remotes {
		remotes = append(remotes, r.Name)
	}
	s.manager.Fetcher.TriggerFetch(remotes)
	return nil, nil
}

func (s *cominServer) Suspend(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.manager.Suspend()
	if err != nil {
		st := status.New(codes.Aborted, err.Error())
		err = st.Err()
	}
	return nil, err
}
func (s *cominServer) Resume(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.manager.Resume(ctx)
	if err != nil {
		st := status.New(codes.Aborted, err.Error())
		err = st.Err()
	}
	return nil, err
}

func (s *cominServer) Confirm(ctx context.Context, req *protobuf.ConfirmRequest) (*emptypb.Empty, error) {
	switch req.For {
	case "build":
		s.manager.BuildConfirmer.Confirm(req.GenerationUuid)
	case "deploy":
		s.manager.DeployConfirmer.Confirm(req.GenerationUuid)
	case "all":
		s.manager.BuildConfirmer.Confirm(req.GenerationUuid)
		s.manager.DeployConfirmer.Confirm(req.GenerationUuid)
	}
	return nil, nil
}

func (c *cominServer) Start() {
	go func() {
		if err := os.RemoveAll(c.unixSocketPath); err != nil {
			log.Fatalf("server: failed to remove existing socket file: %s", err)
		}
		logrus.Infof("server: GRPC server starts listening on the Unix socket %s", c.unixSocketPath)
		lis, err := net.Listen("unix", c.unixSocketPath)
		if err != nil {
			log.Fatalf("server: failed to listen on %s: %s", c.unixSocketPath, err)
		}
		if err := os.Chmod(c.unixSocketPath, 0777); err != nil {
			log.Fatalf("server: failed to change socket permissions: %s", err)
		}
		var opts []grpc.ServerOption
		grpcServer := grpc.NewServer(opts...)
		protobuf.RegisterCominServer(grpcServer, c)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("server: failed to serve: %s", err)
		}
	}()
}

func New(broker *broker.Broker, manager *manager.Manager, unixSocketPath string) *cominServer {
	return &cominServer{
		manager:        manager,
		unixSocketPath: unixSocketPath,
		broker:         broker,
	}
}

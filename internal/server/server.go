package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/nlewo/comin/internal/manager"
	pb "github.com/nlewo/comin/internal/protobuf"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
)

type cominServer struct {
	pb.CominServer
	manager *manager.Manager
}

func (s *cominServer) GetState(ctx context.Context, empty *emptypb.Empty) (*pb.State, error) {
	return s.manager.GetState(), nil

}

func (c *cominServer) Start() {
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", 14242))
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		var opts []grpc.ServerOption
		grpcServer := grpc.NewServer(opts...)
		pb.RegisterCominServer(grpcServer, c)
		grpcServer.Serve(lis)
	}()
}

func New(manager *manager.Manager) *cominServer {
	return &cominServer{
		manager: manager,
	}
}

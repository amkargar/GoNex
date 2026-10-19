package usersgrpc

import (
	"context"
	pb "github.com/amkargar/gonex/contracts/gen/go/identity/v1"
	"github.com/amkargar/gonex/examples/microservices/services/identity/modules/users"
	rpc "github.com/amkargar/gonex/grpc"
)

type Server struct {
	pb.UnimplementedIdentityServiceServer
	Service users.Service
}

func (s Server) GetUser(ctx context.Context, request *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := s.Service.GetUser(ctx, request.GetId())
	if err != nil {
		return nil, rpc.ToStatus(err)
	}
	return &pb.GetUserResponse{Id: user.ID, Name: user.Name, Email: user.Email}, nil
}

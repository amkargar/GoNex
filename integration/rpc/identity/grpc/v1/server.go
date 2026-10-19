package identitygrpc

import (
	"context"
	pb "github.com/amkargar/gonex/contracts/gen/go/identity/v1"
	rpc "github.com/amkargar/gonex/grpc"
	"github.com/amkargar/gonex/integration/rpc/identity"
)

type Server struct {
	pb.UnimplementedIdentityServiceServer
	Service identity.Service
}

func (s Server) GetUser(ctx context.Context, in *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	u, err := s.Service.GetUser(ctx, in.GetId())
	if err != nil {
		return nil, rpc.ToStatus(err)
	}
	return response(u), nil
}

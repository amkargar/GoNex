package identitygrpc

import (
	pb "github.com/amkargar/gonex/contracts/gen/go/identity/v1"
	"github.com/amkargar/gonex/integration/rpc/identity"
)
func response(u identity.User)*pb.GetUserResponse{return &pb.GetUserResponse{Id:u.ID,Name:u.Name,Email:u.Email}}

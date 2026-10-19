package orders

import (
	"context"
	pb "github.com/amkargar/gonex/contracts/gen/go/identity/v1"
	rpc "github.com/amkargar/gonex/grpc"
)

type IdentityClient struct{ client pb.IdentityServiceClient }

func NewIdentityClient(pool *rpc.Clients, config *rpc.ClientConfig) (IdentityClient, error) {
	client, err := rpc.Client(pool, config, pb.NewIdentityServiceClient)
	return IdentityClient{client: client}, err
}
func (c IdentityClient) GetUser(ctx context.Context, id string) (User, error) {
	u, err := c.client.GetUser(ctx, &pb.GetUserRequest{Id: id})
	if err != nil {
		return User{}, rpc.FromStatus(err)
	}
	return User{ID: u.Id, Name: u.Name, Email: u.Email}, nil
}

package ordershttp

import (
	"example.com/gonex-production/modules/identity"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
)

func Register(r transport.EndpointRegistrar, actions Actions, policy identity.Policy) error {
	h := Handler{actions}
	read := []gin.HandlerFunc{policy.Authenticate()}
	write := []gin.HandlerFunc{policy.Authenticate(), policy.Write()}
	security := []map[string][]string{{"bearerAuth": {}}}
	schemes := openapi.Object{"bearerAuth": openapi.Object{"type": "http", "scheme": "bearer"}}
	if err := transport.Register(r, transport.Endpoint[CreateRequest, Response]{Method: "POST", Path: "/api/orders", OperationID: "createOrder", Summary: "Create order", Tags: []string{"orders"}, Status: 201, Errors: []int{401, 403, 409, 503}, Security: security, SecuritySchemes: schemes, Middleware: write, Handler: h.Create}); err != nil {
		return err
	}
	if err := transport.Register(r, transport.Endpoint[ListRequest, ListResponse]{Method: "GET", Path: "/api/orders", OperationID: "listOrders", Summary: "List and filter orders", Tags: []string{"orders"}, Status: 200, Errors: []int{401, 503}, Security: security, SecuritySchemes: schemes, Middleware: read, Handler: h.List}); err != nil {
		return err
	}
	if err := transport.Register(r, transport.Endpoint[GetRequest, Response]{Method: "GET", Path: "/api/orders/:id", OperationID: "getOrder", Summary: "Get order", Tags: []string{"orders"}, Status: 200, Errors: []int{401, 404, 503}, Security: security, SecuritySchemes: schemes, Middleware: read, Handler: h.Get}); err != nil {
		return err
	}
	if err := transport.Register(r, transport.Endpoint[UpdateRequest, Response]{Method: "PUT", Path: "/api/orders/:id", OperationID: "updateOrder", Summary: "Replace order values", Tags: []string{"orders"}, Status: 200, Errors: []int{401, 403, 404, 503}, Security: security, SecuritySchemes: schemes, Middleware: write, Handler: h.Update}); err != nil {
		return err
	}
	return transport.Register(r, transport.Endpoint[GetRequest, transport.NoBody]{Method: "DELETE", Path: "/api/orders/:id", OperationID: "deleteOrder", Summary: "Delete order", Tags: []string{"orders"}, Status: 204, Errors: []int{401, 403, 404, 503}, Security: security, SecuritySchemes: schemes, Middleware: write, Handler: h.Delete})
}

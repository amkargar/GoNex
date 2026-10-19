package http

import (
	"context"
	"fmt"
	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"reflect"
	"strings"
)

type NoBody struct{}
type Endpoint[I, O any] struct {
	Method, Path, OperationID, Summary, Description string
	Tags                                            []string
	Status                                          int
	Errors                                          []int
	MaxBodyBytes                                    int64
	RequestExample, ResponseExample                 any
	Security                                        []map[string][]string
	SecuritySchemes                                 openapi.Object
	Middleware                                      []gin.HandlerFunc
	Handler                                         func(context.Context, I) (O, error)
}
type EndpointRegistrar interface{ Endpoint(Route) error }

var validation = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		for _, tag := range []string{"json", "path", "query", "header"} {
			if s := strings.Split(f.Tag.Get(tag), ",")[0]; s != "" {
				return s
			}
		}
		return f.Name
	})
	return v
}()

func Register[I, O any](r EndpointRegistrar, e Endpoint[I, O]) (err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("invalid endpoint validation definition")
		}
	}()
	if e.Handler == nil {
		return fmt.Errorf("nil endpoint handler")
	}
	if e.MaxBodyBytes < 0 {
		return fmt.Errorf("negative request limit")
	}
	if e.MaxBodyBytes == 0 {
		e.MaxBodyBytes = 1 << 20
	}
	contract := openapi.Contract{Method: strings.ToUpper(e.Method), Path: e.Path, OperationID: e.OperationID, Summary: e.Summary, Description: e.Description, Tags: e.Tags, Status: e.Status, Request: reflect.TypeFor[I](), Response: reflect.TypeFor[O](), Errors: append([]int(nil), e.Errors...)}
	contract.RequestExample = e.RequestExample
	contract.ResponseExample = e.ResponseExample
	contract.Security = e.Security
	contract.SecuritySchemes = e.SecuritySchemes
	if _, err := openapi.Generate([]openapi.Contract{contract}); err != nil {
		return err
	}
	inputs, err := openapi.Inputs(contract.Request)
	if err != nil {
		return err
	}
	var probe I
	_ = validation.Struct(probe)
	handler := func(c *gin.Context) {
		var input I
		if !bind(c, reflect.ValueOf(&input).Elem(), inputs, e.MaxBodyBytes) {
			return
		}
		if err := validation.Struct(input); err != nil {
			fields := []FieldError{}
			if failures, ok := err.(validator.ValidationErrors); ok {
				for _, f := range failures {
					_, path, _ := strings.Cut(f.Namespace(), ".")
					fields = append(fields, FieldError{path, f.Tag()})
				}
			}
			c.JSON(422, ErrorResponse{Code: "validation", Message: "request validation failed", Fields: fields})
			return
		}
		output, err := e.Handler(c.Request.Context(), input)
		if err != nil {
			renderError(c, err, e.Errors)
			return
		}
		if e.Status == 204 {
			c.Status(204)
			return
		}
		c.JSON(e.Status, output)
	}
	return r.Endpoint(Route{Method: contract.Method, Path: e.Path, Handlers: append(append([]gin.HandlerFunc(nil), e.Middleware...), handler), Contract: &contract})
}

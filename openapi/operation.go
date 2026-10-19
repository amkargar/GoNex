package openapi

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

type Contract struct {
	Version                                         string
	Method, Path, OperationID, Summary, Description string
	Tags                                            []string
	Status                                          int
	Request, Response                               reflect.Type
	Errors                                          []int
	RequestExample, ResponseExample                 any
	Security                                        []map[string][]string
	SecuritySchemes                                 Object
}
type Input struct {
	Index        int
	Origin, Name string
	Type         reflect.Type
	Required     bool
}

// Inputs is shared by runtime binding and document generation.
func Inputs(t reflect.Type) ([]Input, error) {
	if t == nil || t.Kind() != reflect.Struct || t.Name() == "" {
		return nil, fmt.Errorf("request must be a named struct")
	}
	result := []Input{}
	seen := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			return nil, fmt.Errorf("request fields must be exported")
		}
		var in Input
		count := 0
		for _, origin := range []string{"path", "query", "header", "body"} {
			if name, ok := f.Tag.Lookup(origin); ok {
				count++
				in = Input{Index: i, Origin: origin, Name: name, Type: f.Type, Required: origin == "path" || hasTag(f.Tag.Get("validate"), "required")}
			}
		}
		if count != 1 || in.Name == "" {
			return nil, fmt.Errorf("field %s must declare exactly one named binding origin", f.Name)
		}
		if in.Origin == "body" {
			in.Name = "body"
			in.Required = true
		} else {
			kind := in.Type.Kind()
			if kind == reflect.Pointer {
				kind = in.Type.Elem().Kind()
			}
			if kind != reflect.String && kind != reflect.Bool && (kind < reflect.Int || kind > reflect.Float64) {
				return nil, fmt.Errorf("parameter %s must be scalar", in.Name)
			}
		}
		key := in.Origin + ":" + strings.ToLower(in.Name)
		if seen[key] {
			return nil, fmt.Errorf("duplicate binding %s", key)
		}
		seen[key] = true
		result = append(result, in)
	}
	return result, nil
}

var pathParam = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

func (c Contract) operation(s *schemas) (string, Object, error) {
	if c.OperationID == "" || c.Status < 200 || c.Status > 299 || c.Response == nil || c.Response.Name() == "" {
		return "", nil, fmt.Errorf("endpoint needs operation ID, success status and named response type")
	}
	if strings.Contains(c.Path, "*") {
		return "", nil, fmt.Errorf("typed endpoints do not support catch-all paths")
	}
	inputs, err := Inputs(c.Request)
	if err != nil {
		return "", nil, err
	}
	path := pathParam.ReplaceAllString(c.Path, "{$1}")
	op := Object{"operationId": c.OperationID, "responses": Object{}}
	if c.Security != nil {
		op["security"] = c.Security
	}
	if c.Summary != "" {
		op["summary"] = c.Summary
	}
	if c.Description != "" {
		op["description"] = c.Description
	}
	if len(c.Tags) > 0 {
		op["tags"] = c.Tags
	}
	parameters := []Object{}
	bound := map[string]bool{}
	for _, in := range inputs {
		schema, e := s.schema(in.Type, false)
		if e != nil {
			return "", nil, e
		}
		if in.Origin == "body" {
			media := Object{"schema": schema}
			if c.RequestExample != nil {
				media["example"] = c.RequestExample
			}
			op["requestBody"] = Object{"required": true, "content": Object{"application/json": media}}
			continue
		}
		if in.Origin == "path" {
			if !strings.Contains(path, "{"+in.Name+"}") {
				return "", nil, fmt.Errorf("path binding %s has no placeholder", in.Name)
			}
			bound[in.Name] = true
		}
		if e = constraints(schema, in.Type, c.Request.Field(in.Index).Tag.Get("validate")); e != nil {
			return "", nil, e
		}
		parameters = append(parameters, Object{"name": in.Name, "in": in.Origin, "required": in.Required, "schema": schema})
	}
	for _, match := range pathParam.FindAllStringSubmatch(c.Path, -1) {
		if !bound[match[1]] {
			return "", nil, fmt.Errorf("unbound path parameter %s", match[1])
		}
	}
	if len(parameters) > 0 {
		op["parameters"] = parameters
	}
	responses := op["responses"].(Object)
	response := Object{"description": "Success"}
	if c.Status != 204 {
		schema, e := s.schema(c.Response, true)
		if e != nil {
			return "", nil, e
		}
		media := Object{"schema": schema}
		if c.ResponseExample != nil {
			media["example"] = c.ResponseExample
		}
		response["content"] = Object{"application/json": media}
	}
	responses[fmt.Sprint(c.Status)] = response
	for _, status := range append([]int{400, 413, 422, 500}, c.Errors...) {
		if status < 400 || status > 599 {
			return "", nil, fmt.Errorf("invalid documented error status %d", status)
		}
		responses[fmt.Sprint(status)] = Object{"description": "Error", "content": Object{"application/json": Object{"schema": Object{"$ref": "#/components/schemas/ErrorResponse"}}}}
	}
	return path, op, nil
}

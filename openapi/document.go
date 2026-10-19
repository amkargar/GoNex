package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Generate produces a deterministic immutable document; call at initialization/export.
func Generate(contracts []Contract) ([]byte, error) {
	s := &schemas{components: Object{"ErrorResponse": Object{"type": "object", "required": []string{"code", "message"}, "properties": Object{"code": Object{"type": "string"}, "message": Object{"type": "string"}, "fields": Object{"type": "array", "items": Object{"type": "object", "required": []string{"field", "code"}, "properties": Object{"field": Object{"type": "string"}, "code": Object{"type": "string"}}}}}}}}
	paths := Object{}
	ids := map[string]bool{}
	securitySchemes := Object{}
	for _, c := range contracts {
		for name, definition := range c.SecuritySchemes {
			if old, ok := securitySchemes[name]; ok {
				a, _ := json.Marshal(old)
				b, _ := json.Marshal(definition)
				if string(a) != string(b) {
					return nil, fmt.Errorf("conflicting security scheme %s", name)
				}
			}
			securitySchemes[name] = definition
		}
		if ids[c.OperationID] {
			return nil, fmt.Errorf("duplicate operation ID %s", c.OperationID)
		}
		ids[c.OperationID] = true
		path, op, err := c.operation(s)
		if err != nil {
			return nil, err
		}
		if paths[path] == nil {
			paths[path] = Object{}
		}
		methods := paths[path].(Object)
		method := strings.ToLower(c.Method)
		if methods[method] != nil {
			return nil, fmt.Errorf("duplicate documented route %s %s", method, path)
		}
		methods[method] = op
	}
	doc := Object{"openapi": "3.1.0", "info": Object{"title": "Gonex API", "version": "1.0.0"}, "paths": paths, "components": Object{"schemas": s.components}}
	if len(securitySchemes) > 0 {
		doc["components"].(Object)["securitySchemes"] = securitySchemes
	}
	return json.MarshalIndent(doc, "", "  ")
}

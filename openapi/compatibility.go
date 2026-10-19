package openapi

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Compatible conservatively permits additive operations/components and rejects
// existing contract changes, including changes to security requirements.
func Compatible(previous, next []byte) error {
	if err := Validate(previous); err != nil {
		return err
	}
	if err := Validate(next); err != nil {
		return err
	}
	var old, current Object
	if err := json.Unmarshal(previous, &old); err != nil {
		return err
	}
	if err := json.Unmarshal(next, &current); err != nil {
		return err
	}
	if !reflect.DeepEqual(old["security"], current["security"]) {
		return fmt.Errorf("changed global API security")
	}
	if !reflect.DeepEqual(old["servers"], current["servers"]) {
		return fmt.Errorf("changed global API servers")
	}
	for path, raw := range object(old["paths"]) {
		methods, ok := object(current["paths"])[path]
		if !ok {
			return fmt.Errorf("removed API path %s", path)
		}
		for _, field := range []string{"parameters", "$ref", "servers"} {
			if !reflect.DeepEqual(object(raw)[field], object(methods)[field]) {
				return fmt.Errorf("changed path %s for %s", field, path)
			}
		}
		for method, operation := range object(raw) {
			if method == "parameters" || method == "$ref" {
				if !reflect.DeepEqual(operation, object(methods)[method]) {
					return fmt.Errorf("changed path contract %s", path)
				}
				continue
			}
			if method != "get" && method != "post" && method != "put" && method != "patch" && method != "delete" && method != "head" && method != "options" && method != "trace" {
				continue
			}
			newOperation, ok := object(methods)[method]
			if !ok {
				return fmt.Errorf("removed API operation %s %s", method, path)
			}
			for _, field := range []string{"operationId", "parameters", "requestBody", "responses", "security", "servers"} {
				if !reflect.DeepEqual(object(operation)[field], object(newOperation)[field]) {
					return fmt.Errorf("changed API %s for %s %s", field, method, path)
				}
			}
		}
	}
	for kind, components := range object(old["components"]) {
		for name, schema := range object(components) {
			if !reflect.DeepEqual(schema, object(object(current["components"])[kind])[name]) {
				return fmt.Errorf("changed API component %s/%s", kind, name)
			}
		}
	}
	return nil
}
func object(value any) Object { result, _ := value.(map[string]any); return result }

// Package openapi projects typed endpoint contracts into OpenAPI 3.1.
package openapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Object = map[string]any
type schemas struct{ components Object }

func (s *schemas) schema(t reflect.Type, output bool) (Object, error) {
	if t.Kind() == reflect.Pointer {
		inner, err := s.schema(t.Elem(), output)
		if err != nil {
			return nil, err
		}
		return Object{"anyOf": []any{inner, Object{"type": "null"}}}, nil
	}
	if t == reflect.TypeFor[time.Time]() {
		return Object{"type": "string", "format": "date-time"}, nil
	}
	if t.Implements(reflect.TypeFor[json.Marshaler]()) || reflect.PointerTo(t).Implements(reflect.TypeFor[json.Marshaler]()) || reflect.PointerTo(t).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		return nil, fmt.Errorf("custom JSON encoding on %s requires a plain transport DTO", t)
	}
	if t.Kind() == reflect.Struct && t.Name() != "" {
		mode := "Input"
		if output {
			mode = "Output"
		}
		sum := sha256.Sum256([]byte(t.PkgPath() + "." + t.Name()))
		name := fmt.Sprintf("%s_%x%s", t.Name(), sum[:4], mode)
		ref := Object{"$ref": "#/components/schemas/" + name}
		if _, ok := s.components[name]; ok {
			return ref, nil
		}
		s.components[name] = Object{}
		obj, err := s.structSchema(t, output)
		if err != nil {
			return nil, err
		}
		s.components[name] = obj
		return ref, nil
	}
	switch t.Kind() {
	case reflect.Struct:
		return s.structSchema(t, output)
	case reflect.String:
		return Object{"type": "string"}, nil
	case reflect.Bool:
		return Object{"type": "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Object{"type": "integer"}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Object{"type": "integer", "minimum": 0}, nil
	case reflect.Float32, reflect.Float64:
		return Object{"type": "number"}, nil
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return nil, fmt.Errorf("byte slices need an explicit transport DTO")
		}
		item, err := s.schema(t.Elem(), output)
		if err != nil {
			return nil, err
		}
		obj := Object{"type": "array", "items": item}
		if t.Kind() == reflect.Array {
			obj["minItems"] = t.Len()
			obj["maxItems"] = t.Len()
		}
		if t.Kind() == reflect.Slice {
			obj["type"] = []string{"array", "null"}
		}
		return obj, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("JSON map keys must be strings")
		}
		item, err := s.schema(t.Elem(), output)
		if err != nil {
			return nil, err
		}
		return Object{"type": []string{"object", "null"}, "additionalProperties": item}, nil
	default:
		return nil, fmt.Errorf("unsupported JSON schema type %s", t)
	}
}

func (s *schemas) structSchema(t reflect.Type, output bool) (Object, error) {
	properties := Object{}
	required := []string{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		parts := strings.Split(f.Tag.Get("json"), ",")
		name := parts[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if f.Anonymous || len(parts) > 2 || len(parts) > 1 && parts[1] != "omitempty" {
			return nil, fmt.Errorf("unsupported JSON field options on %s.%s", t, f.Name)
		}
		if !output && f.Type.Kind() == reflect.Struct && f.Type != reflect.TypeFor[time.Time]() {
			return nil, fmt.Errorf("nested request object %s.%s must use a pointer for explicit presence", t, f.Name)
		}
		if _, exists := properties[name]; exists {
			return nil, fmt.Errorf("duplicate JSON field %s", name)
		}
		fieldType := f.Type
		if !output && hasTag(f.Tag.Get("validate"), "required") && fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		p, err := s.schema(fieldType, output)
		if err != nil {
			return nil, err
		}
		// Constraints belong beside refs, never mutate a shared component.
		copy := Object{}
		for k, v := range p {
			copy[k] = v
		}
		p = copy
		if err = constraints(p, f.Type, f.Tag.Get("validate")); err != nil {
			return nil, err
		}
		if !output && hasTag(f.Tag.Get("validate"), "required") && (f.Type.Kind() == reflect.Slice || f.Type.Kind() == reflect.Map) {
			if f.Type.Kind() == reflect.Slice {
				p["type"] = "array"
			} else {
				p["type"] = "object"
			}
		}
		properties[name] = p
		alwaysPresent := !strings.Contains(f.Tag.Get("json"), ",omitempty") || f.Type.Kind() == reflect.Struct || f.Type.Kind() == reflect.Array && f.Type.Len() > 0
		if output && alwaysPresent || !output && hasTag(f.Tag.Get("validate"), "required") {
			required = append(required, name)
		}
	}
	obj := Object{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		obj["required"] = required
	}
	return obj, nil
}

func hasTag(tags, tag string) bool {
	for _, p := range strings.Split(tags, ",") {
		if p == tag {
			return true
		}
	}
	return false
}
func constraints(obj Object, t reflect.Type, tags string) error {
	parts := strings.Split(tags, ",")
	for index, tag := range parts {
		if tag == "dive" {
			base := t
			for base.Kind() == reflect.Pointer {
				base = base.Elem()
			}
			if base.Kind() != reflect.Slice && base.Kind() != reflect.Array && base.Kind() != reflect.Map {
				return fmt.Errorf("dive requires a collection")
			}
			if err := constraints(obj, t, strings.Join(parts[:index], ",")); err != nil {
				return err
			}
			node := obj
			if alternatives, ok := obj["anyOf"].([]any); ok && len(alternatives) > 0 {
				node, _ = alternatives[0].(map[string]any)
			}
			key := "items"
			if base.Kind() == reflect.Map {
				key = "additionalProperties"
			}
			item, ok := node[key].(map[string]any)
			if !ok {
				return fmt.Errorf("collection schema is missing items")
			}
			return constraints(item, base.Elem(), strings.Join(parts[index+1:], ","))
		}
	}
	if hasTag(tags, "required") && hasTag(tags, "omitempty") {
		return fmt.Errorf("required and omitempty validation must not be combined")
	}
	wasPointer := t.Kind() == reflect.Pointer
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for _, tag := range strings.Split(tags, ",") {
		tagName, _, _ := strings.Cut(tag, "=")
		switch tagName {
		case "", "-", "required", "omitempty", "email", "min", "max":
		default:
			return fmt.Errorf("unsupported validation tag %s; use a documented contract constraint", tagName)
		}
		if tag == "required" && !wasPointer {
			switch t.Kind() {
			case reflect.Struct:
				return fmt.Errorf("required object validation must use a pointer for explicit presence")
			case reflect.String:
				obj["minLength"] = 1
			case reflect.Bool:
				obj["const"] = true
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
				obj["not"] = Object{"const": 0}
			}
		}
		k, v, ok := strings.Cut(tag, "=")
		if tag == "email" {
			if t.Kind() != reflect.String {
				return fmt.Errorf("email requires a string")
			}
			obj["format"] = "email"
		}
		if !ok || k != "min" && k != "max" {
			continue
		}
		n, err := strconv.ParseFloat(v, 64)
		if t.Kind() == reflect.Bool || t.Kind() == reflect.Struct {
			return fmt.Errorf("min/max require a numeric, string or collection field")
		}
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("invalid validation constraint %q", tag)
		}
		key := "minimum"
		if k == "max" {
			key = "maximum"
		}
		if t.Kind() == reflect.String {
			if n < 0 || math.Trunc(n) != n {
				return fmt.Errorf("string limits must be nonnegative integers")
			}
			if k == "min" && hasTag(tags, "required") && !wasPointer && n < 1 {
				n = 1
			}
			key = "minLength"
			if k == "max" {
				key = "maxLength"
			}
		}
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			if n < 0 || math.Trunc(n) != n {
				return fmt.Errorf("array limits must be nonnegative integers")
			}
			key = "minItems"
			if k == "max" {
				key = "maxItems"
			}
		}
		if t.Kind() == reflect.Map {
			if n < 0 || math.Trunc(n) != n {
				return fmt.Errorf("map limits must be nonnegative integers")
			}
			key = "minProperties"
			if k == "max" {
				key = "maxProperties"
			}
		}
		obj[key] = n
	}
	if hasTag(tags, "omitempty") && !wasPointer {
		var zero any
		valid := true
		switch t.Kind() {
		case reflect.String:
			zero = ""
		case reflect.Bool:
			zero = false
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
			zero = 0
		case reflect.Struct:
			return fmt.Errorf("optional nested structs must use pointers")
		default:
			valid = false
		}
		if valid {
			constrained := Object{}
			for key, value := range obj {
				constrained[key] = value
				delete(obj, key)
			}
			obj["anyOf"] = []any{constrained, Object{"const": zero}}
		}
	}
	return nil
}

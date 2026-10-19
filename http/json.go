package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func uniqueJSON(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON nesting limit exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); ok {
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate JSON key")
				}
				seen[name] = true
				if err = uniqueJSON(decoder, depth+1); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err = uniqueJSON(decoder, depth+1); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		}
	}
	return nil
}

// jsonShape rejects JSON null for nonnullable Go fields and bounds recursion;
// encoding/json remains responsible for scalar parsing and unknown fields.
func jsonShape(raw []byte, t reflect.Type, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON nesting limit exceeded")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		switch t.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice:
			return nil
		}
		return fmt.Errorf("null for nonnullable field")
	}
	if t.Kind() == reflect.Pointer {
		return jsonShape(raw, t.Elem(), depth+1)
	}
	if t == reflect.TypeFor[time.Time]() {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return err
		}
		allowed := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			allowed[name] = true
		}
		for name := range object {
			if !allowed[name] {
				return fmt.Errorf("unknown JSON field")
			}
		}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			if value, ok := object[name]; ok {
				if err := jsonShape(value, field.Type, depth+1); err != nil {
					return err
				}
			}
		}
	case reflect.Slice, reflect.Array:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		if t.Kind() == reflect.Array && len(items) != t.Len() {
			return fmt.Errorf("array length mismatch")
		}
		for _, item := range items {
			if err := jsonShape(item, t.Elem(), depth+1); err != nil {
				return err
			}
		}
	case reflect.Map:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return err
		}
		for _, item := range object {
			if err := jsonShape(item, t.Elem(), depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

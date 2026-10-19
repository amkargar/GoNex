package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	nethttp "net/http"
	"reflect"
	"strconv"
)

func bind(c *gin.Context, value reflect.Value, inputs []openapi.Input, limit int64) bool {
	hasBody := false
	bad := func(code int, message string) bool {
		c.JSON(code, ErrorResponse{Code: "malformed_request", Message: message})
		return false
	}
	for _, in := range inputs {
		field := value.Field(in.Index)
		if in.Origin == "body" {
			hasBody = true
			media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if err != nil || media != "application/json" {
				return bad(400, "expected application/json")
			}
			c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, limit)
			decoder := json.NewDecoder(c.Request.Body)
			decoder.DisallowUnknownFields()
			var raw json.RawMessage
			err = decoder.Decode(&raw)
			if err == nil {
				var extra any
				if e := decoder.Decode(&extra); e != io.EOF {
					if e == nil {
						err = errors.New("multiple JSON values")
					} else {
						err = e
					}
				}
			}
			if err != nil {
				var tooLarge *nethttp.MaxBytesError
				if errors.As(err, &tooLarge) {
					return bad(413, "request body too large")
				}
				return bad(400, "invalid JSON body")
			}
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && field.Kind() != reflect.Pointer && field.Kind() != reflect.Slice && field.Kind() != reflect.Map {
				return bad(400, "body must not be null")
			}
			strict := json.NewDecoder(bytes.NewReader(raw))
			strict.DisallowUnknownFields()
			if err = strict.Decode(field.Addr().Interface()); err != nil {
				return bad(400, "invalid JSON body")
			}
			continue
		}
		var raw string
		present := false
		switch in.Origin {
		case "path":
			raw = c.Param(in.Name)
			present = raw != ""
		case "query":
			values, ok := c.Request.URL.Query()[in.Name]
			present = ok
			if len(values) > 1 {
				return bad(400, "duplicate query parameter")
			}
			if ok && len(values) > 0 {
				raw = values[0]
			}
		case "header":
			values := c.Request.Header.Values(in.Name)
			present = len(values) > 0
			if len(values) > 1 {
				return bad(400, "duplicate header parameter")
			}
			if present {
				raw = values[0]
			}
		}
		if !present {
			if in.Required {
				return bad(400, "missing "+in.Origin+" parameter "+in.Name)
			}
			continue
		}
		if field.Kind() == reflect.Pointer {
			field.Set(reflect.New(field.Type().Elem()))
			field = field.Elem()
		}
		var err error
		switch field.Kind() {
		case reflect.String:
			field.SetString(raw)
		case reflect.Bool:
			var v bool
			v, err = strconv.ParseBool(raw)
			field.SetBool(v)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var v int64
			v, err = strconv.ParseInt(raw, 10, field.Type().Bits())
			field.SetInt(v)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			var v uint64
			v, err = strconv.ParseUint(raw, 10, field.Type().Bits())
			field.SetUint(v)
		case reflect.Float32, reflect.Float64:
			var v float64
			v, err = strconv.ParseFloat(raw, field.Type().Bits())
			field.SetFloat(v)
		}
		if err != nil {
			return bad(400, "invalid "+in.Origin+" parameter "+in.Name)
		}
	}
	if !hasBody && c.Request.ContentLength != 0 {
		return bad(400, "endpoint does not accept a body")
	}
	return true
}

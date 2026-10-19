package openapi

import (
	"bytes"
	"encoding/json"
)

type contractMetadata struct {
	RequestExample, ResponseExample any
	Security                        []map[string][]string
	SecuritySchemes                 Object
}

// Freeze detaches mutable metadata from application-owned slices and maps.
func (c Contract) Freeze() (Contract, error) {
	c.Tags = append([]string(nil), c.Tags...)
	c.Errors = append([]int(nil), c.Errors...)
	metadata := contractMetadata{c.RequestExample, c.ResponseExample, c.Security, c.SecuritySchemes}
	data, err := json.Marshal(metadata)
	if err != nil {
		return c, err
	}
	var frozen contractMetadata
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&frozen); err != nil {
		return c, err
	}
	c.RequestExample = frozen.RequestExample
	c.ResponseExample = frozen.ResponseExample
	c.Security = frozen.Security
	c.SecuritySchemes = frozen.SecuritySchemes
	return c, nil
}

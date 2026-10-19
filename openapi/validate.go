package openapi

import (
	_ "embed"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"sync"
)

// Official OpenAPI Initiative schema, https://spec.openapis.org/oas/3.1/schema/2022-10-07.
//
//go:embed oas31.schema.json
var specification []byte
var validatorOnce sync.Once
var specificationValidator *jsonschema.Schema
var specificationError error

func Validate(document []byte) error {
	validatorOnce.Do(func() {
		var raw any
		if specificationError = json.Unmarshal(specification, &raw); specificationError != nil {
			return
		}
		compiler := jsonschema.NewCompiler()
		if specificationError = compiler.AddResource("https://spec.openapis.org/oas/3.1/schema/2022-10-07", raw); specificationError != nil {
			return
		}
		specificationValidator, specificationError = compiler.Compile("https://spec.openapis.org/oas/3.1/schema/2022-10-07")
	})
	if specificationError != nil {
		return specificationError
	}
	var raw any
	if err := json.Unmarshal(document, &raw); err != nil {
		return err
	}
	return specificationValidator.Validate(raw)
}

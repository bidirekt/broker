package rest

import (
	"fmt"
	"strconv"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
)

const (
	minStatusCode = 100
	maxStatusCode = 599
)

type method struct {
	name         string
	takesRequest bool
}

var methods = []method{
	{name: "get", takesRequest: false},
	{name: "post", takesRequest: true},
	{name: "put", takesRequest: true},
	{name: "patch", takesRequest: true},
	{name: "delete", takesRequest: false},
}

var (
	Endpoint = descriptor.Key{
		ErrorCode: "endpoint.syntax",
		Check: func(key string) error {
			return CheckEndpoint(NormalizeEndpoint(key))
		},
	}
	StatusCode = descriptor.Key{ErrorCode: "status.out_of_range", Check: statusCodeInRange}
)

func (this Plugin) Grammar() descriptor.Node {
	responses := descriptor.Map(StatusCode, descriptor.SchemaRef)

	operations := make(descriptor.Fields, len(methods))
	for _, method := range methods {
		operation := descriptor.Fields{"responses": responses}
		if method.takesRequest {
			operation["request"] = descriptor.SchemaRef
		}

		operations[method.name] = descriptor.Object(operation)
	}

	return descriptor.Map(Endpoint, descriptor.Object(operations))
}

func statusCodeInRange(key string) error {
	status, err := strconv.ParseUint(key, 10, 0)
	if err != nil || status < minStatusCode || status > maxStatusCode {
		return fmt.Errorf("must be between %d and %d", minStatusCode, maxStatusCode)
	}

	return nil
}

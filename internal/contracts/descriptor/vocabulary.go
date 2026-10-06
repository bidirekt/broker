package descriptor

import (
	"fmt"
	"strconv"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/validations"
)

var (
	Text      = String()
	Flag      = Bool()
	SchemaRef = String()

	SchemaType = Enum{
		ErrorCode: "schema.invalid_type",
		Allowed:   []string{"object", "array", "string", "integer", "float", "boolean"},
	}

	Endpoint     = Key{ErrorCode: "endpoint.syntax", Check: endpointSyntax}
	ServiceName  = Key{ErrorCode: "service.name_syntax", Check: validations.ParticipantName}
	StatusCode   = Key{ErrorCode: "status.out_of_range", Check: statusCodeInRange}
	SchemaName   = Key{}
	PropertyName = Key{}
)

func endpointSyntax(key string) error {
	return validations.Endpoint(contract.NormalizeEndpoint(key))
}

const (
	minStatusCode = 100
	maxStatusCode = 599
)

func statusCodeInRange(key string) error {
	status, err := strconv.ParseUint(key, 10, 0)
	if err != nil || status < minStatusCode || status > maxStatusCode {
		return fmt.Errorf("must be between %d and %d", minStatusCode, maxStatusCode)
	}

	return nil
}

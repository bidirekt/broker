package descriptor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/validations"
)

var (
	Text      = String()
	Flag      = Bool()
	SchemaRef = String()

	SchemaType = Enum{
		ErrorCode: "schema.invalid_type",
		Allowed:   []string{"object", "array", "map", "string", "integer", "float", "number", "boolean"},
	}

	Endpoint     = Key{ErrorCode: "endpoint.syntax", Check: endpointSyntax}
	ServiceName  = Key{ErrorCode: "service.name_syntax", Check: validations.ParticipantName}
	StatusCode   = Key{ErrorCode: "status.out_of_range", Check: statusCodeInRange}
	SchemaName   = Key{ErrorCode: "schema.name_syntax", Check: pathSafeName}
	PropertyName = Key{ErrorCode: "property.name_syntax", Check: pathSafeName}
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

func pathSafeName(key string) error {
	switch {
	case key == "":
		return errors.New("must not be empty")
	case strings.ContainsAny(key, ".[]{};"):
		return errors.New(`must not contain ".", "[", "]", "{", "}" or ";"`)
	case strings.ContainsFunc(key, unicode.IsControl):
		return errors.New("must not contain control characters")
	default:
		return nil
	}
}

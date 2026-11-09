package descriptor

import (
	"errors"
	"strings"
	"unicode"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
)

var (
	Text      = String()
	Flag      = Bool()
	SchemaRef = String()

	SchemaType = Enum{
		ErrorCode: "schema.invalid_type",
		Allowed: []string{
			model.ObjectType,
			model.ArrayType,
			model.MapType,
			model.StringType,
			model.IntegerType,
			model.FloatType,
			model.NumberType,
			model.BooleanType,
		},
	}

	ServiceName  = Key{ErrorCode: "service.name_syntax", Check: validations.ParticipantName}
	SchemaName   = Key{ErrorCode: "schema.name_syntax", Check: pathSafeName}
	PropertyName = Key{ErrorCode: "property.name_syntax", Check: pathSafeName}
)

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

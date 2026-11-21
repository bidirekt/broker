package descriptor

import (
	"errors"
	"strings"
	"unicode"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
)

var (
	Description = Text{ErrorCode: "schema.description_syntax", Check: alphanumericWords}
	Flag        = Bool()
	SchemaRef   = String()

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
			model.EnumType,
			model.UnionType,
		},
	}

	ServiceName  = Key{ErrorCode: "service.name_syntax", Check: validations.ParticipantName}
	SchemaName   = Key{ErrorCode: "schema.name_syntax", Check: pathSafeName}
	PropertyName = Key{ErrorCode: "property.name_syntax", Check: pathSafeName}
	MemberKey    = Key{ErrorCode: "member.name_syntax", Check: pathSafeName}
)

func pathSafeName(key string) error {
	switch {
	case key == "":
		return errors.New("must not be empty")
	case strings.ContainsAny(key, ".[]{};()"):
		return errors.New(`must not contain ".", "[", "]", "{", "}", ";", "(" or ")"`)
	case strings.ContainsFunc(key, unicode.IsControl):
		return errors.New("must not contain control characters")
	default:
		return nil
	}
}

func alphanumericWords(value string) error {
	if value == "" {
		return errors.New("must not be empty")
	}

	for _, word := range strings.Split(value, " ") {
		if word == "" || !isAlphanumeric(word) {
			return errors.New("must be ASCII letters and digits, with single spaces between words")
		}
	}

	return nil
}

func isAlphanumeric(word string) bool {
	for _, character := range word {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}

	return true
}

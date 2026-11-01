package components

import (
	"bytes"
	"iter"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
)

const invalidInputMessage = "invalid input"

var jsonNULEscape = []byte(`u0000`)

func inputDoor() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if !argsAreText(ctx.Request().URI().QueryArgs().All()) {
			return fiber.NewError(fiber.StatusBadRequest, invalidInputMessage)
		}

		if bytes.HasPrefix(ctx.Request().Header.ContentType(), []byte(fiber.MIMEApplicationForm)) {
			if !argsAreText(ctx.Request().PostArgs().All()) {
				return fiber.NewError(fiber.StatusBadRequest, invalidInputMessage)
			}
			return ctx.Next()
		}

		if body := ctx.Body(); !utf8.Valid(body) || holdsJSONNULEscape(body) {
			return fiber.NewError(fiber.StatusBadRequest, invalidInputMessage)
		}

		return ctx.Next()
	}
}

func argsAreText(args iter.Seq2[[]byte, []byte]) bool {
	for key, value := range args {
		if !isText(key) || !isText(value) {
			return false
		}
	}
	return true
}

func isText(value []byte) bool {
	return utf8.Valid(value) && bytes.IndexByte(value, 0) < 0
}

// A backslash appears in valid JSON only inside a string, so an odd run of
// them before u0000 is always the escape and never an escaped backslash.
func holdsJSONNULEscape(body []byte) bool {
	for offset := 0; ; {
		found := bytes.Index(body[offset:], jsonNULEscape)
		if found < 0 {
			return false
		}
		at := offset + found

		backslashes := 0
		for at-backslashes-1 >= 0 && body[at-backslashes-1] == '\\' {
			backslashes++
		}
		if backslashes%2 == 1 {
			return true
		}

		offset = at + len(jsonNULEscape)
	}
}

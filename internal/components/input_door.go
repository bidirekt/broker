package components

import (
	"bytes"
	"iter"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

const invalidInputMessage = "invalid input"

var jsonNULEscape = []byte(`u0000`)

func inputDoor() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if !argsAreText(ctx.Request().URI().QueryArgs().All()) || !bodyIsText(ctx) {
			return fiber.NewError(fiber.StatusBadRequest, invalidInputMessage)
		}

		return ctx.Next()
	}
}

func bodyIsText(ctx fiber.Ctx) bool {
	contentType := bytes.ToLower(ctx.Request().Header.ContentType())
	if bytes.HasPrefix(contentType, []byte(fiber.MIMEMultipartForm)) {
		return false
	}

	body := ctx.Body()
	if !isText(body) || holdsJSONNULEscape(body) {
		return false
	}
	if !bytes.HasPrefix(contentType, []byte(fiber.MIMEApplicationForm)) {
		return true
	}

	// Parsed apart from the request, since fasthttp skips a form content type
	// of mixed case and would cache the empty result for the handler.
	var form fasthttp.Args
	form.ParseBytes(body)
	return argsAreText(form.All())
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

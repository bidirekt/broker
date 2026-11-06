package pages

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"strings"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/gofiber/fiber/v3"
)

const (
	signedInLayout  = "layout.html"
	signedOutLayout = "layout_signed_out.html"
)

//go:embed templates
var templateFiles embed.FS

var layouts = template.Must(template.New("").Funcs(timeFuncs(time.Now)).ParseFS(templateFiles, "templates/layout*.html", "templates/partial_*.html"))

type layoutData struct {
	CSRFToken    string
	Email        string
	ManagesUsers bool
	CanWrite     bool
	Section      string
	Notice       string
	Page         any
}

func Render(ctx fiber.Ctx, status int, files fs.FS, page string, data any) error {
	set, err := layouts.Clone()
	if err != nil {
		return err
	}
	if _, err := set.ParseFS(files, page); err != nil {
		return err
	}

	layout, values := layoutFor(ctx, data)

	var html bytes.Buffer
	if err := set.ExecuteTemplate(&html, layout, values); err != nil {
		return err
	}

	ctx.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	ctx.Set(fiber.HeaderCacheControl, "no-store")
	// htmx would push the form's action URL into history, and a reload GETs it
	if ctx.Method() != fiber.MethodGet && isHtmxRequest(ctx) {
		ctx.Set("HX-Push-Url", "false")
	}

	return ctx.Status(status).Send(html.Bytes())
}

func isHtmxRequest(ctx fiber.Ctx) bool {
	return ctx.Get("HX-Request") == "true"
}

func layoutFor(ctx fiber.Ctx, page any) (layout string, data layoutData) {
	session, ok := access.SessionFrom(ctx)
	if !ok {
		return signedOutLayout, layoutData{Page: page}
	}

	principal, _ := access.PrincipalFrom(ctx)

	return signedInLayout, layoutData{
		CSRFToken:    session.CSRFToken,
		Email:        principal.Email,
		ManagesUsers: principal.ManagesUsers(),
		CanWrite:     principal.CanWrite(),
		Section:      sectionOf(ctx.Path()),
		Notice:       session.Message,
		Page:         page,
	}
}

func sectionOf(path string) string {
	section, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if section == "checks" {
		return "participants"
	}
	return section
}

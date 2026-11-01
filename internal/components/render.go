package components

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"strings"

	"github.com/gofiber/fiber/v3"
)

const (
	signedInLayout  = "layout.html"
	signedOutLayout = "layout_signed_out.html"
)

//go:embed templates
var templateFiles embed.FS

var layouts = template.Must(template.ParseFS(templateFiles, "templates/layout*.html"))

type layoutData struct {
	CSRFToken string
	IsAdmin   bool
	Section   string
	Page      any
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
	session, ok := SessionFrom(ctx)
	if !ok {
		return signedOutLayout, layoutData{Page: page}
	}

	principal, _ := PrincipalFrom(ctx)

	return signedInLayout, layoutData{
		CSRFToken: session.CSRFToken,
		IsAdmin:   principal.IsAdmin,
		Section:   sectionOf(ctx.Path()),
		Page:      page,
	}
}

func sectionOf(path string) string {
	section, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	return section
}

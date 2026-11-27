package pages_test

import (
	"io"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	writeProbeTemplate = "write_probe.html"
	writeProbeMarker   = "write actions offered"
)

var writeProbeFiles = fstest.MapFS{
	writeProbeTemplate: {Data: []byte(`{{define "title"}}Probe{{end}}{{define "content"}}{{if .CanWrite}}` + writeProbeMarker + `{{end}}{{end}}`)},
}

func renderSignedInAs(t *testing.T, permission model.Permission) string {
	t.Helper()

	app := fiber.New()
	app.Get("/probe", func(ctx fiber.Ctx) error {
		access.GrantSession(ctx, access.Session{CSRFToken: "token"})
		access.GrantPrincipal(ctx, access.Principal{UserID: 1, Email: "someone@acme.com", Permission: permission})

		return pages.Render(ctx, fiber.StatusOK, writeProbeFiles, writeProbeTemplate, nil)
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/probe", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return string(body)
}

func TestLayoutOffersWriteActionsOnlyWithWrite(t *testing.T) {
	assert.NotContains(t, renderSignedInAs(t, model.PermissionRead), writeProbeMarker)
	assert.Contains(t, renderSignedInAs(t, model.PermissionWrite), writeProbeMarker)
	assert.Contains(t, renderSignedInAs(t, model.PermissionAdmin), writeProbeMarker)
}

const sectionProbeTemplate = "section_probe.html"

var sectionProbeFiles = fstest.MapFS{
	sectionProbeTemplate: {Data: []byte(`{{define "title"}}Probe{{end}}{{define "content"}}<div class="sections">{{range .Page}}{{section "probe_breaks" .}}{{end}}</div>{{end}}`)},
}

var probeSectionFiles = fstest.MapFS{
	"probe_breaks.html": {Data: []byte(`{{define "probe_breaks"}}<p class="probe">{{template "icon_x" 14}}<span>{{.}}</span></p>{{end}}`)},
	"probes.css":        {Data: []byte(`.probe{}`)},
}

func renderWithSection(t *testing.T, breaks []string) string {
	t.Helper()

	app := fiber.New()
	app.Get("/probe", func(ctx fiber.Ctx) error {
		access.GrantSession(ctx, access.Session{CSRFToken: "token"})
		access.GrantPrincipal(ctx, access.Principal{UserID: 1, Email: "someone@acme.com", Permission: model.PermissionRead})

		return pages.Render(ctx, fiber.StatusOK, sectionProbeFiles, sectionProbeTemplate, breaks, probeSectionFiles)
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/probe", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return string(body)
}

func TestSectionDrawsItsTemplateWithThePagePartialsAndItsOwnData(t *testing.T) {
	body := renderWithSection(t, []string{"petstore_web pings <petstore_api>", "petstore_web pings twice"})

	assert.Contains(t, body, `<div class="sections"><p class="probe"><svg viewBox="0 0 16 16" width="14" height="14"`)
	assert.Contains(t, body, `<span>petstore_web pings &lt;petstore_api&gt;</span></p><p class="probe">`)
	assert.Contains(t, body, `<span>petstore_web pings twice</span></p></div>`)
}

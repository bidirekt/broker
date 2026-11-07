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

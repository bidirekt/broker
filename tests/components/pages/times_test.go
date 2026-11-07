package pages_test

import (
	"io"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var referenceNow = time.Date(2026, 11, 4, 15, 30, 0, 0, time.UTC)

func TestRelativeTimeFollowsTheAgeLadder(t *testing.T) {
	cases := map[time.Duration]string{
		-2 * time.Hour:                  "just now",
		0:                               "just now",
		59*time.Minute + 59*time.Second: "just now",
		time.Hour:                       "1 hour ago",
		2*time.Hour - time.Second:       "1 hour ago",
		2 * time.Hour:                   "2 hours ago",
		24*time.Hour - time.Second:      "23 hours ago",
		24 * time.Hour:                  "yesterday",
		48*time.Hour - time.Second:      "yesterday",
		48 * time.Hour:                  "2 days ago",
		10 * 24 * time.Hour:             "10 days ago",
	}

	for age, expected := range cases {
		assert.Equal(t, expected, pages.RelativeTime(referenceNow.Add(-age), referenceNow), "age %s", age)
	}
}

func TestListTimeIsRelativeUpToThreeDaysThenTheDate(t *testing.T) {
	assert.Equal(t, "just now", pages.ListTime(referenceNow.Add(time.Hour), referenceNow))
	assert.Equal(t, "3 days ago", pages.ListTime(referenceNow.Add(-4*24*time.Hour+time.Second), referenceNow))
	assert.Equal(t, "2026-10-31", pages.ListTime(referenceNow.Add(-4*24*time.Hour), referenceNow))
}

func TestDetailTimeShowsTheMinuteInUTCAndTheRelativeAtAnyAge(t *testing.T) {
	saoPaulo := time.FixedZone("BRT", -3*60*60)

	assert.Equal(t, "2026-11-04 15:00 UTC · just now", pages.DetailTime(referenceNow.Add(-30*time.Minute).In(saoPaulo), referenceNow))
	assert.Equal(t, "2025-11-04 15:30 UTC · 365 days ago", pages.DetailTime(referenceNow.AddDate(-1, 0, 0), referenceNow))
}

func TestExpiryTimeShowsTheMinuteInUTCWithoutTheRelative(t *testing.T) {
	saoPaulo := time.FixedZone("BRT", -3*60*60)

	assert.Equal(t, "2026-11-05 15:30 UTC", pages.ExpiryTime(referenceNow.Add(24*time.Hour).In(saoPaulo)))
}

func TestMachineTimeIsTheExactInstantInUTCWithZ(t *testing.T) {
	saoPaulo := time.FixedZone("BRT", -3*60*60)
	at := time.Date(2026, 11, 4, 12, 30, 0, 123456000, saoPaulo)

	assert.Equal(t, "2026-11-04T15:30:00.123456Z", pages.MachineTime(at))
}

func TestRenderedPagesFormatTimesAsTimeElements(t *testing.T) {
	at := time.Now().Add(-2*time.Hour - 30*time.Minute)
	expiry := time.Date(2026, 11, 5, 15, 30, 0, 0, time.UTC)
	page := fstest.MapFS{"page.html": {Data: []byte(
		`{{define "title"}}Times{{end}}{{define "content"}}{{listTime .Page.At}}|{{detailTime .Page.At}}|{{expiryTime .Page.Expiry}}{{end}}`,
	)}}

	app := fiber.New()
	app.Get("/", func(ctx fiber.Ctx) error {
		return pages.Render(ctx, fiber.StatusOK, page, "page.html", struct{ At, Expiry time.Time }{at, expiry})
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	open := `<time class="time" datetime="` + pages.MachineTime(at) + `" title="` + pages.ExpiryTime(at) + `">`
	assert.Contains(t, string(body), open+`2 hours ago</time>|`+open+pages.DetailTime(at, time.Now())+`</time>|`)
	assert.Contains(t, string(body), `<time class="time" datetime="2026-11-05T15:30:00Z" title="2026-11-05 15:30 UTC">2026-11-05 15:30 UTC</time>`)
}

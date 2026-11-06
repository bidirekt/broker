package pages_test

import (
	"io"
	"math"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pagerParam = "versions_page"

type pagerOutcome struct {
	page  int64
	ok    bool
	pager pages.Pager
}

func pagerFor(t *testing.T, target string, total int) pagerOutcome {
	t.Helper()

	var outcome pagerOutcome
	app := fiber.New()
	app.Get("/*", func(ctx fiber.Ctx) error {
		outcome.page, outcome.ok = pages.RequestedPage(ctx, pagerParam)
		if outcome.ok {
			outcome.pager, outcome.ok = pages.NewPager(ctx, pagerParam, outcome.page, total)
		}

		return ctx.SendStatus(fiber.StatusNoContent)
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, nil))
	require.NoError(t, err)

	return outcome
}

func TestRequestedPageIsTheFirstWhenTheParameterIsAbsent(t *testing.T) {
	outcome := pagerFor(t, "/things?page=7", 100)

	require.True(t, outcome.ok)
	assert.Equal(t, int64(1), outcome.page)
}

func TestRequestedPageRejectsANonCanonicalValue(t *testing.T) {
	for _, raw := range []string{"", "0", "01", "%2B1", "-1", "abc", "1.0"} {
		outcome := pagerFor(t, "/things?"+pagerParam+"="+raw, 100)

		assert.False(t, outcome.ok, "raw %q", raw)
	}
}

func requestedPageFor(t *testing.T, target string) (page int64, ok bool) {
	t.Helper()

	app := fiber.New()
	app.Get("/things", func(ctx fiber.Ctx) error {
		page, ok = pages.RequestedPage(ctx, pagerParam)
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, nil))
	require.NoError(t, err)

	return page, ok
}

func TestRequestedPageRefusesAPageWhoseOffsetWouldOverflow(t *testing.T) {
	largest := int64(math.MaxInt64 / pages.PageSize)

	page, ok := requestedPageFor(t, "/things?"+pagerParam+"="+strconv.FormatInt(largest, 10))
	require.True(t, ok)
	assert.Equal(t, largest, page)
	assert.Positive(t, pages.Offset(page))

	_, ok = requestedPageFor(t, "/things?"+pagerParam+"="+strconv.FormatInt(largest+1, 10))
	assert.False(t, ok)
}

func TestPagerShowsTwentyFiveItemsAPage(t *testing.T) {
	first := pagerFor(t, "/things", 60)
	require.True(t, first.ok)
	assert.Equal(t, pages.Pager{Page: 1, First: 1, Last: 25, Total: 60, NextURL: "/things?versions_page=2"}, first.pager)

	last := pagerFor(t, "/things?"+pagerParam+"=3", 60)
	require.True(t, last.ok)
	assert.Equal(t, pages.Pager{Page: 3, First: 51, Last: 60, Total: 60, PreviousURL: "/things?versions_page=2"}, last.pager)

	assert.Equal(t, 50, pages.Offset(3))
}

func TestPagerRejectsAPagePastTheLast(t *testing.T) {
	assert.True(t, pagerFor(t, "/things?"+pagerParam+"=2", 26).ok)
	assert.False(t, pagerFor(t, "/things?"+pagerParam+"=2", 25).ok)
}

func TestPagerOnAnEmptyListHasOnlyTheFirstPage(t *testing.T) {
	first := pagerFor(t, "/things", 0)
	require.True(t, first.ok)
	assert.Equal(t, pages.Pager{Page: 1, Total: 0}, first.pager)

	assert.False(t, pagerFor(t, "/things?"+pagerParam+"=2", 0).ok)
}

func TestPagerKeepsTheOtherQueryParametersAndTheEscapedPath(t *testing.T) {
	outcome := pagerFor(t, "/participants/petstore%20web?environment=7&"+pagerParam+"=2&checks_page=4", 80)

	require.True(t, outcome.ok)
	assert.Equal(t, "/participants/petstore%20web?checks_page=4&environment=7&versions_page=1", outcome.pager.PreviousURL)
	assert.Equal(t, "/participants/petstore%20web?checks_page=4&environment=7&versions_page=3", outcome.pager.NextURL)
}

func TestPagerPartialShowsTheRangeAndBothDirections(t *testing.T) {
	page := fstest.MapFS{"page.html": {Data: []byte(
		`{{define "title"}}Pager{{end}}{{define "content"}}{{template "pager" .Page}}{{end}}`,
	)}}

	app := fiber.New()
	app.Get("/things", func(ctx fiber.Ctx) error {
		pager, _ := pages.NewPager(ctx, pagerParam, 2, 60)
		return pages.Render(ctx, fiber.StatusOK, page, "page.html", pager)
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/things?"+pagerParam+"=2", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Contains(t, string(body), `<div class="sheet-foot">`)
	assert.Contains(t, string(body), `<span>showing 26–50 of 60</span>`)
	assert.Contains(t, string(body), `<a href="/things?versions_page=1" rel="prev">Previous</a>`)
	assert.Contains(t, string(body), `<span aria-current="page">2</span>`)
	assert.Contains(t, string(body), `<a href="/things?versions_page=3" rel="next">Next</a>`)
}

func TestPagerPartialTurnsOffTheDirectionsThatDoNotExist(t *testing.T) {
	page := fstest.MapFS{"page.html": {Data: []byte(
		`{{define "title"}}Pager{{end}}{{define "content"}}{{template "pager" .Page}}{{end}}`,
	)}}

	app := fiber.New()
	app.Get("/things", func(ctx fiber.Ctx) error {
		pager, _ := pages.NewPager(ctx, pagerParam, 1, 3)
		return pages.Render(ctx, fiber.StatusOK, page, "page.html", pager)
	})

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/things", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Contains(t, string(body), `<span>showing 1–3 of 3</span>`)
	assert.Contains(t, string(body), `<span class="off">Previous</span>`)
	assert.Contains(t, string(body), `<span class="off">Next</span>`)
}

func returnPageFor(t *testing.T, method, target, body string) (page int64, ok bool) {
	t.Helper()

	app := fiber.New()
	app.Add([]string{method}, "/users/1/edit", func(ctx fiber.Ctx) error {
		page, ok = pages.ReturnPage(ctx, "page")
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	_, err := app.Test(request)
	require.NoError(t, err)

	return page, ok
}

func TestReturnPageIsZeroWhenThePageIsAbsent(t *testing.T) {
	page, ok := returnPageFor(t, fiber.MethodGet, "/users/1/edit", "")
	assert.True(t, ok)
	assert.Zero(t, page)

	page, ok = returnPageFor(t, fiber.MethodPost, "/users/1/edit?page=3", "email=ana%40x.com")
	assert.True(t, ok, "a POST reads its form, not the query")
	assert.Zero(t, page)
}

func TestReturnPageReadsTheQueryOnAGetAndTheFormOnAPost(t *testing.T) {
	page, ok := returnPageFor(t, fiber.MethodGet, "/users/1/edit?page=3", "")
	assert.True(t, ok)
	assert.Equal(t, int64(3), page)

	page, ok = returnPageFor(t, fiber.MethodPost, "/users/1/edit", "page=4")
	assert.True(t, ok)
	assert.Equal(t, int64(4), page)
}

func TestReturnPageRejectsANonCanonicalValue(t *testing.T) {
	for _, raw := range []string{"", "0", "01", "%2B1", "abc"} {
		_, ok := returnPageFor(t, fiber.MethodGet, "/users/1/edit?page="+raw, "")
		assert.False(t, ok, "GET %q", raw)

		_, ok = returnPageFor(t, fiber.MethodPost, "/users/1/edit", "page="+raw)
		assert.False(t, ok, "POST %q", raw)
	}
}

func TestReturnPageRefusesAPageWhoseOffsetWouldOverflow(t *testing.T) {
	largest := int64(math.MaxInt64 / pages.PageSize)

	page, ok := returnPageFor(t, fiber.MethodGet, "/users/1/edit?page="+strconv.FormatInt(largest, 10), "")
	require.True(t, ok)
	assert.Equal(t, largest, page)
	assert.Positive(t, pages.Offset(page))

	_, ok = returnPageFor(t, fiber.MethodGet, "/users/1/edit?page="+strconv.FormatInt(largest+1, 10), "")
	assert.False(t, ok, "GET")

	_, ok = returnPageFor(t, fiber.MethodPost, "/users/1/edit", "page="+strconv.FormatInt(largest+1, 10))
	assert.False(t, ok, "POST")
}

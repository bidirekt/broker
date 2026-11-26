package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing/fstest"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/breakreport"
	restbreakreport "github.com/bidirekt/broker/internal/components/breakreport/protocols/rest"
	"github.com/bidirekt/broker/internal/features/show_check"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

const (
	checkSectionsEndpointsStylesheet = `<link rel="stylesheet" href="/assets/protocols/endpoints.css">`
	checkSectionsProbesStylesheet    = `<link rel="stylesheet" href="/assets/protocols/probes.css">`
)

const checkSectionsAPIContract = `provides:
  rest:
    /pets/*:
      get:
        responses:
          200: ~
`

const checkSectionsReviewsContract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
`

const checkSectionsBreakingWebContract = `consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            404: ~
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
`

const checkSectionsCompatibleWebContract = `consumes:
  petstore_api:
    rest:
      /pets/*:
        get:
          responses:
            200: ~
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
`

var checkSectionsProbeFiles = fstest.MapFS{
	"probe_breaks.html": {Data: []byte(`{{define "probe_breaks"}}<p class="group-label">Probes</p>{{range .}}<p class="probe-break">{{template "icon_x" 14}}<span>{{.}}</span></p>{{end}}{{end}}`)},
	"probes.css":        {Data: []byte(`.probe-break{display:flex}`)},
}

type checkSectionsProbe struct{}

func (this checkSectionsProbe) Protocol() model.Protocol {
	return "probe"
}

func (this checkSectionsProbe) ReportKey() string {
	return "probes"
}

func (this checkSectionsProbe) ToSection(_ breakreport.CheckedCounterpart, tree json.RawMessage) (breakreport.Section, error) {
	var breaks []string
	if tree != nil {
		if err := json.Unmarshal(tree, &breaks); err != nil {
			return breakreport.Section{}, err
		}
	}
	return breakreport.Section{Display: breaks, Breaks: len(breaks)}, nil
}

func (this checkSectionsProbe) Files() fs.FS {
	return checkSectionsProbeFiles
}

func (this checkSectionsProbe) Template() string {
	return "probe_breaks"
}

func (this *IntegrationSuite) mustPostForCheckSections(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) checkForCheckSections(webContract string) (checkPath string) {
	this.mustPostForCheckSections("/api/environments", `{"environment":"production"}`)
	for _, participant := range []string{"petstore_api", "petstore_reviews", "petstore_web"} {
		this.mustPostForCheckSections("/api/participants", `{"participant":"`+participant+`"}`)
	}
	this.mustPostForCheckSections("/api/contracts", this.publishBody("petstore_api", "1.4.0", contractFragment{"api.yaml", checkSectionsAPIContract}))
	this.mustPostForCheckSections("/api/deployments", `{"participant":"petstore_api","version":"1.4.0","environment":"production"}`)
	this.mustPostForCheckSections("/api/contracts", this.publishBody("petstore_reviews", "1.0.0", contractFragment{"reviews.yaml", checkSectionsReviewsContract}))
	this.mustPostForCheckSections("/api/deployments", `{"participant":"petstore_reviews","version":"1.0.0","environment":"production"}`)
	this.mustPostForCheckSections("/api/contracts", this.publishBody("petstore_web", "2.3.0", contractFragment{"web.yaml", webContract}))
	this.mustPostForCheckSections("/api/can-i-deploy", `{"participant":"petstore_web","version":"2.3.0","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return "/checks/" + strconv.FormatInt(checkID, 10)
}

func (this *IntegrationSuite) storeProbeBreaksForCheckSections(counterpart string, breaks ...string) {
	tree, err := json.Marshal(breaks)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(context.Background(),
		`UPDATE compatibility_checks SET deployable = false, report = jsonb_set(report, ARRAY[$1::text, 'probes'], $2::jsonb)`,
		counterpart, string(tree))
	this.Require().NoError(err)
	_, err = this.Pool.Exec(context.Background(), `UPDATE compatibility_check_results SET deployable = false WHERE counterpart_name = $1`, counterpart)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) getFromProbeCheckPage(path string) string {
	server := fiber.New()
	server.Use(func(ctx fiber.Ctx) error {
		access.GrantSession(ctx, access.Session{CSRFToken: "check-sections-token"})
		access.GrantPrincipal(ctx, access.Principal{UserID: this.adminID, Email: suiteAdminEmail, Permission: model.PermissionRead})
		return ctx.Next()
	})
	show_check.Register(
		&components.Components{Server: server, Pool: this.Pool},
		breakreport.NewReader([]breakreport.ProtocolSection{checkSectionsProbe{}, restbreakreport.New()}),
	)

	resp, err := server.Test(httptest.NewRequest("GET", path, nil), fiber.TestConfig{Timeout: 10 * time.Second})
	this.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	this.Require().NoError(err)
	this.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	return string(body)
}

func (this *IntegrationSuite) TestShowCheck_DrawsASecondRegisteredSectionWithItsTemplateAndItsCounterpartBreaks() {
	checkPath := this.checkForCheckSections(checkSectionsBreakingWebContract)
	this.storeProbeBreaksForCheckSections("petstore_reviews", "petstore_web pings petstore_reviews, but it stopped answering")

	body := this.getFromProbeCheckPage(checkPath)

	this.Contains(body, `<div><b>2 of 2</b><span>counterparts break</span></div>`)
	this.Equal(2, strings.Count(body, `<details class="cp" open>`))
	this.NotContains(body, `Also compared with, and compatible:`)

	reviewsAt := strings.Index(body, `<span class="who-cp">petstore_reviews</span>`)
	this.Require().Positive(reviewsAt)
	this.Contains(body[reviewsAt:], `no · 1 break</span></summary>`)
	this.Contains(body[reviewsAt:], `<p class="group-label">Probes</p><p class="probe-break"><svg viewBox="0 0 16 16" width="14" height="14"`)
	this.Contains(body[reviewsAt:], `<span>petstore_web pings petstore_reviews, but it stopped answering</span></p>`)
	this.Contains(body[:reviewsAt], `<p class="ix-name">response 404</p>`)
}

func (this *IntegrationSuite) TestShowCheck_LinksTheStylesheetOfEachSectionItDrawsOnceInsideTheContent() {
	checkPath := this.checkForCheckSections(checkSectionsBreakingWebContract)
	this.storeProbeBreaksForCheckSections("petstore_api", "petstore_web pings petstore_api, but it stopped answering")
	this.storeProbeBreaksForCheckSections("petstore_reviews", "petstore_web pings petstore_reviews, but it stopped answering")

	body := this.getFromProbeCheckPage(checkPath)

	this.Equal(2, strings.Count(body, `<p class="group-label">Probes</p>`))
	this.Equal(1, strings.Count(body, checkSectionsEndpointsStylesheet))
	this.Equal(1, strings.Count(body, checkSectionsProbesStylesheet))
	this.Less(strings.Index(body, `<main class="main">`), strings.Index(body, checkSectionsEndpointsStylesheet))
	this.Less(strings.Index(body, checkSectionsEndpointsStylesheet), strings.Index(body, checkSectionsProbesStylesheet))
	this.Less(strings.Index(body, checkSectionsProbesStylesheet), strings.Index(body, `<nav class="crumbs"`))
}

func (this *IntegrationSuite) TestShowCheck_PageWithoutRestBreaksDoesNotLinkTheRestStylesheet() {
	checkPath := this.checkForCheckSections(checkSectionsCompatibleWebContract)

	_, _, compatible := this.getPage(checkPath, this.signInAsAdmin())
	this.Contains(compatible, `yes · can be deployed`)
	this.NotContains(compatible, `/assets/protocols/`)

	this.storeProbeBreaksForCheckSections("petstore_reviews", "petstore_web pings petstore_reviews, but it stopped answering")
	probesOnly := this.getFromProbeCheckPage(checkPath)
	this.Contains(probesOnly, checkSectionsProbesStylesheet)
	this.NotContains(probesOnly, `/assets/protocols/endpoints.css`)
}

func (this *IntegrationSuite) TestShowCheck_BoostedNavigationFindsTheSectionStylesheetInTheSwappedBody() {
	checkPath := this.checkForCheckSections(checkSectionsBreakingWebContract)

	req := httptest.NewRequest("GET", checkPath, nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	status, _, body := this.sendPageRequest(req, this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	bodyAt := strings.Index(body, `<body `)
	this.Require().Positive(bodyAt)
	this.NotContains(body[:bodyAt], `/assets/protocols/`)
	this.Contains(body[bodyAt:strings.Index(body, `</body>`)], checkSectionsEndpointsStylesheet)
}

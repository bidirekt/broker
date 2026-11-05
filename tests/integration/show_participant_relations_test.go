package integration_test

import (
	"context"
	"net/http"
	"strings"

	"github.com/bidirekt/broker/internal/model"
)

const (
	relationsReaderEmail    = "reader@x.com"
	relationsReaderPassword = "reader-correct-horse-battery"
)

const relationsAPIContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: ~
`

const relationsReviewsV1Contract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: ~
`

const relationsReviewsV2Contract = `provides:
  rest:
    /reviews:
      get:
        responses:
          200: ~
consumes:
  petstore_admin:
    rest:
      /admin:
        get:
          responses:
            200: ~
`

const relationsWebV1Contract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: ~
`

const relationsWebV2Contract = `consumes:
  petstore_reviews:
    rest:
      /reviews:
        get:
          responses:
            200: ~
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: ~
      /pets/*:
        get:
          responses:
            200: ~
  petstore_admin:
    rest:
      /admin:
        get:
          responses:
            200: ~
  ghost:
    rest:
      /ghosts:
        get:
          responses:
            200: ~
`

const relationsFooter = `A can-i-deploy compares the version with the deployed counterparts that share an endpoint with it. A consumer counts only when it is deployed; a provider it consumes from that is not deployed is a break.`

func (this *IntegrationSuite) mustPostForRelations(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForRelations(participant, version, content string) {
	this.mustPostForRelations("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForRelations(participant, version, environment string) {
	this.mustPostForRelations("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"`+environment+`"}`)
}

func (this *IntegrationSuite) createForRelations(environments, participants []string) {
	for _, environment := range environments {
		this.mustPostForRelations("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range participants {
		this.mustPostForRelations("/api/participants", `{"participant":"`+participant+`"}`)
	}
}

func (this *IntegrationSuite) petstoreForRelations() {
	this.createForRelations([]string{"staging", "production"}, []string{"petstore_web", "petstore_api", "petstore_reviews", "petstore_admin"})
	this.publishForRelations("petstore_api", "1.4.0", relationsAPIContract)
	this.deployForRelations("petstore_api", "1.4.0", "production")
	this.publishForRelations("petstore_api", "1.5.0", relationsAPIContract+"    /pets/*:\n      get:\n        responses:\n          200: ~\n")
	this.deployForRelations("petstore_api", "1.5.0", "staging")
	this.publishForRelations("petstore_reviews", "1.0.0", relationsReviewsV1Contract)
	this.deployForRelations("petstore_reviews", "1.0.0", "production")
	this.publishForRelations("petstore_reviews", "1.1.0", relationsReviewsV2Contract)
	this.deployForRelations("petstore_reviews", "1.1.0", "staging")
	this.publishForRelations("petstore_web", "2.2.0", relationsWebV1Contract)
	this.deployForRelations("petstore_web", "2.2.0", "production")
	this.publishForRelations("petstore_web", "2.3.0", relationsWebV2Contract)
}

func (this *IntegrationSuite) TestShowParticipantRelations_ShowsTheHeaderTabsAndFooter() {
	this.petstoreForRelations()

	status, header, body := this.getPage("/participants/petstore_web/relations", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Equal("no-store", header.Get("Cache-Control"))
	this.Contains(body, `<h1 class="mono">petstore_web</h1>`)
	this.Contains(body, `<span>Latest <span class="ver">2.3.0</span></span>`)
	this.Contains(body, `<span class="badge env">production <span class="v">2.2.0</span></span>`)
	this.NotContains(body, `<span class="badge env">staging <span class="v">2.`)
	this.Contains(body, `<a href="/participants/petstore_web">Versions <span class="count">2</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/deployments">Deployments <span class="count">1</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/checks">Checks <span class="count">0</span></a>`)
	this.Contains(body, `<a href="/participants/petstore_web/relations" aria-current="page">Relations <span class="count">4</span></a>`)
	this.Contains(body, relationsFooter)
}

func (this *IntegrationSuite) TestShowParticipantRelations_ConsumesListsTheLatestVersionProvidersWithWhereTheyAreDeployed() {
	this.petstoreForRelations()

	_, _, body := this.getPage("/participants/petstore_web/relations", this.signInAsAdmin())

	this.Contains(body, `<span class="muted">from the latest version, 2.3.0</span>`)
	ghost := `<li><span class="name">ghost</span><span class="badges"><span class="badge quiet">not a participant</span></span></li>`
	admin := `<li><a class="name" href="/participants/petstore_admin">petstore_admin</a><span class="badges"><span class="badge quiet">never published</span></span></li>`
	api := `<li><a class="name" href="/participants/petstore_api">petstore_api</a><span class="badges"><span class="badge env">production <span class="v">1.4.0</span></span><span class="badge env">staging <span class="v">1.5.0</span></span></span></li>`
	reviews := `<li><a class="name" href="/participants/petstore_reviews">petstore_reviews</a><span class="badges"><span class="badge env">production <span class="v">1.0.0</span></span><span class="badge env">staging <span class="v">1.1.0</span></span></span></li>`
	for _, row := range []string{ghost, admin, api, reviews} {
		this.Equal(1, strings.Count(body, row), row)
	}
	this.Less(strings.Index(body, ghost), strings.Index(body, admin))
	this.Less(strings.Index(body, admin), strings.Index(body, api))
	this.Less(strings.Index(body, api), strings.Index(body, reviews))
	this.NotContains(body, `href="/participants/ghost"`)
	this.Contains(body, `No deployed participant consumes <span class="name">petstore_web</span>, so its checks compare only with providers.`)
}

func (this *IntegrationSuite) TestShowParticipantRelations_ConsumedByListsPerEnvironmentTheDeployedVersionsThatStillConsumeIt() {
	this.petstoreForRelations()

	_, _, body := this.getPage("/participants/petstore_api/relations", this.signInAsAdmin())

	this.Contains(body, `<span class="name">petstore_api</span> consumes nothing in its latest version.`)
	production := `<p class="pad"><span class="badge env">production</span></p>`
	reviews := `<li><a class="name" href="/participants/petstore_reviews">petstore_reviews</a><span class="ver">1.0.0</span></li>`
	web := `<li><a class="name" href="/participants/petstore_web">petstore_web</a><span class="ver">2.2.0</span></li>`
	this.Contains(body, production)
	this.Less(strings.Index(body, production), strings.Index(body, reviews))
	this.Less(strings.Index(body, reviews), strings.Index(body, web))
	this.NotContains(body, `<p class="pad"><span class="badge env">staging</span></p>`)
	this.NotContains(body, `petstore_web</a><span class="ver">2.3.0</span>`)
	this.Contains(body, `Relations <span class="count">2</span>`)
	this.NotContains(body, `href="/environments`)
}

func (this *IntegrationSuite) TestShowParticipantRelations_NeverPublishedParticipantStillShowsConsumedBy() {
	this.petstoreForRelations()

	status, _, body := this.getPage("/participants/petstore_admin/relations", this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="badge quiet">never published</span>`)
	this.NotContains(body, `Latest <span class="ver">`)
	this.NotContains(body, `from the latest version`)
	this.Contains(body, `<span class="name">petstore_admin</span> has never published a contract, so it consumes nothing yet.`)
	this.Contains(body, `<code class="cmd">bidirekt publish</code>`)
	this.Contains(body, `<p class="pad"><span class="badge env">staging</span></p>`)
	this.Contains(body, `<li><a class="name" href="/participants/petstore_reviews">petstore_reviews</a><span class="ver">1.1.0</span></li>`)
	this.NotContains(body, `<p class="pad"><span class="badge env">production</span></p>`)
	this.NotContains(body, `petstore_web</a><span class="ver">`)
	this.Contains(body, `Relations <span class="count">1</span>`)
	this.Contains(body, `Versions <span class="count">0</span>`)
}

func (this *IntegrationSuite) TestShowParticipantRelations_PublishedButUndeployedSaysNotDeployed() {
	this.createForRelations([]string{"production"}, []string{"petstore_web", "petstore_api"})
	this.publishForRelations("petstore_api", "1.4.0", relationsAPIContract)
	this.publishForRelations("petstore_web", "2.2.0", relationsWebV1Contract)

	_, _, body := this.getPage("/participants/petstore_web/relations", this.signInAsAdmin())

	this.Contains(body, `<span>Latest <span class="ver">2.2.0</span></span>`)
	this.Contains(body, `<a class="name" href="/participants/petstore_api">petstore_api</a><span class="badges"><span class="badge quiet">not deployed</span></span>`)
	this.Equal(2, strings.Count(body, `<span class="badge quiet">not deployed</span>`))
}

func (this *IntegrationSuite) TestShowParticipantRelations_ReadUserSeesThePageWithoutWriteActions() {
	this.petstoreForRelations()
	this.createActiveUser(relationsReaderEmail, model.PermissionRead, relationsReaderPassword)

	status, _, body := this.getPage("/participants/petstore_api/relations", this.signInAsUser(relationsReaderEmail, relationsReaderPassword))

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<h1 class="mono">petstore_api</h1>`)
	this.NotContains(body, `method="post" action="/participants`)
	this.NotContains(body, `/rename`)
}

func (this *IntegrationSuite) TestShowParticipantRelations_SignedOutIsRedirectedToSignIn() {
	this.petstoreForRelations()

	status, header, _ := this.getPage("/participants/petstore_api/relations", "")

	this.Equal(http.StatusSeeOther, status)
	this.Equal("/sign-in", header.Get("Location"))
}

func (this *IntegrationSuite) TestShowParticipantRelations_UnknownParticipantIsNotFound() {
	this.petstoreForRelations()

	status, header, body := this.getPage("/participants/ghost/relations", this.signInAsAdmin())

	this.Equal(http.StatusNotFound, status)
	this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
	this.Contains(body, "Not Found")
}

func (this *IntegrationSuite) TestShowParticipantRelations_ConsumedProviderIsIndexedForConsumes() {
	var definition string
	err := this.Pool.QueryRow(context.Background(),
		`SELECT indexdef FROM pg_indexes WHERE tablename = 'resources' AND indexdef LIKE '%(consumed_provider)%'`,
	).Scan(&definition)

	this.Require().NoError(err)
	this.Contains(definition, `WHERE (direction = 'consumes'::text)`)
}

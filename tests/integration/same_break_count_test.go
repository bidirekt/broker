package integration_test

import (
	"context"
	"net/http"
	"strconv"
)

const sameBreakCountAPIv1Contract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
    /pets/*:
      get:
        responses:
          200: Pet
          404: ~
    /pets/*/adopt:
      post:
        responses:
          204: ~
schemas:
  Pets:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      id: { type: string }
      status: { type: string }
`

const sameBreakCountWebContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pets
      /pets/*:
        get:
          responses:
            200: Pet
            404: ~
      /pets/*/adopt:
        post:
          responses:
            204: ~
schemas:
  Pets:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      id: { type: string }
      status: { type: string }
`

const sameBreakCountAPIv2Contract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pets
    /pets/*:
      get:
        responses:
          200: Pet
schemas:
  Pets:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      id: { type: string }
`

func (this *IntegrationSuite) mustPostForSameBreakCount(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishForSameBreakCount(participant, version, content string) {
	this.mustPostForSameBreakCount("/api/contracts", this.publishBody(participant, version, contractFragment{"contract.yaml", content}))
}

func (this *IntegrationSuite) deployForSameBreakCount(participant, version string) {
	this.mustPostForSameBreakCount("/api/deployments",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) TestSameBreakCount_OneCheckShowsTheSameBreakCountOnEveryScreen() {
	this.mustPostForSameBreakCount("/api/environments", `{"environment":"production"}`)
	this.mustPostForSameBreakCount("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForSameBreakCount("/api/participants", `{"participant":"petstore_web"}`)
	this.publishForSameBreakCount("petstore_api", "1.0.0", sameBreakCountAPIv1Contract)
	this.deployForSameBreakCount("petstore_api", "1.0.0")
	this.publishForSameBreakCount("petstore_web", "2.0.0", sameBreakCountWebContract)
	this.deployForSameBreakCount("petstore_web", "2.0.0")
	this.publishForSameBreakCount("petstore_api", "2.0.0", sameBreakCountAPIv2Contract)
	this.mustPostForSameBreakCount("/api/can-i-deploy", `{"participant":"petstore_api","version":"2.0.0","environment":"production"}`)

	var checkID, environmentID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id, environment_id FROM compatibility_checks`).Scan(&checkID, &environmentID))
	check := strconv.FormatInt(checkID, 10)
	cookie := this.signInAsAdmin()

	status, _, checkPage := this.getPage("/checks/"+check, cookie)
	this.Require().Equal(http.StatusOK, status, checkPage)
	this.Contains(checkPage, `<span class="prop">FIELD</span><span>Pet.status</span><span class="badge">2 places</span>`)
	this.Contains(checkPage, `<b class="name">petstore_api</b> now answers <code>GET /pets/*</code> only with 200, but <b class="name">petstore_web</b> still reads response 404`)
	this.Contains(checkPage, `<b class="name">petstore_api</b> removed <code>POST /pets/*/adopt</code>, but <b class="name">petstore_web</b> still calls it`)
	this.Contains(checkPage, `<div><b>3</b><span>breaks</span></div>`)

	screens := map[string]string{
		"/participants/petstore_api/checks":                     `no · 3 breaks</span>`,
		"/participants/petstore_api/versions/2.0.0":             `no · 3 breaks</span>`,
		"/environments/" + strconv.FormatInt(environmentID, 10): `no · 3 breaks</span>`,
		"/compatibility?participant=petstore_api":               `cannot deploy · 3 breaks</a>`,
		"/": `3 breaks · check #` + check + `</a>`,
	}
	for path, count := range screens {
		status, _, body := this.getPage(path, cookie)
		this.Require().Equal(http.StatusOK, status, path)
		this.Contains(body, count, path)
	}

	_, _, overview := this.getPage("/", cookie)
	this.Contains(overview, `can-i-deploy no · 3 breaks</a>`)
}

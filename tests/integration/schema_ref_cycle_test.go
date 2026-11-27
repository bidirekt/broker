package integration_test

import (
	"encoding/json"
	"net/http"
)

const refCycleRESTContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: A
schemas:
  A: {ref: B}
  B: {ref: A}
`

const refCycleGraphQLContract = `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: A, returns: Pet}
schemas:
  A: {ref: B}
  B: {ref: A}
  Pet: {type: object, properties: {id: {type: string}}}
`

const refCycleViolations = `{"message":"contract validation failed","violations":[` +
	`{"code":"schema.ref_cycle","path":"schemas;A","source":"petstore_web.yaml","details":{"schemas":["A","B"]}}` +
	`]}`

type refCycleFile struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

func (this *IntegrationSuite) mustPostForRefCycle(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) validateRefCycle(contract string) (status int, body string) {
	request, err := json.Marshal(struct {
		Participant string         `json:"participant"`
		Environment string         `json:"environment"`
		Contracts   []refCycleFile `json:"contracts"`
	}{Participant: "petstore_web", Environment: "production", Contracts: []refCycleFile{{"petstore_web.yaml", contract}}})
	this.Require().NoError(err)

	return this.post("/api/contracts/validate", string(request))
}

func (this *IntegrationSuite) TestPublishAndValidate_SchemasThatOnlyReferToEachOther_AreA400InEveryProtocol() {
	this.mustPostForRefCycle("/api/environments", `{"environment":"production"}`)
	this.mustPostForRefCycle("/api/participants", `{"participant":"petstore_web"}`)

	for name, contract := range map[string]string{"rest": refCycleRESTContract, "graphql": refCycleGraphQLContract} {
		status, body := this.post("/api/contracts", this.publishBody("petstore_web", "1.0.0", contractFragment{"petstore_web.yaml", contract}))
		this.Equal(http.StatusBadRequest, status, name)
		this.JSONEq(refCycleViolations, body, name)

		status, body = this.validateRefCycle(contract)
		this.Equal(http.StatusBadRequest, status, name)
		this.JSONEq(refCycleViolations, body, name)
	}

	this.Equal(0, this.countRows("contracts"))
}

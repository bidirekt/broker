package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

const returnsRootProviderSDL = `type Query {
  age: Int!
}
`

const returnsRootConsumer = `consumes:
  petstore_api:
    graphql:
      query:
        age: {returns: Age}
schemas:
  Age: {type: string}
`

func (this *IntegrationSuite) mustPostForReturnsRoot(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForReturnsRoot(participant, version string) (results string, checkPath string) {
	status, body := this.post("/api/can-i-deploy", `{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equal(http.StatusOK, status, body)

	var response struct {
		Results json.RawMessage `json:"results"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	return string(response.Results), "/checks/" + strconv.FormatInt(checkID, 10)
}

func (this *IntegrationSuite) TestCanIDeployAndShowCheck_ABreakAtTheRootOfWhatAFieldReturns_NamesTheFieldByItsCoordinate() {
	this.mustPostForReturnsRoot("/api/environments", `{"environment":"production"}`)
	this.mustPostForReturnsRoot("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForReturnsRoot("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForReturnsRoot("/api/contracts", this.publishBody("petstore_api", "1.0.0", contractFragment{"schema.graphqls", returnsRootProviderSDL}))
	this.mustPostForReturnsRoot("/api/deployments", `{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForReturnsRoot("/api/contracts", this.publishBody("petstore_web", "1.0.0", contractFragment{"web.yaml", returnsRootConsumer}))

	results, _ := this.canIDeployForReturnsRoot("petstore_web", "1.0.0")
	this.JSONEq(`{"petstore_api": {
		"deployable": false,
		"participantVersion": "1.0.0",
		"operations": {"query": {"age": {"returns": [
			{"reason": "property_type_mismatch", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "petstore_api", "property": "$", "consumerPropertyType": "String!", "providerPropertyType": "Int!", "consumerField": "Query.age", "providerField": "Query.age"}}
		]}}}
	}}`, results)

	this.mustPostForReturnsRoot("/api/deployments", `{"participant":"petstore_web","version":"1.0.0","environment":"production"}`)
	this.mustPostForReturnsRoot("/api/contracts", this.publishBody("petstore_api", "2.0.0", contractFragment{"schema.graphqls", returnsRootProviderSDL}))

	_, checkPath := this.canIDeployForReturnsRoot("petstore_api", "2.0.0")
	status, _, body := this.getPage(checkPath, this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, body)

	this.Contains(body, `<span class="op-prop">FIELD</span><span>Query.age</span><span class="badge">1 place</span>`)
	this.Contains(body, `<li><span class="op-kind">query</span><span>age</span><span class="at">returns</span><span>at &#34;$&#34;</span></li>`)
	this.NotContains(body, `<span class="cf">`)
	this.NotContains(body, `:returns`)
	this.NotContains(body, `:arguments`)
}

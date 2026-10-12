package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const exponentialSchemaLevels = 22

const exponentialSchemaBudget = time.Second

func exponentialSchemaCatalog() string {
	var catalog strings.Builder
	catalog.WriteString("schemas:\n")
	for level := 0; level < exponentialSchemaLevels; level++ {
		fmt.Fprintf(&catalog, "  S%d:\n    type: object\n    properties:\n      left:\n        ref: S%d\n      right:\n        ref: S%d\n",
			level, level+1, level+1)
	}
	fmt.Fprintf(&catalog, "  S%d:\n    type: string\n", exponentialSchemaLevels)

	return catalog.String()
}

func exponentialSchemaProviderContract() string {
	return "provides:\n  rest:\n    /tree:\n      get:\n        responses:\n          200: S0\n" + exponentialSchemaCatalog()
}

func exponentialSchemaConsumerContract() string {
	return "consumes:\n  tree_api:\n    rest:\n      /tree:\n        get:\n          responses:\n            200: S0\n" + exponentialSchemaCatalog()
}

func (this *IntegrationSuite) timedPostForExponentialSchema(label, path, body string) string {
	start := time.Now()
	status, response := this.post(path, body)
	elapsed := time.Since(start)

	this.Require().Equalf(http.StatusOK, status, "%s: %s", label, response)
	this.Lessf(elapsed, exponentialSchemaBudget, "%s took %s", label, elapsed)

	return response
}

func (this *IntegrationSuite) mustPostForExponentialSchema(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) TestExponentialSchema_PublishCanIDeployAndValidate_EachUnderOneSecond() {
	this.mustPostForExponentialSchema("/api/environments", `{"environment":"production"}`)
	this.mustPostForExponentialSchema("/api/participants", `{"participant":"tree_api"}`)
	this.mustPostForExponentialSchema("/api/participants", `{"participant":"tree_web"}`)

	this.timedPostForExponentialSchema("provider publish", "/api/contracts",
		this.publishBody("tree_api", "1", contractFragment{"api.yaml", exponentialSchemaProviderContract()}))
	this.timedPostForExponentialSchema("consumer publish", "/api/contracts",
		this.publishBody("tree_web", "1", contractFragment{"web.yaml", exponentialSchemaConsumerContract()}))
	this.mustPostForExponentialSchema("/api/deployments", `{"participant":"tree_api","version":"1","environment":"production"}`)

	canIDeploy := this.timedPostForExponentialSchema("can-i-deploy", "/api/can-i-deploy",
		`{"participant":"tree_web","version":"1","environment":"production"}`)
	this.True(this.exponentialSchemaDeployable(canIDeploy), canIDeploy)
	this.Contains(canIDeploy, `"tree_api"`)

	validateBody, err := json.Marshal(struct {
		Participant string             `json:"participant"`
		Environment string             `json:"environment"`
		Contracts   []contractFragment `json:"contracts"`
	}{"tree_web", "production", []contractFragment{{"web.yaml", exponentialSchemaConsumerContract()}}})
	this.Require().NoError(err)
	validate := this.timedPostForExponentialSchema("validate", "/api/contracts/validate", string(validateBody))
	this.True(this.exponentialSchemaDeployable(validate), validate)
	this.Contains(validate, `"tree_api"`)

	var providerSchemas int
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM schemas JOIN participants ON participants.id = schemas.participant_id
		  WHERE participants.name = 'tree_api'`).Scan(&providerSchemas))
	this.Equal(exponentialSchemaLevels+1, providerSchemas)
}

func (this *IntegrationSuite) exponentialSchemaDeployable(body string) bool {
	var response struct {
		Deployable bool `json:"deployable"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	return response.Deployable
}

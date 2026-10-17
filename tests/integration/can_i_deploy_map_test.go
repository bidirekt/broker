package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
)

type mapLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type mapResult struct {
	Deployable bool                                       `json:"deployable"`
	Endpoints  map[string]map[string]map[string][]mapLeaf `json:"endpoints"`
}

type mapResponse struct {
	Deployable bool                 `json:"deployable"`
	Results    map[string]mapResult `json:"results"`
}

const mapCatalogAPIContract = `provides:
  rest:
    /api/catalog:
      get:
        responses:
          200: Catalog
schemas:
  Catalog:
    type: object
    properties:
      prices:
        PRICES_SHAPE
`

const mapCatalogSPAContract = `consumes:
  catalog_api:
    rest:
      /api/catalog:
        get:
          responses:
            200: Catalog
schemas:
  Catalog:
    type: object
    properties:
      prices:
        type: map
        values:
          type: object
          properties:
            amount: { type: integer }
`

const (
	mapPricesAsAMap = `type: map
        values:
          type: object
          properties:
            amount: { type: AMOUNT_TYPE }`
	mapPricesAsAMapOfStrings = `type: map
        values: { type: string }`
	mapPricesAsAnObject = `type: object
        properties:
          usd:
            type: object
            properties:
              amount: { type: integer }`
)

func mapCatalogAPI(pricesShape string) string {
	return strings.Replace(mapCatalogAPIContract, "PRICES_SHAPE", pricesShape, 1)
}

func (this *IntegrationSuite) mustPostForMap(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForMap(participant, version string) mapResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var parsed mapResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) deployMapCatalog() {
	this.mustPostForMap("/api/environments", `{"environment":"production"}`)
	this.mustPostForMap("/api/participants", `{"participant":"catalog_api"}`)
	this.mustPostForMap("/api/participants", `{"participant":"catalog_spa"}`)
	this.mustPostForMap("/api/contracts", this.publishBody("catalog_api", "1.0.0",
		contractFragment{"api.yaml", mapCatalogAPI(strings.Replace(mapPricesAsAMap, "AMOUNT_TYPE", "integer", 1))}))
	this.mustPostForMap("/api/deployments", `{"participant":"catalog_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForMap("/api/contracts", this.publishBody("catalog_spa", "1.0.0",
		contractFragment{"spa.yaml", mapCatalogSPAContract}))
	this.mustPostForMap("/api/deployments", `{"participant":"catalog_spa","version":"1.0.0","environment":"production"}`)
}

func (this *IntegrationSuite) providerBreaksOfMapCatalog(version, pricesShape string) map[string]map[string]map[string][]mapLeaf {
	this.mustPostForMap("/api/contracts", this.publishBody("catalog_api", version,
		contractFragment{"api.yaml", mapCatalogAPI(pricesShape)}))

	got := this.canIDeployForMap("catalog_api", version)
	this.False(got.Deployable)

	return got.Results["catalog_spa"].Endpoints
}

func mapCatalogBreak(property, consumerField, providerField, consumerType, providerType string) map[string]map[string]map[string][]mapLeaf {
	return map[string]map[string]map[string][]mapLeaf{
		"/api/catalog": {"get": {"200": {{
			Reason: "property_type_mismatch",
			Role:   "provider",
			Details: map[string]string{
				"property":             property,
				"consumerField":        consumerField,
				"providerField":        providerField,
				"consumerName":         "catalog_spa",
				"providerName":         "catalog_api",
				"consumerPropertyType": consumerType,
				"providerPropertyType": providerType,
			},
		}}}},
	}
}

func (this *IntegrationSuite) TestCanIDeploy_MapReadAgainstAMap_IsDeployable() {
	this.deployMapCatalog()

	fromTheConsumer := this.canIDeployForMap("catalog_spa", "1.0.0")
	this.True(fromTheConsumer.Deployable)
	this.Empty(fromTheConsumer.Results["catalog_api"].Endpoints)

	fromTheProvider := this.canIDeployForMap("catalog_api", "1.0.0")
	this.True(fromTheProvider.Deployable)
	this.Empty(fromTheProvider.Results["catalog_spa"].Endpoints)
}

func (this *IntegrationSuite) TestCanIDeploy_MapValuesOfAnotherType_BreakOnTheValues() {
	this.deployMapCatalog()

	this.Equal(
		mapCatalogBreak("$.prices{}", "Catalog.prices{}", "Catalog.prices{}", "object", "string"),
		this.providerBreaksOfMapCatalog("2.0.0", mapPricesAsAMapOfStrings),
	)
}

func (this *IntegrationSuite) TestCanIDeploy_MapValueMemberOfAnotherType_BreaksOnTheMember() {
	this.deployMapCatalog()

	this.Equal(
		mapCatalogBreak("$.prices{}.amount", "Catalog.prices{}.amount", "Catalog.prices{}.amount", "integer", "string"),
		this.providerBreaksOfMapCatalog("2.0.0", strings.Replace(mapPricesAsAMap, "AMOUNT_TYPE", "string", 1)),
	)
}

func (this *IntegrationSuite) TestCanIDeploy_MapReadAgainstAnObject_BreaksOnTheParentOnly() {
	this.deployMapCatalog()

	this.Equal(
		mapCatalogBreak("$.prices", "Catalog.prices", "Catalog.prices", "map<object>", "object"),
		this.providerBreaksOfMapCatalog("2.0.0", mapPricesAsAnObject),
	)
}

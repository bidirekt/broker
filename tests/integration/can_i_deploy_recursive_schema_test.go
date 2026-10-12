package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
)

type recursiveSchemaLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type recursiveSchemaResult struct {
	Deployable bool                                                   `json:"deployable"`
	Endpoints  map[string]map[string]map[string][]recursiveSchemaLeaf `json:"endpoints"`
}

type recursiveSchemaResponse struct {
	Deployable bool                             `json:"deployable"`
	Results    map[string]recursiveSchemaResult `json:"results"`
}

const recursiveSchemaAPIContract = `provides:
  rest:
    /comments/*:
      get:
        responses:
          200: Comment
    /posts/*:
      get:
        responses:
          200: Post
    /loops:
      get:
        responses:
          200: Loop
schemas:
  Comment:
    type: object
    properties:
      text: { type: TEXT_TYPE }
      replies:
        type: array
        items: { ref: Comment }
  Post:
    type: object
    properties:
      title: { type: string }
      author: { ref: User }
  User:
    type: object
    properties:
      name: { type: string }
      posts:
        type: array
        items: { ref: Post }
  Loop:
    ref: Loop
`

const recursiveSchemaSPAContract = `consumes:
  threads_api:
    rest:
      /comments/*:
        get:
          responses:
            200: Comment
      /posts/*:
        get:
          responses:
            200: Post
      /loops:
        get:
          responses:
            200: Loop
schemas:
  Comment:
    type: object
    properties:
      text: { type: string }
      replies:
        type: array
        items: { ref: Comment }
  Post:
    type: object
    properties:
      title: { type: string }
      author: { ref: User }
  User:
    type: object
    properties:
      name: { type: string }
      posts:
        type: array
        items: { ref: Post }
  Loop:
    ref: Loop
`

func recursiveSchemaAPI(textType string) string {
	return strings.Replace(recursiveSchemaAPIContract, "TEXT_TYPE", textType, 1)
}

func (this *IntegrationSuite) mustPostForRecursiveSchema(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) checkForRecursiveSchema(path, body string) recursiveSchemaResponse {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)

	var parsed recursiveSchemaResponse
	this.Require().NoError(json.Unmarshal([]byte(response), &parsed))

	return parsed
}

func (this *IntegrationSuite) canIDeployForRecursiveSchema(participant, version string) recursiveSchemaResponse {
	return this.checkForRecursiveSchema("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
}

func (this *IntegrationSuite) validateForRecursiveSchema(participant, content string) recursiveSchemaResponse {
	return this.checkForRecursiveSchema("/api/contracts/validate",
		this.validateBody(participant, "production", validateFile{Source: "contract.yaml", Content: content}))
}

func (this *IntegrationSuite) setupRecursiveSchemaPair() {
	this.mustPostForRecursiveSchema("/api/environments", `{"environment":"production"}`)
	this.mustPostForRecursiveSchema("/api/participants", `{"participant":"threads_api"}`)
	this.mustPostForRecursiveSchema("/api/participants", `{"participant":"threads_spa"}`)
	this.mustPostForRecursiveSchema("/api/contracts",
		this.publishBody("threads_api", "1.0.0", contractFragment{"api.yaml", recursiveSchemaAPI("string")}))
	this.mustPostForRecursiveSchema("/api/deployments",
		`{"participant":"threads_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForRecursiveSchema("/api/contracts",
		this.publishBody("threads_spa", "1.0.0", contractFragment{"spa.yaml", recursiveSchemaSPAContract}))
	this.mustPostForRecursiveSchema("/api/deployments",
		`{"participant":"threads_spa","version":"1.0.0","environment":"production"}`)
}

func (this *IntegrationSuite) TestCanIDeploy_SameRecursiveSchemasOnBothSides_AreDeployable() {
	this.setupRecursiveSchemaPair()

	fromTheConsumer := this.canIDeployForRecursiveSchema("threads_spa", "1.0.0")
	this.True(fromTheConsumer.Deployable)
	this.Empty(fromTheConsumer.Results["threads_api"].Endpoints)

	fromTheProvider := this.canIDeployForRecursiveSchema("threads_api", "1.0.0")
	this.True(fromTheProvider.Deployable)
	this.Empty(fromTheProvider.Results["threads_spa"].Endpoints)

	validated := this.validateForRecursiveSchema("threads_spa", recursiveSchemaSPAContract)
	this.True(validated.Deployable)
	this.Empty(validated.Results["threads_api"].Endpoints)
}

func (this *IntegrationSuite) TestCanIDeploy_DifferenceInsideARecursiveSchema_BreaksOnce() {
	this.setupRecursiveSchemaPair()

	expected := func(role string) map[string]map[string]map[string][]recursiveSchemaLeaf {
		return map[string]map[string]map[string][]recursiveSchemaLeaf{
			"/comments/*": {"get": {"200": {{
				Reason: "property_type_mismatch",
				Role:   role,
				Details: map[string]string{
					"property":             "$.text",
					"consumerField":        "Comment.text",
					"providerField":        "Comment.text",
					"consumerName":         "threads_spa",
					"providerName":         "threads_api",
					"consumerPropertyType": "string",
					"providerPropertyType": "integer",
				},
			}}}},
		}
	}

	validated := this.validateForRecursiveSchema("threads_api", recursiveSchemaAPI("integer"))
	this.False(validated.Deployable)
	this.Equal(expected("provider"), validated.Results["threads_spa"].Endpoints)

	this.mustPostForRecursiveSchema("/api/contracts",
		this.publishBody("threads_api", "2.0.0", contractFragment{"api.yaml", recursiveSchemaAPI("integer")}))

	published := this.canIDeployForRecursiveSchema("threads_api", "2.0.0")
	this.False(published.Deployable)
	this.Equal(expected("provider"), published.Results["threads_spa"].Endpoints)
}

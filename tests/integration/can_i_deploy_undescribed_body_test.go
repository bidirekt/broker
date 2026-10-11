package integration_test

import (
	"encoding/json"
	"net/http"
)

type undescribedBodyLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type undescribedBodyResult struct {
	Deployable bool                                                   `json:"deployable"`
	Endpoints  map[string]map[string]map[string][]undescribedBodyLeaf `json:"endpoints"`
}

type undescribedBodyResponse struct {
	Deployable bool                             `json:"deployable"`
	Results    map[string]undescribedBodyResult `json:"results"`
}

type undescribedBodyFile struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

const undescribedDownloadProviderYAML = `provides:
  rest:
    /files/*:
      get:
        responses:
          200: ~
`

const describedDownloadProviderYAML = `provides:
  rest:
    /files/*:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const undescribedDownloadConsumerYAML = `consumes:
  files_api:
    rest:
      /files/*:
        get:
          responses:
            200: ~
`

const describedDownloadConsumerYAML = `consumes:
  files_api:
    rest:
      /files/*:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

const undescribedUploadProviderYAML = `provides:
  rest:
    /files:
      post:
        request: ~
`

const uploadWithoutRequestProviderYAML = `provides:
  rest:
    /files:
      post:
        responses:
          201: ~
`

const describedUploadConsumerYAML = `consumes:
  files_api:
    rest:
      /files:
        post:
          request: NewFile
schemas:
  NewFile:
    type: object
    properties:
      name:
        type: string
`

func (this *IntegrationSuite) mustPostForUndescribedBodyTable(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) checkForUndescribedBodyTable(path, body string) undescribedBodyResponse {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)

	var parsed undescribedBodyResponse
	this.Require().NoError(json.Unmarshal([]byte(response), &parsed))

	return parsed
}

func (this *IntegrationSuite) canIDeployAndValidateForUndescribedBody(participant, content string) (canIDeploy, validate undescribedBodyResponse) {
	canIDeploy = this.checkForUndescribedBodyTable("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"1","environment":"production"}`)

	body, err := json.Marshal(struct {
		Participant string                `json:"participant"`
		Environment string                `json:"environment"`
		Contracts   []undescribedBodyFile `json:"contracts"`
	}{participant, "production", []undescribedBodyFile{{"api.yaml", content}}})
	this.Require().NoError(err)

	validate = this.checkForUndescribedBodyTable("/api/contracts/validate", string(body))

	return canIDeploy, validate
}

func (this *IntegrationSuite) publishAndDeployProviderForUndescribedBody(providerYAML, consumerYAML string) {
	this.mustPostForUndescribedBodyTable("/api/environments", `{"environment":"production"}`)
	this.mustPostForUndescribedBodyTable("/api/participants", `{"participant":"files_api"}`)
	this.mustPostForUndescribedBodyTable("/api/participants", `{"participant":"files_web"}`)
	this.mustPostForUndescribedBodyTable("/api/contracts",
		this.publishBody("files_api", "1", contractFragment{"api.yaml", providerYAML}))
	this.mustPostForUndescribedBodyTable("/api/deployments",
		`{"participant":"files_api","version":"1","environment":"production"}`)
	this.mustPostForUndescribedBodyTable("/api/contracts",
		this.publishBody("files_web", "1", contractFragment{"api.yaml", consumerYAML}))
}

func (this *IntegrationSuite) assertUndescribedBodyCompatibleFromBothSides(providerYAML, consumerYAML string) {
	this.publishAndDeployProviderForUndescribedBody(providerYAML, consumerYAML)

	fromConsumer, validatedConsumer := this.canIDeployAndValidateForUndescribedBody("files_web", consumerYAML)
	for name, response := range map[string]undescribedBodyResponse{"can-i-deploy": fromConsumer, "validate": validatedConsumer} {
		this.True(response.Deployable, "consumer %s", name)
		this.True(response.Results["files_api"].Deployable, "consumer %s", name)
		this.Empty(response.Results["files_api"].Endpoints, "consumer %s", name)
	}

	this.mustPostForUndescribedBodyTable("/api/deployments",
		`{"participant":"files_web","version":"1","environment":"production"}`)

	fromProvider, validatedProvider := this.canIDeployAndValidateForUndescribedBody("files_api", providerYAML)
	for name, response := range map[string]undescribedBodyResponse{"can-i-deploy": fromProvider, "validate": validatedProvider} {
		this.True(response.Deployable, "provider %s", name)
		this.True(response.Results["files_web"].Deployable, "provider %s", name)
		this.Empty(response.Results["files_web"].Endpoints, "provider %s", name)
	}
}

func (this *IntegrationSuite) TestCanIDeploy_UndescribedResponseOnBothSides_IsCompatible() {
	this.assertUndescribedBodyCompatibleFromBothSides(undescribedDownloadProviderYAML, undescribedDownloadConsumerYAML)
}

func (this *IntegrationSuite) TestCanIDeploy_DescribedResponseReadAsUndescribed_IsCompatible() {
	this.assertUndescribedBodyCompatibleFromBothSides(describedDownloadProviderYAML, undescribedDownloadConsumerYAML)
}

func (this *IntegrationSuite) TestCanIDeploy_UndescribedRequestAcceptingADescribedOne_IsCompatible() {
	this.assertUndescribedBodyCompatibleFromBothSides(undescribedUploadProviderYAML, describedUploadConsumerYAML)
}

func (this *IntegrationSuite) TestCanIDeploy_UndescribedResponseReadAsDescribed_BreaksOnceAtTheBody() {
	this.publishAndDeployProviderForUndescribedBody(undescribedDownloadProviderYAML, describedDownloadConsumerYAML)

	details := map[string]string{
		"property":     "$",
		"consumerName": "files_web",
		"providerName": "files_api",
		"propertyType": "object",
	}

	fromConsumer, validatedConsumer := this.canIDeployAndValidateForUndescribedBody("files_web", describedDownloadConsumerYAML)
	for name, response := range map[string]undescribedBodyResponse{"can-i-deploy": fromConsumer, "validate": validatedConsumer} {
		this.False(response.Deployable, "consumer %s", name)
		this.Equal(map[string]map[string]map[string][]undescribedBodyLeaf{
			"/files/*": {"get": {"200": {{Reason: "property_missing_in_provider", Role: "consumer", Details: details}}}},
		}, response.Results["files_api"].Endpoints, "consumer %s", name)
	}

	this.mustPostForUndescribedBodyTable("/api/deployments",
		`{"participant":"files_web","version":"1","environment":"production"}`)

	fromProvider, validatedProvider := this.canIDeployAndValidateForUndescribedBody("files_api", undescribedDownloadProviderYAML)
	for name, response := range map[string]undescribedBodyResponse{"can-i-deploy": fromProvider, "validate": validatedProvider} {
		this.False(response.Deployable, "provider %s", name)
		this.Equal(map[string]map[string]map[string][]undescribedBodyLeaf{
			"/files/*": {"get": {"200": {{Reason: "property_missing_in_provider", Role: "provider", Details: details}}}},
		}, response.Results["files_web"].Endpoints, "provider %s", name)
	}
}

func (this *IntegrationSuite) TestCanIDeploy_RequestOmittedByTheProvider_IsProviderResourceNotFound() {
	this.publishAndDeployProviderForUndescribedBody(uploadWithoutRequestProviderYAML, describedUploadConsumerYAML)

	fromConsumer, validatedConsumer := this.canIDeployAndValidateForUndescribedBody("files_web", describedUploadConsumerYAML)
	for name, response := range map[string]undescribedBodyResponse{"can-i-deploy": fromConsumer, "validate": validatedConsumer} {
		this.False(response.Deployable, "consumer %s", name)
		this.Equal(map[string]map[string]map[string][]undescribedBodyLeaf{
			"/files": {"post": {"request": {{Reason: "provider_resource_not_found", Role: "consumer"}}}},
		}, response.Results["files_api"].Endpoints, "consumer %s", name)
	}
}

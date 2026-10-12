package integration_test

import (
	"encoding/json"
	"net/http"
)

type nullableBreakLeaf struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

type nullableBreakResult struct {
	Endpoints map[string]map[string]map[string][]nullableBreakLeaf `json:"endpoints"`
}

type nullableBreakResponse struct {
	Results map[string]nullableBreakResult `json:"results"`
}

const nullableProfilesAPIContract = `provides:
  rest:
    /profiles/*:
      put:
        request: ProfileChanges
        responses:
          200: Profile
schemas:
  ProfileChanges:
    type: object
    properties:
      avatar:
        type: string
  Profile:
    type: object
    properties:
      avatar:
        type: string
        nullable: true
`

const nullableProfilesSPAContract = `consumes:
  profiles_api:
    rest:
      /profiles/*:
        put:
          request: ProfileChanges
          responses:
            200: Profile
schemas:
  ProfileChanges:
    type: object
    properties:
      avatar:
        type: string
        nullable: true
  Profile:
    type: object
    properties:
      avatar:
        type: string
`

func (this *IntegrationSuite) mustPostForNullableBreak(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) canIDeployForNullableBreak(participant, version string) nullableBreakResponse {
	status, body := this.post("/api/can-i-deploy",
		`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s: %s", participant, version, body)

	var parsed nullableBreakResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &parsed))

	return parsed
}

func (this *IntegrationSuite) TestCanIDeploy_NullabilityBreaksCarryTheReasonAndTheRole() {
	this.mustPostForNullableBreak("/api/environments", `{"environment":"production"}`)
	this.mustPostForNullableBreak("/api/participants", `{"participant":"profiles_api"}`)
	this.mustPostForNullableBreak("/api/participants", `{"participant":"profiles_spa"}`)
	this.mustPostForNullableBreak("/api/contracts",
		this.publishBody("profiles_api", "1.0.0", contractFragment{"api.yaml", nullableProfilesAPIContract}))
	this.mustPostForNullableBreak("/api/deployments",
		`{"participant":"profiles_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForNullableBreak("/api/contracts",
		this.publishBody("profiles_spa", "1.0.0", contractFragment{"spa.yaml", nullableProfilesSPAContract}))

	responseDetails := map[string]string{
		"property":      "$.avatar",
		"consumerField": "Profile.avatar",
		"providerField": "Profile.avatar",
		"consumerName":  "profiles_spa",
		"providerName":  "profiles_api",
		"propertyType":  "string",
	}
	requestDetails := map[string]string{
		"property":      "$.avatar",
		"consumerField": "ProfileChanges.avatar",
		"providerField": "ProfileChanges.avatar",
		"consumerName":  "profiles_spa",
		"providerName":  "profiles_api",
		"propertyType":  "string",
	}

	fromTheConsumer := this.canIDeployForNullableBreak("profiles_spa", "1.0.0").
		Results["profiles_api"].Endpoints["/profiles/*"]["put"]
	this.Equal([]nullableBreakLeaf{{
		Reason: "property_nullable_in_provider_not_nullable_in_consumer", Role: "consumer", Details: responseDetails,
	}}, fromTheConsumer["200"])
	this.Equal([]nullableBreakLeaf{{
		Reason: "property_nullable_in_consumer_not_nullable_in_provider", Role: "consumer", Details: requestDetails,
	}}, fromTheConsumer["request"])

	this.mustPostForNullableBreak("/api/deployments",
		`{"participant":"profiles_spa","version":"1.0.0","environment":"production"}`)

	fromTheProvider := this.canIDeployForNullableBreak("profiles_api", "1.0.0").
		Results["profiles_spa"].Endpoints["/profiles/*"]["put"]
	this.Equal([]nullableBreakLeaf{{
		Reason: "property_nullable_in_provider_not_nullable_in_consumer", Role: "provider", Details: responseDetails,
	}}, fromTheProvider["200"])
	this.Equal([]nullableBreakLeaf{{
		Reason: "property_nullable_in_consumer_not_nullable_in_provider", Role: "provider", Details: requestDetails,
	}}, fromTheProvider["request"])
}

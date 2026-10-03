package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

type breakJSON struct {
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Details map[string]string `json:"details"`
}

// endpoints nest as endpoint -> method -> interaction ("request" or a status code) -> breaks
type endpointsJSON map[string]map[string]map[string][]breakJSON

type resultJSON struct {
	Deployable         bool          `json:"deployable"`
	ParticipantVersion *string       `json:"participantVersion"`
	Endpoints          endpointsJSON `json:"endpoints"`
}

type canIDeployResponse struct {
	Message     string                `json:"message"`
	Participant string                `json:"participant"`
	Version     string                `json:"version"`
	Environment string                `json:"environment"`
	Deployable  bool                  `json:"deployable"`
	Results     map[string]resultJSON `json:"results"`
}

type verdictBreakJSON struct {
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Interaction string            `json:"interaction"`
	Reason      string            `json:"reason"`
	Details     map[string]string `json:"details"`
}

// breaksByReason indexes an interaction's breaks by their reason (the scenarios below have
// at most one break per reason within a single interaction).
func breaksByReason(breaks []breakJSON) map[string]breakJSON {
	out := make(map[string]breakJSON, len(breaks))
	for _, b := range breaks {
		out[b.Reason] = b
	}
	return out
}

const apiV1ProviderContract = `
{
  "provides": {
    "rest": {
      "/things": {
        "get": {
          "responses": {
            "200": "Thing"
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

const frontV1ConsumerContract = `
{
  "consumes": {
    "api": {
      "rest": {
        "/things": {
          "get": {
            "responses": {
              "200": "Thing"
            }
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

const frontV2ConsumerContract = `
{
  "consumes": {
    "api": {
      "rest": {
        "/things": {
          "get": {
            "responses": {
              "200": "Thing"
            }
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "integer" },
        "name": { "type": "string" }
      }
    }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_HappyPath() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))
	mustPost("/api/environments", `{"environment":"production"}`)
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", frontV1ConsumerContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var v1Got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &v1Got))
	s.True(v1Got.Deployable)
	s.Equal("front", v1Got.Participant)
	s.Equal("v1", v1Got.Version)
	s.Equal("production", v1Got.Environment)

	s.Require().Len(v1Got.Results, 1)
	v1Api := v1Got.Results["api"]
	s.True(v1Api.Deployable)
	s.Require().NotNil(v1Api.ParticipantVersion)
	s.Equal("v1", *v1Api.ParticipantVersion)
	// compatible counterparts render "endpoints":{}, never null
	s.Contains(body, `"endpoints":{}`)
	s.Require().NotNil(v1Api.Endpoints)
	s.Empty(v1Api.Endpoints)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(1, s.countRows("compatibility_check_results"))
	s.Equal(1, s.countRows("compatibility_verdicts"))

	var v1Deployable, v1ResultDeployable, v1VerdictDeployable bool
	var v1Breaks string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT ch.deployable, r.deployable, v.deployable, v.breaks::text
		   FROM compatibility_checks ch
		   JOIN compatibility_check_results r ON r.check_id = ch.id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE ch.version = 'v1'`).
		Scan(&v1Deployable, &v1ResultDeployable, &v1VerdictDeployable, &v1Breaks))
	s.True(v1Deployable)
	s.True(v1ResultDeployable)
	s.True(v1VerdictDeployable)
	s.Equal("[]", v1Breaks)

	mustPost("/api/deployments", `{"participant":"front","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("front", "v2", contractFragment{"api.yaml", frontV2ConsumerContract}))

	status, body = s.post("/api/can-i-deploy", `{"participant":"front","version":"v2","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("front", got.Participant)
	s.Equal("v2", got.Version)
	s.Equal("production", got.Environment)

	// break leaves are slim {reason, role, details} — no resources on the wire
	s.NotContains(body, "checkedResource")
	s.NotContains(body, "counterpartResource")

	s.Require().Len(got.Results, 1)
	api := got.Results["api"]
	s.False(api.Deployable)
	s.Require().NotNil(api.ParticipantVersion)
	s.Equal("v1", *api.ParticipantVersion)

	s.Require().Len(api.Endpoints, 1)
	s.Require().Len(api.Endpoints["/things"], 1)
	s.Require().Len(api.Endpoints["/things"]["get"], 1)
	breaks := api.Endpoints["/things"]["get"]["200"]
	s.Require().Len(breaks, 2)

	byReason := breaksByReason(breaks)

	typeMismatch, ok := byReason["property_type_mismatch"]
	s.Require().True(ok)
	s.Equal("consumer", typeMismatch.Role)
	s.Equal(map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}, typeMismatch.Details)

	missing, ok := byReason["property_missing_in_provider"]
	s.Require().True(ok)
	s.Equal("consumer", missing.Role)
	s.Equal(map[string]string{"property": "$.name", "consumerName": "front", "providerName": "api", "propertyType": "string"}, missing.Details)

	s.Equal(2, s.countRows("compatibility_checks"))
	s.Equal(2, s.countRows("compatibility_check_results"))
	s.Equal(2, s.countRows("compatibility_verdicts"))

	var v2Deployable, v2VerdictDeployable bool
	var v2Breaks []byte
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT ch.deployable, v.deployable, v.breaks
		   FROM compatibility_checks ch
		   JOIN compatibility_check_results r ON r.check_id = ch.id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE ch.version = 'v2'`).
		Scan(&v2Deployable, &v2VerdictDeployable, &v2Breaks))
	s.False(v2Deployable)
	s.False(v2VerdictDeployable)

	var storedBreaks []verdictBreakJSON
	s.Require().NoError(json.Unmarshal(v2Breaks, &storedBreaks))
	s.Require().Len(storedBreaks, 2)

	storedByReason := map[string]verdictBreakJSON{}
	for _, stored := range storedBreaks {
		s.Equal("/things", stored.Endpoint)
		s.Equal("get", stored.Method)
		s.Equal("200", stored.Interaction)
		storedByReason[stored.Reason] = stored
	}

	s.Equal(typeMismatch.Details, storedByReason["property_type_mismatch"].Details)
	s.Equal(missing.Details, storedByReason["property_missing_in_provider"].Details)
}

const providerCheckedConsumerContract = `
{
  "consumes": {
    "api": {
      "rest": {
        "/things": {
          "get": {
            "responses": {
              "200": "Thing"
            }
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "integer" }
      }
    }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_ProviderCheckedAgainstDeployedConsumer() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", providerCheckedConsumerContract}))
	mustPost("/api/deployments", `{"participant":"front","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("api", got.Participant)
	s.Equal("v1", got.Version)

	s.Require().Len(got.Results, 1)
	front := got.Results["front"]
	s.False(front.Deployable)
	s.Require().NotNil(front.ParticipantVersion)
	s.Equal("v1", *front.ParticipantVersion)

	breaks := front.Endpoints["/things"]["get"]["200"]
	s.Require().Len(breaks, 1)

	b := breaks[0]
	s.Equal("property_type_mismatch", b.Reason)
	s.Equal("provider", b.Role)
	// types resolved by role even though the provider is the checked side
	s.Equal(map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}, b.Details)
}

const appV1ThreeDependenciesContract = `
{
  "consumes": {
    "users":   { "rest": { "/users":   { "get": { "responses": { "200": "User" } } } } },
    "auth":    { "rest": { "/auth":    { "get": { "responses": { "200": "Token" } } } } },
    "catalog": { "rest": { "/catalog": { "get": { "responses": { "200": "Product" } } } } }
  },
  "schemas": {
    "User":    { "type": "object", "properties": { "id":    { "type": "string" } } },
    "Token":   { "type": "object", "properties": { "value": { "type": "string" } } },
    "Product": { "type": "object", "properties": { "id":    { "type": "string" } } }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_RecordsOneRowPerDependency() {
	status, _ := s.post("/api/participants", `{"participant":"app"}`)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/environments", `{"environment":"production"}`)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts",
		s.publishBody("app", "v1", contractFragment{"api.yaml", appV1ThreeDependenciesContract}))
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/can-i-deploy",
		`{"participant":"app","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("app", got.Participant)
	s.Equal("v1", got.Version)

	// a never_published provider has no version to report
	s.Contains(body, `"participantVersion":null`)

	s.Require().Len(got.Results, 3)
	for _, provider := range []string{"users", "auth", "catalog"} {
		result, ok := got.Results[provider]
		s.Require().Truef(ok, "missing result for %s", provider)
		s.False(result.Deployable)
		s.Nil(result.ParticipantVersion)
		breaks := result.Endpoints["/"+provider]["get"]["200"]
		s.Require().Lenf(breaks, 1, "missing break for %s", provider)
		b := breaks[0]
		s.Equal("provider_resource_not_found", b.Reason)
		s.Equal("consumer", b.Role)
		s.Empty(b.Details)
	}

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(3, s.countRows("compatibility_check_results"))
	s.Equal(0, s.countRows("compatibility_verdicts"))

	var checkDeployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT deployable FROM compatibility_checks WHERE version = 'v1'`).Scan(&checkDeployable))
	s.False(checkDeployable)

	rows, err := s.Pool.Query(context.Background(),
		`SELECT r.counterpart_name, r.counterpart_participant_id, r.counterpart_version,
		        r.verdict_contract_id_one, r.verdict_contract_id_two, r.deployable
		   FROM compatibility_check_results r
		   JOIN compatibility_checks ch ON ch.id = r.check_id
		  WHERE ch.version = 'v1'`)
	s.Require().NoError(err)
	defer rows.Close()

	var counterpartNames []string
	for rows.Next() {
		var name string
		var participantID, verdictOne, verdictTwo *int64
		var version *string
		var resultDeployable bool
		s.Require().NoError(rows.Scan(&name, &participantID, &version, &verdictOne, &verdictTwo, &resultDeployable))
		s.Nilf(participantID, "counterpart %s", name)
		s.Nilf(version, "counterpart %s", name)
		s.Nilf(verdictOne, "counterpart %s", name)
		s.Nilf(verdictTwo, "counterpart %s", name)
		s.Falsef(resultDeployable, "counterpart %s", name)
		counterpartNames = append(counterpartNames, name)
	}
	s.Require().NoError(rows.Err())

	s.ElementsMatch([]string{"users", "auth", "catalog"}, counterpartNames)
}

const usersV1ProviderContract = `
{
  "provides": { "rest": { "/users": { "get": { "responses": { "200": "User" } } } } },
  "schemas": { "User": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const authV1ProviderContract = `
{
  "provides": { "rest": { "/auth": { "get": { "responses": { "200": "Token" } } } } },
  "schemas": { "Token": { "type": "object", "properties": { "value": { "type": "string" } } } }
}`

const catalogV1ProviderContract = `
{
  "provides": { "rest": { "/catalog": { "get": { "responses": { "200": "Product" } } } } },
  "schemas": { "Product": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const appV1MixedDependenciesContract = `
{
  "consumes": {
    "users":   { "rest": { "/users":   { "get": { "responses": { "200": "User" } } } } },
    "auth":    { "rest": { "/auth":    { "get": { "responses": { "200": "Token" } } } } },
    "catalog": { "rest": { "/catalog": { "get": { "responses": { "200": "Product" } } } } }
  },
  "schemas": {
    "User":    { "type": "object", "properties": { "id":    { "type": "string" } } },
    "Token":   { "type": "object", "properties": { "value": { "type": "string" } } },
    "Product": { "type": "object", "properties": { "id":    { "type": "integer" } } }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_TwoDeployableOneBreaking() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	for _, name := range []string{"users", "auth", "catalog", "app"} {
		mustPost("/api/participants", `{"participant":"`+name+`"}`)
	}
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("users", "v1", contractFragment{"api.yaml", usersV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"users","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("auth", "v1", contractFragment{"api.yaml", authV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"auth","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("catalog", "v1", contractFragment{"api.yaml", catalogV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"catalog","version":"v1","environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("app", "v1", contractFragment{"api.yaml", appV1MixedDependenciesContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"app","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("app", got.Participant)
	s.Equal("v1", got.Version)

	s.Require().Len(got.Results, 3)
	for _, provider := range []string{"users", "auth"} {
		result := got.Results[provider]
		s.Truef(result.Deployable, "expected %s to be deployable", provider)
		s.Require().NotNil(result.ParticipantVersion)
		s.Equal("v1", *result.ParticipantVersion)
		s.Require().NotNilf(result.Endpoints, "expected %s endpoints to render as {}", provider)
		s.Empty(result.Endpoints)
	}

	catalog := got.Results["catalog"]
	s.False(catalog.Deployable)
	s.Require().NotNil(catalog.ParticipantVersion)
	s.Equal("v1", *catalog.ParticipantVersion)
	breaks := catalog.Endpoints["/catalog"]["get"]["200"]
	s.Require().Len(breaks, 1)
	b := breaks[0]
	s.Equal("property_type_mismatch", b.Reason)
	s.Equal("consumer", b.Role)
	s.Equal(map[string]string{
		"property":             "$.id",
		"consumerName":         "app",
		"providerName":         "catalog",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}, b.Details)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(3, s.countRows("compatibility_check_results"))
	s.Equal(3, s.countRows("compatibility_verdicts"))

	rows, err := s.Pool.Query(context.Background(),
		`SELECT p.name, r.deployable, r.counterpart_version, v.deployable, v.breaks::text
		   FROM compatibility_check_results r
		   JOIN compatibility_checks ch ON ch.id = r.check_id
		   JOIN participants p ON p.id = r.counterpart_participant_id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE ch.version = 'v1'`)
	s.Require().NoError(err)
	defer rows.Close()

	deployableByProvider := map[string]bool{}
	versionByProvider := map[string]string{}
	verdictDeployableByProvider := map[string]bool{}
	verdictBreaksByProvider := map[string]string{}
	for rows.Next() {
		var name string
		var deployable, verdictDeployable bool
		var counterpartVersion, verdictBreaks string
		s.Require().NoError(rows.Scan(&name, &deployable, &counterpartVersion, &verdictDeployable, &verdictBreaks))
		deployableByProvider[name] = deployable
		versionByProvider[name] = counterpartVersion
		verdictDeployableByProvider[name] = verdictDeployable
		verdictBreaksByProvider[name] = verdictBreaks
	}
	s.Require().NoError(rows.Err())

	s.Equal(map[string]bool{"users": true, "auth": true, "catalog": false}, deployableByProvider)
	s.Equal(map[string]string{"users": "v1", "auth": "v1", "catalog": "v1"}, versionByProvider)
	s.Equal(map[string]bool{"users": true, "auth": true, "catalog": false}, verdictDeployableByProvider)
	s.Equal("[]", verdictBreaksByProvider["users"])
	s.Equal("[]", verdictBreaksByProvider["auth"])
}

const providerThingContract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const consumerThingContract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

func (s *IntegrationSuite) TestCanIDeploy_ProviderExistsButNotDeployedInTargetEnv() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)
	mustPost("/api/environments", `{"environment":"staging"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", providerThingContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"staging"}`)

	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", consumerThingContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	s.False(got.Deployable)
	s.Equal("front", got.Participant)
	s.Equal("v1", got.Version)

	s.Require().Len(got.Results, 1)
	api := got.Results["api"]
	s.False(api.Deployable)
	s.Nil(api.ParticipantVersion)
	breaks := api.Endpoints["/things"]["get"]["200"]
	s.Require().Len(breaks, 1)

	b := breaks[0]
	s.Equal("provider_resource_not_deployed_in_environment", b.Reason)
	s.Equal("consumer", b.Role)
	s.Equal(map[string]string{"deployedEnvironments": "staging"}, b.Details)

	s.Equal(1, s.countRows("compatibility_checks"))
	s.Equal(1, s.countRows("compatibility_check_results"))
	s.Equal(0, s.countRows("compatibility_verdicts"))

	// the provider participant is known even though it is not deployed in the target
	// environment: the result keeps its identity with a NULL version and no verdict
	var counterpartName, participantName string
	var counterpartVersion *string
	var verdictOne, verdictTwo *int64
	var resultDeployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT r.counterpart_name, p.name, r.counterpart_version,
		        r.verdict_contract_id_one, r.verdict_contract_id_two, r.deployable
		   FROM compatibility_check_results r
		   JOIN participants p ON p.id = r.counterpart_participant_id`).
		Scan(&counterpartName, &participantName, &counterpartVersion, &verdictOne, &verdictTwo, &resultDeployable))
	s.Equal("api", counterpartName)
	s.Equal("api", participantName)
	s.Nil(counterpartVersion)
	s.Nil(verdictOne)
	s.Nil(verdictTwo)
	s.False(resultDeployable)
}

const dualRoleUsersV1Contract = `
{
  "provides": {
    "rest": {
      "/users": {
        "post": {
          "request": "CreateUserRequest",
          "responses": { "200": "CreateUserResponse" }
        }
      },
      "/users/*": {
        "get": { "responses": { "200": "User" } }
      }
    }
  },
  "schemas": {
    "CreateUserRequest": {
      "type": "object",
      "properties": {
        "email":    { "type": "string" },
        "password": { "type": "string" }
      }
    },
    "CreateUserResponse": {
      "type": "object",
      "properties": {
        "userId": { "type": "integer" }
      }
    },
    "User": {
      "type": "object",
      "properties": {
        "userId": { "type": "integer" },
        "status": { "type": "string" }
      }
    }
  }
}`

const dualRolePetsV1Contract = `
{
  "consumes": {
    "users": {
      "rest": {
        "/users/*": {
          "get": { "responses": { "200": "User" } }
        }
      }
    }
  },
  "provides": {
    "rest": {
      "/pets": {
        "post": {
          "request": "CreatePetRequest",
          "responses": { "200": "Pet" }
        }
      },
      "/pets/*": {
        "get": { "responses": { "200": "Pet" } }
      }
    }
  },
  "schemas": {
    "User": {
      "type": "object",
      "properties": {
        "userId": { "type": "integer" }
      }
    },
    "CreatePetRequest": {
      "type": "object",
      "properties": {
        "name":   { "type": "string" },
        "userId": { "type": "integer" }
      }
    },
    "Pet": {
      "type": "object",
      "properties": {
        "petId":  { "type": "integer" },
        "userId": { "type": "integer" },
        "name":   { "type": "string" }
      }
    }
  }
}`

const dualRolePetsV2Contract = `
{
  "consumes": {
    "users": {
      "rest": {
        "/users/*": {
          "get": { "responses": { "200": "User" } }
        }
      }
    }
  },
  "provides": {
    "rest": {
      "/pets": {
        "post": {
          "request": "CreatePetRequest",
          "responses": { "200": "Pet" }
        }
      },
      "/pets/*": {
        "get": { "responses": { "200": "PetSummary" } }
      }
    }
  },
  "schemas": {
    "User": {
      "type": "object",
      "properties": {
        "userId": { "type": "string" }
      }
    },
    "CreatePetRequest": {
      "type": "object",
      "properties": {
        "name":   { "type": "string" },
        "userId": { "type": "integer" },
        "breed":  { "type": "string" }
      }
    },
    "Pet": {
      "type": "object",
      "properties": {
        "petId":  { "type": "integer" },
        "userId": { "type": "integer" },
        "name":   { "type": "string" }
      }
    },
    "PetSummary": {
      "type": "object",
      "properties": {
        "petId":  { "type": "integer" },
        "userId": { "type": "integer" }
      }
    }
  }
}`

const dualRoleAppV1Contract = `
{
  "consumes": {
    "users": {
      "rest": {
        "/users": {
          "post": {
            "request": "CreateUserRequest",
            "responses": { "200": "CreateUserResponse" }
          }
        },
        "/users/*": {
          "get": { "responses": { "200": "User" } }
        }
      }
    },
    "pets": {
      "rest": {
        "/pets": {
          "post": {
            "request": "CreatePetRequest",
            "responses": { "200": "Pet" }
          }
        },
        "/pets/*": {
          "get": { "responses": { "200": "Pet" } }
        }
      }
    }
  },
  "schemas": {
    "CreateUserRequest": {
      "type": "object",
      "properties": {
        "email":    { "type": "string" },
        "password": { "type": "string" }
      }
    },
    "CreateUserResponse": {
      "type": "object",
      "properties": {
        "userId": { "type": "integer" }
      }
    },
    "User": {
      "type": "object",
      "properties": {
        "userId": { "type": "integer" },
        "status": { "type": "string" }
      }
    },
    "CreatePetRequest": {
      "type": "object",
      "properties": {
        "name":   { "type": "string" },
        "userId": { "type": "integer" }
      }
    },
    "Pet": {
      "type": "object",
      "properties": {
        "petId":  { "type": "integer" },
        "userId": { "type": "integer" },
        "name":   { "type": "string" }
      }
    }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_ConsumerAndProviderSameContract() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	checkDeployableAndDeploy := func(participant, version string) {
		status, body := s.post("/api/can-i-deploy",
			`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
		s.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s", participant, version)
		var deployGot canIDeployResponse
		s.Require().NoErrorf(json.Unmarshal([]byte(body), &deployGot), "can-i-deploy %s %s", participant, version)
		s.Truef(deployGot.Deployable, "can-i-deploy %s %s", participant, version)
		for counterpart, result := range deployGot.Results {
			s.Truef(result.Deployable, "can-i-deploy %s %s vs %s", participant, version, counterpart)
			s.Emptyf(result.Endpoints, "can-i-deploy %s %s vs %s", participant, version, counterpart)
		}
		mustPost("/api/deployments",
			`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	}

	for _, name := range []string{"users", "pets", "app"} {
		mustPost("/api/participants", `{"participant":"`+name+`"}`)
	}
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("users", "v1", contractFragment{"api.yaml", dualRoleUsersV1Contract}))
	checkDeployableAndDeploy("users", "v1")

	mustPost("/api/contracts", s.publishBody("pets", "v1", contractFragment{"api.yaml", dualRolePetsV1Contract}))
	checkDeployableAndDeploy("pets", "v1")

	mustPost("/api/contracts", s.publishBody("app", "v1", contractFragment{"api.yaml", dualRoleAppV1Contract}))
	checkDeployableAndDeploy("app", "v1")

	mustPost("/api/contracts", s.publishBody("pets", "v2", contractFragment{"api.yaml", dualRolePetsV2Contract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"pets","version":"v2","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	// empty versions render as JSON null, never empty strings
	s.NotContains(body, `"participantVersion":""`)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)
	s.Equal("pets", got.Participant)
	s.Equal("v2", got.Version)
	s.Equal("production", got.Environment)

	s.Require().Len(got.Results, 2)

	// pets v2 acts as a provider (checked against the deployed app consumer) and
	// as a consumer of users (checked against the deployed users provider).
	appResult, ok := got.Results["app"]
	s.Require().True(ok)
	s.False(appResult.Deployable)
	s.Require().NotNil(appResult.ParticipantVersion)
	s.Equal("v1", *appResult.ParticipantVersion)
	s.Require().Len(appResult.Endpoints, 2)

	requestBreaks := appResult.Endpoints["/pets"]["post"]["request"]
	s.Require().Len(requestBreaks, 1)
	s.Equal("property_missing_in_consumer", requestBreaks[0].Reason)
	s.Equal("provider", requestBreaks[0].Role)
	s.Equal(map[string]string{"property": "$.breed", "consumerName": "app", "providerName": "pets", "propertyType": "string"}, requestBreaks[0].Details)

	responseBreaks := appResult.Endpoints["/pets/*"]["get"]["200"]
	s.Require().Len(responseBreaks, 1)
	s.Equal("property_missing_in_provider", responseBreaks[0].Reason)
	s.Equal("provider", responseBreaks[0].Role)
	s.Equal(map[string]string{"property": "$.name", "consumerName": "app", "providerName": "pets", "propertyType": "string"}, responseBreaks[0].Details)

	usersResult, ok := got.Results["users"]
	s.Require().True(ok)
	s.False(usersResult.Deployable)
	s.Require().NotNil(usersResult.ParticipantVersion)
	s.Equal("v1", *usersResult.ParticipantVersion)

	consumerBreaks := usersResult.Endpoints["/users/*"]["get"]["200"]
	s.Require().Len(consumerBreaks, 1)
	s.Equal("property_type_mismatch", consumerBreaks[0].Reason)
	s.Equal("consumer", consumerBreaks[0].Role)
	s.Equal(map[string]string{
		"property":             "$.userId",
		"consumerName":         "pets",
		"providerName":         "users",
		"consumerPropertyType": "string",
		"providerPropertyType": "integer",
	}, consumerBreaks[0].Details)

	type checkResultRow struct {
		Counterpart       string
		Version           string
		Deployable        bool
		VerdictDeployable bool
	}

	rows, err := s.Pool.Query(context.Background(),
		`SELECT r.counterpart_name, r.counterpart_version, r.deployable, v.deployable
		   FROM compatibility_check_results r
		   JOIN compatibility_checks ch ON ch.id = r.check_id
		   JOIN participants checked ON checked.id = ch.participant_id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE checked.name = 'pets' AND ch.version = 'v2'`)
	s.Require().NoError(err)
	defer rows.Close()

	var checkResultRows []checkResultRow
	for rows.Next() {
		var row checkResultRow
		s.Require().NoError(rows.Scan(&row.Counterpart, &row.Version, &row.Deployable, &row.VerdictDeployable))
		checkResultRows = append(checkResultRows, row)
	}
	s.Require().NoError(rows.Err())

	s.ElementsMatch([]checkResultRow{
		{Counterpart: "users", Version: "v1", Deployable: false, VerdictDeployable: false},
		{Counterpart: "app", Version: "v1", Deployable: false, VerdictDeployable: false},
	}, checkResultRows)

	var petsV2CheckDeployable bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT ch.deployable
		   FROM compatibility_checks ch
		   JOIN participants checked ON checked.id = ch.participant_id
		  WHERE checked.name = 'pets' AND ch.version = 'v2'`).Scan(&petsV2CheckDeployable))
	s.False(petsV2CheckDeployable)
}

const arrayProviderContract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "string" } } } }
}`

const arrayConsumerContract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id":   { "type": "string" },
        "list": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": { "name": { "type": "string" } }
          }
        },
        "tags": {
          "type": "array",
          "optional": true,
          "items": { "type": "string" }
        }
      }
    }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_MissingArrayReportsEveryNestedProperty() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", arrayProviderContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", arrayConsumerContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))
	s.False(got.Deployable)

	breaks := got.Results["api"].Endpoints["/things"]["get"]["200"]
	s.Require().Len(breaks, 4)

	byProperty := map[string]breakJSON{}
	for _, b := range breaks {
		byProperty[b.Details["property"]] = b
	}

	// a missing required array reports itself and every property inside it
	list, ok := byProperty["$.list"]
	s.Require().True(ok)
	s.Equal("property_missing_in_provider", list.Reason)
	s.Equal("consumer", list.Role)
	s.Equal(map[string]string{"property": "$.list", "consumerName": "front", "providerName": "api", "propertyType": "array<object>"}, list.Details)

	listItems, ok := byProperty["$.list[]"]
	s.Require().True(ok)
	s.Equal("property_missing_in_provider", listItems.Reason)
	s.Equal("consumer", listItems.Role)
	s.Equal(map[string]string{"property": "$.list[]", "consumerName": "front", "providerName": "api", "propertyType": "object"}, listItems.Details)

	listItemName, ok := byProperty["$.list[].name"]
	s.Require().True(ok)
	s.Equal("property_missing_in_provider", listItemName.Reason)
	s.Equal("consumer", listItemName.Role)
	s.Equal(map[string]string{"property": "$.list[].name", "consumerName": "front", "providerName": "api", "propertyType": "string"}, listItemName.Details)

	// an optional missing array emits no break itself, but its required items still report
	items, ok := byProperty["$.tags[]"]
	s.Require().True(ok)
	s.Equal("property_missing_in_provider", items.Reason)
	s.Equal("consumer", items.Role)
	s.Equal(map[string]string{"property": "$.tags[]", "consumerName": "front", "providerName": "api", "propertyType": "string"}, items.Details)
}

func (s *IntegrationSuite) TestCanIDeploy_ProviderExistsButDeployedNowhere() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", providerThingContract}))
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", consumerThingContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	s.False(got.Deployable)
	s.Equal("front", got.Participant)
	s.Equal("v1", got.Version)

	s.Require().Len(got.Results, 1)
	api := got.Results["api"]
	s.False(api.Deployable)
	s.Nil(api.ParticipantVersion)
	breaks := api.Endpoints["/things"]["get"]["200"]
	s.Require().Len(breaks, 1)

	b := breaks[0]
	s.Equal("provider_resource_not_deployed_in_environment", b.Reason)
	s.Equal("consumer", b.Role)
	s.Empty(b.Details)
}

const apiV2IncompatibleProviderContract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "integer" } } } }
}`

func (s *IntegrationSuite) TestCanIDeploy_ChecksProviderAtItsDeployedVersion() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)
	// published but never deployed — must not influence the verdict
	mustPost("/api/contracts", s.publishBody("api", "v2", contractFragment{"api.yaml", apiV2IncompatibleProviderContract}))
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", frontV1ConsumerContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	s.True(got.Deployable)
	s.Require().Len(got.Results, 1)
	api := got.Results["api"]
	s.True(api.Deployable)
	s.Empty(api.Endpoints)
	// the version compared against is the one reported
	s.Require().NotNil(api.ParticipantVersion)
	s.Equal("v1", *api.ParticipantVersion)
}

func (s *IntegrationSuite) TestCanIDeploy_ChecksConsumerAtItsDeployedVersion() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", frontV1ConsumerContract}))
	mustPost("/api/deployments", `{"participant":"front","version":"v1","environment":"production"}`)
	// published but never deployed — must not influence the verdict
	mustPost("/api/contracts", s.publishBody("front", "v2", contractFragment{"api.yaml", frontV2ConsumerContract}))
	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	s.True(got.Deployable)
	s.Require().Len(got.Results, 1)
	front := got.Results["front"]
	s.True(front.Deployable)
	s.Empty(front.Endpoints)
	s.Require().NotNil(front.ParticipantVersion)
	s.Equal("v1", *front.ParticipantVersion)
}

const removedRequestProviderV1Contract = `
{
  "provides": { "rest": { "/orders": { "post": { "request": "Order" } } } },
  "schemas": {
    "Order": {
      "type": "object",
      "properties": {
        "id":     { "type": "string" },
        "coupon": { "type": "string" }
      }
    }
  }
}`

const removedRequestProviderV2Contract = `
{
  "provides": { "rest": { "/orders": { "post": { "request": "Order" } } } },
  "schemas": {
    "Order": { "type": "object", "properties": { "id": { "type": "string" } } }
  }
}`

const removedRequestConsumerContract = `
{
  "consumes": { "api": { "rest": { "/orders": { "post": { "request": "Order" } } } } },
  "schemas": {
    "Order": { "type": "object", "properties": { "id": { "type": "string" } } }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_RemovedProviderPropertyIsNotChecked() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", removedRequestProviderV1Contract}))
	mustPost("/api/contracts", s.publishBody("api", "v2", contractFragment{"api.yaml", removedRequestProviderV2Contract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v2","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", removedRequestConsumerContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	// $.coupon was removed in api v2, so it is no longer part of the provider resource
	s.True(got.Deployable)
	s.Require().Len(got.Results, 1)
	api := got.Results["api"]
	s.True(api.Deployable)
	s.Empty(api.Endpoints)
	s.NotContains(body, "$.coupon")
}

const removedResponseConsumerV1Contract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id":     { "type": "string" },
        "legacy": { "type": "string" }
      }
    }
  }
}`

const removedResponseConsumerV2Contract = `
{
  "consumes": { "api": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } } },
  "schemas": {
    "Thing": { "type": "object", "properties": { "id": { "type": "string" } } }
  }
}`

func (s *IntegrationSuite) TestCanIDeploy_RemovedConsumerPropertyIsNotChecked() {
	mustPost := func(path, body string) {
		status, _ := s.post(path, body)
		s.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", s.publishBody("front", "v1", contractFragment{"api.yaml", removedResponseConsumerV1Contract}))
	mustPost("/api/contracts", s.publishBody("front", "v2", contractFragment{"api.yaml", removedResponseConsumerV2Contract}))
	mustPost("/api/deployments", `{"participant":"front","version":"v2","environment":"production"}`)
	mustPost("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))

	status, body := s.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	s.Equal(http.StatusOK, status)

	var got canIDeployResponse
	s.Require().NoError(json.Unmarshal([]byte(body), &got))

	// $.legacy was removed in front v2, so the deployed consumer no longer requires it
	s.True(got.Deployable)
	s.Require().Len(got.Results, 1)
	front := got.Results["front"]
	s.True(front.Deployable)
	s.Empty(front.Endpoints)
	s.NotContains(body, "$.legacy")
}

func (s *IntegrationSuite) TestCanIDeploy_UnknownEnvironmentReturns404() {
	status, _ := s.post("/api/participants", `{"participant":"api"}`)
	s.Require().Equal(http.StatusOK, status)

	status, _ = s.post("/api/contracts", s.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))
	s.Require().Equal(http.StatusOK, status)

	status, body := s.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	s.Equal(http.StatusNotFound, status)
	s.JSONEq(`{"message":"environment not found"}`, body)

	s.Equal(0, s.countRows("compatibility_checks"))
}

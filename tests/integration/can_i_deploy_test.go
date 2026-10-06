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

func (this *IntegrationSuite) TestCanIDeploy_HappyPath() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))
	mustPost("/api/environments", `{"environment":"production"}`)
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", frontV1ConsumerContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var v1Got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &v1Got))
	this.True(v1Got.Deployable)
	this.Equal("front", v1Got.Participant)
	this.Equal("v1", v1Got.Version)
	this.Equal("production", v1Got.Environment)

	this.Require().Len(v1Got.Results, 1)
	v1Api := v1Got.Results["api"]
	this.True(v1Api.Deployable)
	this.Require().NotNil(v1Api.ParticipantVersion)
	this.Equal("v1", *v1Api.ParticipantVersion)
	// compatible counterparts render "endpoints":{}, never null
	this.Contains(body, `"endpoints":{}`)
	this.Require().NotNil(v1Api.Endpoints)
	this.Empty(v1Api.Endpoints)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_check_results"))
	this.Equal(1, this.countRows("compatibility_verdicts"))

	var v1Deployable, v1ResultDeployable, v1VerdictDeployable bool
	var v1Breaks string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT ch.deployable, r.deployable, v.deployable, v.breaks::text
		   FROM compatibility_checks ch
		   JOIN compatibility_check_results r ON r.check_id = ch.id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE ch.version = 'v1'`).
		Scan(&v1Deployable, &v1ResultDeployable, &v1VerdictDeployable, &v1Breaks))
	this.True(v1Deployable)
	this.True(v1ResultDeployable)
	this.True(v1VerdictDeployable)
	this.Equal("[]", v1Breaks)

	mustPost("/api/deployments", `{"participant":"front","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("front", "v2", contractFragment{"api.yaml", frontV2ConsumerContract}))

	status, body = this.post("/api/can-i-deploy", `{"participant":"front","version":"v2","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("front", got.Participant)
	this.Equal("v2", got.Version)
	this.Equal("production", got.Environment)

	// break leaves are slim {reason, role, details} — no resources on the wire
	this.NotContains(body, "checkedResource")
	this.NotContains(body, "counterpartResource")

	this.Require().Len(got.Results, 1)
	api := got.Results["api"]
	this.False(api.Deployable)
	this.Require().NotNil(api.ParticipantVersion)
	this.Equal("v1", *api.ParticipantVersion)

	this.Require().Len(api.Endpoints, 1)
	this.Require().Len(api.Endpoints["/things"], 1)
	this.Require().Len(api.Endpoints["/things"]["get"], 1)
	breaks := api.Endpoints["/things"]["get"]["200"]
	this.Require().Len(breaks, 2)

	byReason := breaksByReason(breaks)

	typeMismatch, ok := byReason["property_type_mismatch"]
	this.Require().True(ok)
	this.Equal("consumer", typeMismatch.Role)
	this.Equal(map[string]string{
		"property":             "$.id",
		"consumerName":         "front",
		"providerName":         "api",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}, typeMismatch.Details)

	missing, ok := byReason["property_missing_in_provider"]
	this.Require().True(ok)
	this.Equal("consumer", missing.Role)
	this.Equal(map[string]string{"property": "$.name", "consumerName": "front", "providerName": "api", "propertyType": "string"}, missing.Details)

	this.Equal(2, this.countRows("compatibility_checks"))
	this.Equal(2, this.countRows("compatibility_check_results"))
	this.Equal(2, this.countRows("compatibility_verdicts"))

	var v2Deployable, v2VerdictDeployable bool
	var v2Breaks []byte
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT ch.deployable, v.deployable, v.breaks
		   FROM compatibility_checks ch
		   JOIN compatibility_check_results r ON r.check_id = ch.id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE ch.version = 'v2'`).
		Scan(&v2Deployable, &v2VerdictDeployable, &v2Breaks))
	this.False(v2Deployable)
	this.False(v2VerdictDeployable)

	var storedBreaks []verdictBreakJSON
	this.Require().NoError(json.Unmarshal(v2Breaks, &storedBreaks))
	this.Require().Len(storedBreaks, 2)

	storedByReason := map[string]verdictBreakJSON{}
	for _, stored := range storedBreaks {
		this.Equal("/things", stored.Endpoint)
		this.Equal("get", stored.Method)
		this.Equal("200", stored.Interaction)
		storedByReason[stored.Reason] = stored
	}

	this.Equal(typeMismatch.Details, storedByReason["property_type_mismatch"].Details)
	this.Equal(missing.Details, storedByReason["property_missing_in_provider"].Details)
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

func (this *IntegrationSuite) TestCanIDeploy_ProviderCheckedAgainstDeployedConsumer() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", providerCheckedConsumerContract}))
	mustPost("/api/deployments", `{"participant":"front","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("api", got.Participant)
	this.Equal("v1", got.Version)

	this.Require().Len(got.Results, 1)
	front := got.Results["front"]
	this.False(front.Deployable)
	this.Require().NotNil(front.ParticipantVersion)
	this.Equal("v1", *front.ParticipantVersion)

	breaks := front.Endpoints["/things"]["get"]["200"]
	this.Require().Len(breaks, 1)

	b := breaks[0]
	this.Equal("property_type_mismatch", b.Reason)
	this.Equal("provider", b.Role)
	// types resolved by role even though the provider is the checked side
	this.Equal(map[string]string{
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

func (this *IntegrationSuite) TestCanIDeploy_RecordsOneRowPerDependency() {
	status, _ := this.post("/api/participants", `{"participant":"app"}`)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/environments", `{"environment":"production"}`)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts",
		this.publishBody("app", "v1", contractFragment{"api.yaml", appV1ThreeDependenciesContract}))
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/can-i-deploy",
		`{"participant":"app","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("app", got.Participant)
	this.Equal("v1", got.Version)

	// a never_published provider has no version to report
	this.Contains(body, `"participantVersion":null`)

	this.Require().Len(got.Results, 3)
	for _, provider := range []string{"users", "auth", "catalog"} {
		result, ok := got.Results[provider]
		this.Require().Truef(ok, "missing result for %s", provider)
		this.False(result.Deployable)
		this.Nil(result.ParticipantVersion)
		breaks := result.Endpoints["/"+provider]["get"]["200"]
		this.Require().Lenf(breaks, 1, "missing break for %s", provider)
		b := breaks[0]
		this.Equal("provider_resource_not_found", b.Reason)
		this.Equal("consumer", b.Role)
		this.Empty(b.Details)
	}

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(3, this.countRows("compatibility_check_results"))
	this.Equal(0, this.countRows("compatibility_verdicts"))

	var checkDeployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT deployable FROM compatibility_checks WHERE version = 'v1'`).Scan(&checkDeployable))
	this.False(checkDeployable)

	rows, err := this.Pool.Query(context.Background(),
		`SELECT r.counterpart_name, r.counterpart_participant_id, r.counterpart_version,
		        r.verdict_contract_id_one, r.verdict_contract_id_two, r.deployable
		   FROM compatibility_check_results r
		   JOIN compatibility_checks ch ON ch.id = r.check_id
		  WHERE ch.version = 'v1'`)
	this.Require().NoError(err)
	defer rows.Close()

	var counterpartNames []string
	for rows.Next() {
		var name string
		var participantID, verdictOne, verdictTwo *int64
		var version *string
		var resultDeployable bool
		this.Require().NoError(rows.Scan(&name, &participantID, &version, &verdictOne, &verdictTwo, &resultDeployable))
		this.Nilf(participantID, "counterpart %s", name)
		this.Nilf(version, "counterpart %s", name)
		this.Nilf(verdictOne, "counterpart %s", name)
		this.Nilf(verdictTwo, "counterpart %s", name)
		this.Falsef(resultDeployable, "counterpart %s", name)
		counterpartNames = append(counterpartNames, name)
	}
	this.Require().NoError(rows.Err())

	this.ElementsMatch([]string{"users", "auth", "catalog"}, counterpartNames)
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

func (this *IntegrationSuite) TestCanIDeploy_TwoDeployableOneBreaking() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	for _, name := range []string{"users", "auth", "catalog", "app"} {
		mustPost("/api/participants", `{"participant":"`+name+`"}`)
	}
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("users", "v1", contractFragment{"api.yaml", usersV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"users","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("auth", "v1", contractFragment{"api.yaml", authV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"auth","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("catalog", "v1", contractFragment{"api.yaml", catalogV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"catalog","version":"v1","environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("app", "v1", contractFragment{"api.yaml", appV1MixedDependenciesContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"app","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("app", got.Participant)
	this.Equal("v1", got.Version)

	this.Require().Len(got.Results, 3)
	for _, provider := range []string{"users", "auth"} {
		result := got.Results[provider]
		this.Truef(result.Deployable, "expected %s to be deployable", provider)
		this.Require().NotNil(result.ParticipantVersion)
		this.Equal("v1", *result.ParticipantVersion)
		this.Require().NotNilf(result.Endpoints, "expected %s endpoints to render as {}", provider)
		this.Empty(result.Endpoints)
	}

	catalog := got.Results["catalog"]
	this.False(catalog.Deployable)
	this.Require().NotNil(catalog.ParticipantVersion)
	this.Equal("v1", *catalog.ParticipantVersion)
	breaks := catalog.Endpoints["/catalog"]["get"]["200"]
	this.Require().Len(breaks, 1)
	b := breaks[0]
	this.Equal("property_type_mismatch", b.Reason)
	this.Equal("consumer", b.Role)
	this.Equal(map[string]string{
		"property":             "$.id",
		"consumerName":         "app",
		"providerName":         "catalog",
		"consumerPropertyType": "integer",
		"providerPropertyType": "string",
	}, b.Details)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(3, this.countRows("compatibility_check_results"))
	this.Equal(3, this.countRows("compatibility_verdicts"))

	rows, err := this.Pool.Query(context.Background(),
		`SELECT p.name, r.deployable, r.counterpart_version, v.deployable, v.breaks::text
		   FROM compatibility_check_results r
		   JOIN compatibility_checks ch ON ch.id = r.check_id
		   JOIN participants p ON p.id = r.counterpart_participant_id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE ch.version = 'v1'`)
	this.Require().NoError(err)
	defer rows.Close()

	deployableByProvider := map[string]bool{}
	versionByProvider := map[string]string{}
	verdictDeployableByProvider := map[string]bool{}
	verdictBreaksByProvider := map[string]string{}
	for rows.Next() {
		var name string
		var deployable, verdictDeployable bool
		var counterpartVersion, verdictBreaks string
		this.Require().NoError(rows.Scan(&name, &deployable, &counterpartVersion, &verdictDeployable, &verdictBreaks))
		deployableByProvider[name] = deployable
		versionByProvider[name] = counterpartVersion
		verdictDeployableByProvider[name] = verdictDeployable
		verdictBreaksByProvider[name] = verdictBreaks
	}
	this.Require().NoError(rows.Err())

	this.Equal(map[string]bool{"users": true, "auth": true, "catalog": false}, deployableByProvider)
	this.Equal(map[string]string{"users": "v1", "auth": "v1", "catalog": "v1"}, versionByProvider)
	this.Equal(map[string]bool{"users": true, "auth": true, "catalog": false}, verdictDeployableByProvider)
	this.Equal("[]", verdictBreaksByProvider["users"])
	this.Equal("[]", verdictBreaksByProvider["auth"])
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

func (this *IntegrationSuite) TestCanIDeploy_ProviderExistsButNotDeployedInTargetEnv() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)
	mustPost("/api/environments", `{"environment":"staging"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", providerThingContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"staging"}`)

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", consumerThingContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.False(got.Deployable)
	this.Equal("front", got.Participant)
	this.Equal("v1", got.Version)

	this.Require().Len(got.Results, 1)
	api := got.Results["api"]
	this.False(api.Deployable)
	this.Nil(api.ParticipantVersion)
	breaks := api.Endpoints["/things"]["get"]["200"]
	this.Require().Len(breaks, 1)

	b := breaks[0]
	this.Equal("provider_resource_not_deployed_in_environment", b.Reason)
	this.Equal("consumer", b.Role)
	this.Equal(map[string]string{"deployedEnvironments": "staging"}, b.Details)

	this.Equal(1, this.countRows("compatibility_checks"))
	this.Equal(1, this.countRows("compatibility_check_results"))
	this.Equal(0, this.countRows("compatibility_verdicts"))

	// the provider participant is known even though it is not deployed in the target
	// environment: the result keeps its identity with a NULL version and no verdict
	var counterpartName, participantName string
	var counterpartVersion *string
	var verdictOne, verdictTwo *int64
	var resultDeployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT r.counterpart_name, p.name, r.counterpart_version,
		        r.verdict_contract_id_one, r.verdict_contract_id_two, r.deployable
		   FROM compatibility_check_results r
		   JOIN participants p ON p.id = r.counterpart_participant_id`).
		Scan(&counterpartName, &participantName, &counterpartVersion, &verdictOne, &verdictTwo, &resultDeployable))
	this.Equal("api", counterpartName)
	this.Equal("api", participantName)
	this.Nil(counterpartVersion)
	this.Nil(verdictOne)
	this.Nil(verdictTwo)
	this.False(resultDeployable)
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

func (this *IntegrationSuite) TestCanIDeploy_ConsumerAndProviderSameContract() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	checkDeployableAndDeploy := func(participant, version string) {
		status, body := this.post("/api/can-i-deploy",
			`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
		this.Require().Equalf(http.StatusOK, status, "can-i-deploy %s %s", participant, version)
		var deployGot canIDeployResponse
		this.Require().NoErrorf(json.Unmarshal([]byte(body), &deployGot), "can-i-deploy %s %s", participant, version)
		this.Truef(deployGot.Deployable, "can-i-deploy %s %s", participant, version)
		for counterpart, result := range deployGot.Results {
			this.Truef(result.Deployable, "can-i-deploy %s %s vs %s", participant, version, counterpart)
			this.Emptyf(result.Endpoints, "can-i-deploy %s %s vs %s", participant, version, counterpart)
		}
		mustPost("/api/deployments",
			`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	}

	for _, name := range []string{"users", "pets", "app"} {
		mustPost("/api/participants", `{"participant":"`+name+`"}`)
	}
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("users", "v1", contractFragment{"api.yaml", dualRoleUsersV1Contract}))
	checkDeployableAndDeploy("users", "v1")

	mustPost("/api/contracts", this.publishBody("pets", "v1", contractFragment{"api.yaml", dualRolePetsV1Contract}))
	checkDeployableAndDeploy("pets", "v1")

	mustPost("/api/contracts", this.publishBody("app", "v1", contractFragment{"api.yaml", dualRoleAppV1Contract}))
	checkDeployableAndDeploy("app", "v1")

	mustPost("/api/contracts", this.publishBody("pets", "v2", contractFragment{"api.yaml", dualRolePetsV2Contract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"pets","version":"v2","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	// empty versions render as JSON null, never empty strings
	this.NotContains(body, `"participantVersion":""`)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)
	this.Equal("pets", got.Participant)
	this.Equal("v2", got.Version)
	this.Equal("production", got.Environment)

	this.Require().Len(got.Results, 2)

	// pets v2 acts as a provider (checked against the deployed app consumer) and
	// as a consumer of users (checked against the deployed users provider).
	appResult, ok := got.Results["app"]
	this.Require().True(ok)
	this.False(appResult.Deployable)
	this.Require().NotNil(appResult.ParticipantVersion)
	this.Equal("v1", *appResult.ParticipantVersion)
	this.Require().Len(appResult.Endpoints, 2)

	requestBreaks := appResult.Endpoints["/pets"]["post"]["request"]
	this.Require().Len(requestBreaks, 1)
	this.Equal("property_missing_in_consumer", requestBreaks[0].Reason)
	this.Equal("provider", requestBreaks[0].Role)
	this.Equal(map[string]string{"property": "$.breed", "consumerName": "app", "providerName": "pets", "propertyType": "string"}, requestBreaks[0].Details)

	responseBreaks := appResult.Endpoints["/pets/*"]["get"]["200"]
	this.Require().Len(responseBreaks, 1)
	this.Equal("property_missing_in_provider", responseBreaks[0].Reason)
	this.Equal("provider", responseBreaks[0].Role)
	this.Equal(map[string]string{"property": "$.name", "consumerName": "app", "providerName": "pets", "propertyType": "string"}, responseBreaks[0].Details)

	usersResult, ok := got.Results["users"]
	this.Require().True(ok)
	this.False(usersResult.Deployable)
	this.Require().NotNil(usersResult.ParticipantVersion)
	this.Equal("v1", *usersResult.ParticipantVersion)

	consumerBreaks := usersResult.Endpoints["/users/*"]["get"]["200"]
	this.Require().Len(consumerBreaks, 1)
	this.Equal("property_type_mismatch", consumerBreaks[0].Reason)
	this.Equal("consumer", consumerBreaks[0].Role)
	this.Equal(map[string]string{
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

	rows, err := this.Pool.Query(context.Background(),
		`SELECT r.counterpart_name, r.counterpart_version, r.deployable, v.deployable
		   FROM compatibility_check_results r
		   JOIN compatibility_checks ch ON ch.id = r.check_id
		   JOIN participants checked ON checked.id = ch.participant_id
		   JOIN compatibility_verdicts v
		     ON v.contract_id_one = r.verdict_contract_id_one
		    AND v.contract_id_two = r.verdict_contract_id_two
		  WHERE checked.name = 'pets' AND ch.version = 'v2'`)
	this.Require().NoError(err)
	defer rows.Close()

	var checkResultRows []checkResultRow
	for rows.Next() {
		var row checkResultRow
		this.Require().NoError(rows.Scan(&row.Counterpart, &row.Version, &row.Deployable, &row.VerdictDeployable))
		checkResultRows = append(checkResultRows, row)
	}
	this.Require().NoError(rows.Err())

	this.ElementsMatch([]checkResultRow{
		{Counterpart: "users", Version: "v1", Deployable: false, VerdictDeployable: false},
		{Counterpart: "app", Version: "v1", Deployable: false, VerdictDeployable: false},
	}, checkResultRows)

	var petsV2CheckDeployable bool
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT ch.deployable
		   FROM compatibility_checks ch
		   JOIN participants checked ON checked.id = ch.participant_id
		  WHERE checked.name = 'pets' AND ch.version = 'v2'`).Scan(&petsV2CheckDeployable))
	this.False(petsV2CheckDeployable)
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

func (this *IntegrationSuite) TestCanIDeploy_MissingArrayReportsEveryNestedProperty() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", arrayProviderContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", arrayConsumerContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))
	this.False(got.Deployable)

	breaks := got.Results["api"].Endpoints["/things"]["get"]["200"]
	this.Require().Len(breaks, 4)

	byProperty := map[string]breakJSON{}
	for _, b := range breaks {
		byProperty[b.Details["property"]] = b
	}

	// a missing required array reports itself and every property inside it
	list, ok := byProperty["$.list"]
	this.Require().True(ok)
	this.Equal("property_missing_in_provider", list.Reason)
	this.Equal("consumer", list.Role)
	this.Equal(map[string]string{"property": "$.list", "consumerName": "front", "providerName": "api", "propertyType": "array<object>"}, list.Details)

	listItems, ok := byProperty["$.list[]"]
	this.Require().True(ok)
	this.Equal("property_missing_in_provider", listItems.Reason)
	this.Equal("consumer", listItems.Role)
	this.Equal(map[string]string{"property": "$.list[]", "consumerName": "front", "providerName": "api", "propertyType": "object"}, listItems.Details)

	listItemName, ok := byProperty["$.list[].name"]
	this.Require().True(ok)
	this.Equal("property_missing_in_provider", listItemName.Reason)
	this.Equal("consumer", listItemName.Role)
	this.Equal(map[string]string{"property": "$.list[].name", "consumerName": "front", "providerName": "api", "propertyType": "string"}, listItemName.Details)

	// an optional missing array emits no break itself, but its required items still report
	items, ok := byProperty["$.tags[]"]
	this.Require().True(ok)
	this.Equal("property_missing_in_provider", items.Reason)
	this.Equal("consumer", items.Role)
	this.Equal(map[string]string{"property": "$.tags[]", "consumerName": "front", "providerName": "api", "propertyType": "string"}, items.Details)
}

func (this *IntegrationSuite) TestCanIDeploy_ProviderExistsButDeployedNowhere() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", providerThingContract}))
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", consumerThingContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.False(got.Deployable)
	this.Equal("front", got.Participant)
	this.Equal("v1", got.Version)

	this.Require().Len(got.Results, 1)
	api := got.Results["api"]
	this.False(api.Deployable)
	this.Nil(api.ParticipantVersion)
	breaks := api.Endpoints["/things"]["get"]["200"]
	this.Require().Len(breaks, 1)

	b := breaks[0]
	this.Equal("provider_resource_not_deployed_in_environment", b.Reason)
	this.Equal("consumer", b.Role)
	this.Empty(b.Details)
}

const apiV2IncompatibleProviderContract = `
{
  "provides": { "rest": { "/things": { "get": { "responses": { "200": "Thing" } } } } },
  "schemas": { "Thing": { "type": "object", "properties": { "id": { "type": "integer" } } } }
}`

func (this *IntegrationSuite) TestCanIDeploy_ChecksProviderAtItsDeployedVersion() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v1","environment":"production"}`)
	// published but never deployed — must not influence the verdict
	mustPost("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", apiV2IncompatibleProviderContract}))
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", frontV1ConsumerContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.True(got.Deployable)
	this.Require().Len(got.Results, 1)
	api := got.Results["api"]
	this.True(api.Deployable)
	this.Empty(api.Endpoints)
	// the version compared against is the one reported
	this.Require().NotNil(api.ParticipantVersion)
	this.Equal("v1", *api.ParticipantVersion)
}

func (this *IntegrationSuite) TestCanIDeploy_ChecksConsumerAtItsDeployedVersion() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", frontV1ConsumerContract}))
	mustPost("/api/deployments", `{"participant":"front","version":"v1","environment":"production"}`)
	// published but never deployed — must not influence the verdict
	mustPost("/api/contracts", this.publishBody("front", "v2", contractFragment{"api.yaml", frontV2ConsumerContract}))
	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	this.True(got.Deployable)
	this.Require().Len(got.Results, 1)
	front := got.Results["front"]
	this.True(front.Deployable)
	this.Empty(front.Endpoints)
	this.Require().NotNil(front.ParticipantVersion)
	this.Equal("v1", *front.ParticipantVersion)
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

func (this *IntegrationSuite) TestCanIDeploy_RemovedProviderPropertyIsNotChecked() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", removedRequestProviderV1Contract}))
	mustPost("/api/contracts", this.publishBody("api", "v2", contractFragment{"api.yaml", removedRequestProviderV2Contract}))
	mustPost("/api/deployments", `{"participant":"api","version":"v2","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", removedRequestConsumerContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"front","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	// $.coupon was removed in api v2, so it is no longer part of the provider resource
	this.True(got.Deployable)
	this.Require().Len(got.Results, 1)
	api := got.Results["api"]
	this.True(api.Deployable)
	this.Empty(api.Endpoints)
	this.NotContains(body, "$.coupon")
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

func (this *IntegrationSuite) TestCanIDeploy_RemovedConsumerPropertyIsNotChecked() {
	mustPost := func(path, body string) {
		status, _ := this.post(path, body)
		this.Require().Equalf(http.StatusOK, status, "POST %s", path)
	}

	mustPost("/api/participants", `{"participant":"api"}`)
	mustPost("/api/participants", `{"participant":"front"}`)
	mustPost("/api/environments", `{"environment":"production"}`)

	mustPost("/api/contracts", this.publishBody("front", "v1", contractFragment{"api.yaml", removedResponseConsumerV1Contract}))
	mustPost("/api/contracts", this.publishBody("front", "v2", contractFragment{"api.yaml", removedResponseConsumerV2Contract}))
	mustPost("/api/deployments", `{"participant":"front","version":"v2","environment":"production"}`)
	mustPost("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	this.Equal(http.StatusOK, status)

	var got canIDeployResponse
	this.Require().NoError(json.Unmarshal([]byte(body), &got))

	// $.legacy was removed in front v2, so the deployed consumer no longer requires it
	this.True(got.Deployable)
	this.Require().Len(got.Results, 1)
	front := got.Results["front"]
	this.True(front.Deployable)
	this.Empty(front.Endpoints)
	this.NotContains(body, "$.legacy")
}

func (this *IntegrationSuite) TestCanIDeploy_UnknownEnvironmentReturns404() {
	status, _ := this.post("/api/participants", `{"participant":"api"}`)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("api", "v1", contractFragment{"api.yaml", apiV1ProviderContract}))
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/can-i-deploy", `{"participant":"api","version":"v1","environment":"production"}`)
	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"environment not found"}`, body)

	this.Equal(0, this.countRows("compatibility_checks"))
}

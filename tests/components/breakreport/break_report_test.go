package breakreport_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/breakreport"
	restbreakreport "github.com/bidirekt/broker/internal/components/breakreport/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	checkedParticipant = "petstore_web"
	checkEnvironment   = "production"
)

const consumerSideReport = `{
  "petstore_api": {
    "deployable": false,
    "participantVersion": "1.4.0",
    "endpoints": {
      "/pets": {"get": {"200": [
        {"reason": "property_type_mismatch", "role": "consumer", "details": {"property": "$[].weight", "consumerField": "Pet.weight", "providerField": "Animal.weight", "consumerPropertyType": "string", "providerPropertyType": "float"}}
      ]}},
      "/pets/*": {"get": {"200": [
        {"reason": "property_type_mismatch", "role": "consumer", "details": {"property": "$.weight", "consumerField": "Pet.weight", "consumerPropertyType": "string", "providerPropertyType": "float"}}
      ]}},
      "/pets/*/adopt": {"post": {"200": [
        {"reason": "provider_resource_interaction_not_found", "role": "consumer", "details": {"providerStatuses": "204, 404"}}
      ]}}
    }
  },
  "petstore_reviews": {
    "deployable": false,
    "participantVersion": null,
    "endpoints": {
      "/reviews": {"get": {"200": [
        {"reason": "provider_resource_not_deployed_in_environment", "role": "consumer"}
      ]}}
    }
  },
  "petstore_users": {"deployable": true, "participantVersion": "3.0.0", "endpoints": {}}
}`

const providerSideReport = `{
  "petstore_web": {
    "deployable": false,
    "participantVersion": "2.2.0",
    "endpoints": {
      "/pets/*/adopt": {"post": {
        "200": [{"reason": "provider_resource_removed_but_still_consumed", "role": "provider"}],
        "404": [{"reason": "provider_resource_removed_but_still_consumed", "role": "provider"}]
      }},
      "/pets": {"get": {
        "200": [{"reason": "provider_resource_interaction_removed_but_still_consumed", "role": "provider", "details": {"providerStatuses": "204"}}],
        "404": [{"reason": "provider_resource_interaction_removed_but_still_consumed", "role": "provider", "details": {"providerStatuses": "204"}}]
      }}
    }
  }
}`

func consumerSideCheck() (model.CompatibilityCheck, []model.CompatibilityCheckResult) {
	check := model.CompatibilityCheck{
		ParticipantName: checkedParticipant,
		EnvironmentName: checkEnvironment,
		Report:          []byte(consumerSideReport),
	}
	results := []model.CompatibilityCheckResult{
		{CounterpartName: "petstore_users", Role: model.CounterpartProvider, CounterpartVersion: null.StringFrom("3.0.0"), CounterpartParticipantID: null.IntFrom(1), Deployable: true},
		{CounterpartName: "petstore_reviews", Role: model.CounterpartProvider, CounterpartParticipantID: null.IntFrom(1)},
		{CounterpartName: "petstore_api", Role: model.CounterpartBoth, CounterpartVersion: null.StringFrom("1.4.0"), CounterpartParticipantID: null.IntFrom(1)},
	}
	return check, results
}

func fromStoredCheck(check model.CompatibilityCheck, results []model.CompatibilityCheckResult) breakreport.Report {
	return breakreport.NewReader([]breakreport.ProtocolSection{restbreakreport.New()}).FromStoredCheck(check, results)
}

func counterpartNamed(t *testing.T, report breakreport.Report, name string) breakreport.Counterpart {
	for _, counterpart := range report.Counterparts {
		if counterpart.Name == name {
			return counterpart
		}
	}
	require.FailNow(t, "counterpart not in the report", name)
	return breakreport.Counterpart{}
}

func restResources(t *testing.T, counterpart breakreport.Counterpart) restbreakreport.Resources {
	resources, ok := counterpart.Protocols["rest"].(restbreakreport.Resources)
	require.True(t, ok, "counterpart without REST resources", counterpart.Name)
	return resources
}

func TestCountsOneBreakPerFieldBlockAndPerEndpointBreak(t *testing.T) {
	report := fromStoredCheck(consumerSideCheck())

	assert.True(t, report.Stored)
	assert.Equal(t, 3, report.Breaks)
	assert.Equal(t, 2, report.CounterpartsBreaking)
	assert.Equal(t, 2, report.Places)

	api := counterpartNamed(t, report, "petstore_api")
	assert.Equal(t, 2, api.Breaks)
	require.Len(t, restResources(t, api).Fields, 1)
	assert.Len(t, restResources(t, api).Fields[0].Places, 2)
	assert.Equal(t, 1, counterpartNamed(t, report, "petstore_reviews").Breaks)
	assert.Equal(t, 0, counterpartNamed(t, report, "petstore_users").Breaks)
}

func TestSortsCounterpartsByName(t *testing.T) {
	report := fromStoredCheck(consumerSideCheck())

	names := make([]string, 0, len(report.Counterparts))
	for _, counterpart := range report.Counterparts {
		names = append(names, counterpart.Name)
	}
	assert.Equal(t, []string{"petstore_api", "petstore_reviews", "petstore_users"}, names)
}

func TestKeepsTheStoredRoleVersionAndRegistration(t *testing.T) {
	api := counterpartNamed(t, fromStoredCheck(consumerSideCheck()), "petstore_api")

	assert.Equal(t, model.CounterpartBoth, api.Role)
	assert.Equal(t, "1.4.0", api.Version)
	assert.True(t, api.Registered)
	assert.False(t, api.Deployable)
}

func TestFieldBlockMergesPlacesAndNamesTheCounterpartField(t *testing.T) {
	field := restResources(t, counterpartNamed(t, fromStoredCheck(consumerSideCheck()), "petstore_api")).Fields[0]

	assert.Equal(t, "Pet.weight", field.Field)
	assert.Equal(t, model.ReasonPropertyTypeMismatch, field.Reason)
	assert.Equal(t, "petstore_web reads it as string, but petstore_api provides float", field.Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_web", Action: "read it as float", Checked: true},
		{Participant: "petstore_api", Action: "provide it as string there"},
	}, field.Fixes)
	assert.Equal(t, []restbreakreport.Place{
		{Method: "GET", Endpoint: "/pets", Interaction: "response 200", Property: `"$[].weight"`, CounterpartField: "Animal.weight"},
		{Method: "GET", Endpoint: "/pets/*", Interaction: "response 200", Property: `"$.weight"`},
	}, field.Places)
}

func TestEndpointBreakCarriesItsInteractionSentenceReasonAndFixes(t *testing.T) {
	api := counterpartNamed(t, fromStoredCheck(consumerSideCheck()), "petstore_api")

	require.Len(t, restResources(t, api).Endpoints, 1)
	endpoint := restResources(t, api).Endpoints[0]
	assert.Equal(t, "POST", endpoint.Method)
	assert.Equal(t, "/pets/*/adopt", endpoint.Path)
	require.Len(t, endpoint.Breaks, 1)

	broken := endpoint.Breaks[0]
	assert.Equal(t, "response 200", broken.Interaction)
	assert.Equal(t, "petstore_web reads response 200, but petstore_api answers only 204 or 404", broken.Sentence.String())
	assert.Equal(t, "petstore_api 1.4.0, deployed in production, declares the endpoint but not this interaction (declared: 204, 404)", broken.ReasonLine)
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_web", Action: "read response 204 or 404", Checked: true},
		{Participant: "petstore_api", Action: "answer 200"},
	}, broken.Fixes)
}

func TestNotDeployedBreakHasOnlyTheCounterpartFix(t *testing.T) {
	reviews := counterpartNamed(t, fromStoredCheck(consumerSideCheck()), "petstore_reviews")

	broken := restResources(t, reviews).Endpoints[0].Breaks[0]
	assert.Equal(t, "petstore_web calls GET /reviews, but petstore_reviews is not deployed in production", broken.Sentence.String())
	assert.Equal(t, "not deployed in production", broken.ReasonLine)
	assert.Equal(t, []breakreport.Fix{{Participant: "petstore_reviews", Action: "deploy it to production first"}}, broken.Fixes)
}

func TestSentenceMarksTheNamesAndTheResource(t *testing.T) {
	sentence := restResources(t, counterpartNamed(t, fromStoredCheck(consumerSideCheck()), "petstore_reviews")).Endpoints[0].Breaks[0].Sentence

	var names, codes []string
	for _, segment := range sentence {
		if segment.IsName() {
			names = append(names, segment.Text)
		}
		if segment.IsCode() {
			codes = append(codes, segment.Text)
		}
	}
	assert.Equal(t, []string{"petstore_web", "petstore_reviews", "production"}, names)
	assert.Equal(t, []string{"GET /reviews"}, codes)
}

func TestRemovedCountsOncePerEndpointAndMethodAndInteractionRemovedOncePerInteraction(t *testing.T) {
	report := fromStoredCheck(model.CompatibilityCheck{
		ParticipantName: "petstore_api",
		EnvironmentName: checkEnvironment,
		Report:          []byte(providerSideReport),
	}, []model.CompatibilityCheckResult{{CounterpartName: "petstore_web", Role: model.CounterpartConsumer, CounterpartVersion: null.StringFrom("2.2.0"), CounterpartParticipantID: null.IntFrom(1)}})

	assert.Equal(t, 3, report.Breaks)
	assert.Equal(t, 0, report.Places)

	web := report.Counterparts[0]
	require.Len(t, restResources(t, web).Endpoints, 2)

	interactionRemoved := restResources(t, web).Endpoints[0]
	assert.Equal(t, "/pets", interactionRemoved.Path)
	require.Len(t, interactionRemoved.Breaks, 2)
	assert.Equal(t, "response 200", interactionRemoved.Breaks[0].Interaction)
	assert.Equal(t, "response 404", interactionRemoved.Breaks[1].Interaction)
	assert.Equal(t, model.ReasonProviderResourceInteractionRemovedButStillConsumed, interactionRemoved.Breaks[0].Reason)
	assert.Equal(t, "petstore_api now answers GET /pets only with 204, but petstore_web still reads response 200", interactionRemoved.Breaks[0].Sentence.String())
	assert.Empty(t, interactionRemoved.Breaks[0].ReasonLine)

	removed := restResources(t, web).Endpoints[1]
	assert.Equal(t, "/pets/*/adopt", removed.Path)
	require.Len(t, removed.Breaks, 1)
	assert.Empty(t, removed.Breaks[0].Interaction)
	assert.Equal(t, "petstore_api removed POST /pets/*/adopt, but petstore_web still calls it", removed.Breaks[0].Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_api", Action: "keep it until petstore_web stops calling it", Checked: true},
		{Participant: "petstore_web", Action: "stop calling it"},
	}, removed.Breaks[0].Fixes)
}

func TestProviderSideFieldBreakPutsTheCheckedProviderFixFirst(t *testing.T) {
	report := fromStoredCheck(model.CompatibilityCheck{
		ParticipantName: "petstore_api",
		EnvironmentName: checkEnvironment,
		Report: []byte(`{"petstore_web": {"deployable": false, "participantVersion": "2.2.0", "endpoints": {
			"/pets": {"get": {"200": [{"reason": "property_type_mismatch", "role": "provider", "details": {"property": "$", "consumerField": "Pets", "consumerPropertyType": "array<object>", "providerPropertyType": "object"}}]}}
		}}}`),
	}, []model.CompatibilityCheckResult{{CounterpartName: "petstore_web", Role: model.CounterpartConsumer, CounterpartVersion: null.StringFrom("2.2.0"), CounterpartParticipantID: null.IntFrom(1)}})

	field := restResources(t, report.Counterparts[0]).Fields[0]
	assert.Equal(t, "Pets", field.Field)
	assert.Equal(t, "petstore_api provides it as object, but petstore_web reads array<object>", field.Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_api", Action: "provide it as array<object>", Checked: true},
		{Participant: "petstore_web", Action: "read it as object there"},
	}, field.Fixes)
	assert.Equal(t, []restbreakreport.Place{{Method: "GET", Endpoint: "/pets", Interaction: "response 200", Property: "the body", CounterpartField: "Pets"}}, field.Places)
}

func TestDistinctFieldBreaksCountSeparately(t *testing.T) {
	report := fromStoredCheck(model.CompatibilityCheck{
		ParticipantName: checkedParticipant,
		EnvironmentName: checkEnvironment,
		Report: []byte(`{"petstore_api": {"deployable": false, "participantVersion": "1.4.0", "endpoints": {
			"/pets": {"get": {"200": [
				{"reason": "property_missing_in_provider", "role": "consumer", "details": {"property": "$[].status", "consumerField": "Pet.status", "propertyType": "string"}},
				{"reason": "property_missing_in_provider", "role": "consumer", "details": {"property": "$[].tag", "consumerField": "Pet.tag", "propertyType": "string"}}
			]}}
		}}}`),
	}, []model.CompatibilityCheckResult{{CounterpartName: "petstore_api", Role: model.CounterpartProvider, CounterpartVersion: null.StringFrom("1.4.0"), CounterpartParticipantID: null.IntFrom(1)}})

	assert.Equal(t, 2, report.Breaks)
	assert.Equal(t, 2, report.Places)
	assert.Empty(t, restResources(t, report.Counterparts[0]).Endpoints)
	assert.Equal(t, "Pet.status", restResources(t, report.Counterparts[0]).Fields[0].Field)
	assert.Equal(t, "provide it", restResources(t, report.Counterparts[0]).Fields[0].Fixes[1].Action)
}

func TestNotFoundReasonNamesTheDeployedVersion(t *testing.T) {
	report := fromStoredCheck(model.CompatibilityCheck{
		ParticipantName: checkedParticipant,
		EnvironmentName: checkEnvironment,
		Report: []byte(`{"petstore_api": {"deployable": false, "participantVersion": "1.4.0", "endpoints": {
			"/pets": {"get": {"200": [{"reason": "provider_resource_not_found", "role": "consumer"}]}}
		}}}`),
	}, []model.CompatibilityCheckResult{{CounterpartName: "petstore_api", Role: model.CounterpartProvider, CounterpartVersion: null.StringFrom("1.4.0"), CounterpartParticipantID: null.IntFrom(1)}})

	assert.Equal(t, "petstore_api 1.4.0, deployed in production, doesn't provide this endpoint",
		restResources(t, report.Counterparts[0]).Endpoints[0].Breaks[0].ReasonLine)
}

func TestNotDeployedReasonIsTheSameWhetherTheCounterpartIsAParticipantOrNot(t *testing.T) {
	notDeployed := `{"petstore_api": {"deployable": false, "participantVersion": null, "endpoints": {
		"/pets": {"get": {"200": [{"reason": "provider_resource_not_deployed_in_environment", "role": "consumer"}]}}
	}}}`
	reasonFor := func(registered bool) string {
		report := fromStoredCheck(model.CompatibilityCheck{
			ParticipantName: checkedParticipant,
			EnvironmentName: checkEnvironment,
			Report:          []byte(notDeployed),
		}, []model.CompatibilityCheckResult{{CounterpartName: "petstore_api", Role: model.CounterpartProvider, CounterpartParticipantID: null.NewInt(1, registered)}})
		return restResources(t, report.Counterparts[0]).Endpoints[0].Breaks[0].ReasonLine
	}

	assert.Equal(t, "not deployed in production", reasonFor(true))
	assert.Equal(t, "not deployed in production", reasonFor(false))
}

func TestUnknownBreakFallsBackToItsReasonAndDetails(t *testing.T) {
	report := fromStoredCheck(model.CompatibilityCheck{
		ParticipantName: checkedParticipant,
		EnvironmentName: checkEnvironment,
		Report: []byte(`{"petstore_api": {"deployable": false, "participantVersion": "1.4.0", "endpoints": {
			"/pets": {"get": {"200": [{"reason": "property_missing_in_provider", "role": "consumer", "details": {"propertyType": "string"}}]}}
		}}}`),
	}, []model.CompatibilityCheckResult{{CounterpartName: "petstore_api", CounterpartVersion: null.StringFrom("1.4.0"), CounterpartParticipantID: null.IntFrom(1)}})

	assert.Equal(t, 1, report.Breaks)
	broken := restResources(t, report.Counterparts[0]).Endpoints[0].Breaks[0]
	assert.Equal(t, "property_missing_in_provider (propertyType: string)", broken.Sentence.String())
	assert.Empty(t, broken.Fixes)
}

func TestCheckWithoutAStoredReportListsTheCounterpartsWithoutCounts(t *testing.T) {
	report := fromStoredCheck(model.CompatibilityCheck{
		ParticipantName: checkedParticipant,
		EnvironmentName: checkEnvironment,
	}, []model.CompatibilityCheckResult{
		{CounterpartName: "petstore_reviews", Deployable: true},
		{CounterpartName: "petstore_api", CounterpartVersion: null.StringFrom("1.4.0"), CounterpartParticipantID: null.IntFrom(1)},
	})

	assert.False(t, report.Stored)
	assert.Zero(t, report.Breaks)
	assert.Equal(t, []breakreport.Counterpart{
		{Name: "petstore_api", Version: "1.4.0", Registered: true},
		{Name: "petstore_reviews", Deployable: true},
	}, report.Counterparts)
}

func TestUndecodableReportPanics(t *testing.T) {
	assert.Panics(t, func() {
		breakreport.NewReader([]breakreport.ProtocolSection{restbreakreport.New()}).FromStoredCheck(model.CompatibilityCheck{Report: []byte(`[`)}, nil)
	})
}

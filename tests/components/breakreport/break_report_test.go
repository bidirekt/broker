package breakreport_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
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
        {"reason": "provider_resource_not_deployed_in_environment", "role": "consumer", "details": {"deployedEnvironments": "staging, qa"}}
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

func consumerSideCheck() breakreport.StoredCheck {
	return breakreport.StoredCheck{
		Participant: checkedParticipant,
		Environment: checkEnvironment,
		Report:      []byte(consumerSideReport),
		Counterparts: []breakreport.StoredCounterpart{
			{Name: "petstore_users", Role: model.CounterpartProvider, Version: "3.0.0", Registered: true, Deployable: true},
			{Name: "petstore_reviews", Role: model.CounterpartProvider, Registered: true},
			{Name: "petstore_api", Role: model.CounterpartBoth, Version: "1.4.0", Registered: true},
		},
	}
}

func fromStoredCheck(t *testing.T, check breakreport.StoredCheck) breakreport.Report {
	report, err := breakreport.FromStoredCheck(check)
	require.NoError(t, err)
	return report
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

func TestCountsOneBreakPerFieldBlockAndPerEndpointBreak(t *testing.T) {
	report := fromStoredCheck(t, consumerSideCheck())

	assert.True(t, report.Stored)
	assert.Equal(t, 3, report.Breaks)
	assert.Equal(t, 2, report.CounterpartsBreaking)
	assert.Equal(t, 2, report.Places)

	api := counterpartNamed(t, report, "petstore_api")
	assert.Equal(t, 2, api.Breaks)
	require.Len(t, api.Fields, 1)
	assert.Len(t, api.Fields[0].Places, 2)
	assert.Equal(t, 1, counterpartNamed(t, report, "petstore_reviews").Breaks)
	assert.Equal(t, 0, counterpartNamed(t, report, "petstore_users").Breaks)
}

func TestSortsCounterpartsByName(t *testing.T) {
	report := fromStoredCheck(t, consumerSideCheck())

	names := make([]string, 0, len(report.Counterparts))
	for _, counterpart := range report.Counterparts {
		names = append(names, counterpart.Name)
	}
	assert.Equal(t, []string{"petstore_api", "petstore_reviews", "petstore_users"}, names)
}

func TestKeepsTheStoredRoleVersionAndRegistration(t *testing.T) {
	api := counterpartNamed(t, fromStoredCheck(t, consumerSideCheck()), "petstore_api")

	assert.Equal(t, model.CounterpartBoth, api.Role)
	assert.Equal(t, "1.4.0", api.Version)
	assert.True(t, api.Registered)
	assert.False(t, api.Deployable)
}

func TestFieldBlockMergesPlacesAndNamesTheCounterpartField(t *testing.T) {
	field := counterpartNamed(t, fromStoredCheck(t, consumerSideCheck()), "petstore_api").Fields[0]

	assert.Equal(t, "Pet.weight", field.Field)
	assert.Equal(t, model.ReasonPropertyTypeMismatch, field.Reason)
	assert.Equal(t, "petstore_web reads it as string, but petstore_api provides float", field.Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_web", Action: "read it as float", Checked: true},
		{Participant: "petstore_api", Action: "provide it as string there"},
	}, field.Fixes)
	assert.Equal(t, []breakreport.Place{
		{Method: "GET", Endpoint: "/pets", Interaction: "response 200", Property: `"$[].weight"`, CounterpartField: "Animal.weight"},
		{Method: "GET", Endpoint: "/pets/*", Interaction: "response 200", Property: `"$.weight"`},
	}, field.Places)
}

func TestEndpointBreakCarriesItsInteractionSentenceReasonAndFixes(t *testing.T) {
	api := counterpartNamed(t, fromStoredCheck(t, consumerSideCheck()), "petstore_api")

	require.Len(t, api.Endpoints, 1)
	endpoint := api.Endpoints[0]
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

func TestNotDeployedBreakHasOnlyTheCounterpartFixAndListsWhereItIsDeployed(t *testing.T) {
	reviews := counterpartNamed(t, fromStoredCheck(t, consumerSideCheck()), "petstore_reviews")

	broken := reviews.Endpoints[0].Breaks[0]
	assert.Equal(t, "petstore_web calls GET /reviews, but petstore_reviews is not deployed in production", broken.Sentence.String())
	assert.Equal(t, "not deployed in production (deployed in: staging, qa)", broken.ReasonLine)
	assert.Equal(t, []breakreport.Fix{{Participant: "petstore_reviews", Action: "deploy it to production first"}}, broken.Fixes)
}

func TestSentenceMarksTheNamesAndTheResource(t *testing.T) {
	sentence := counterpartNamed(t, fromStoredCheck(t, consumerSideCheck()), "petstore_reviews").Endpoints[0].Breaks[0].Sentence

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
	report := fromStoredCheck(t, breakreport.StoredCheck{
		Participant:  "petstore_api",
		Environment:  checkEnvironment,
		Report:       []byte(providerSideReport),
		Counterparts: []breakreport.StoredCounterpart{{Name: "petstore_web", Role: model.CounterpartConsumer, Version: "2.2.0", Registered: true}},
	})

	assert.Equal(t, 3, report.Breaks)
	assert.Equal(t, 0, report.Places)

	web := report.Counterparts[0]
	require.Len(t, web.Endpoints, 2)

	interactionRemoved := web.Endpoints[0]
	assert.Equal(t, "/pets", interactionRemoved.Path)
	require.Len(t, interactionRemoved.Breaks, 2)
	assert.Equal(t, "response 200", interactionRemoved.Breaks[0].Interaction)
	assert.Equal(t, "response 404", interactionRemoved.Breaks[1].Interaction)
	assert.Equal(t, model.ReasonProviderResourceInteractionRemovedButStillConsumed, interactionRemoved.Breaks[0].Reason)
	assert.Equal(t, "petstore_api now answers GET /pets only with 204, but petstore_web still reads response 200", interactionRemoved.Breaks[0].Sentence.String())
	assert.Empty(t, interactionRemoved.Breaks[0].ReasonLine)

	removed := web.Endpoints[1]
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
	report := fromStoredCheck(t, breakreport.StoredCheck{
		Participant: "petstore_api",
		Environment: checkEnvironment,
		Report: []byte(`{"petstore_web": {"deployable": false, "participantVersion": "2.2.0", "endpoints": {
			"/pets": {"get": {"200": [{"reason": "property_type_mismatch", "role": "provider", "details": {"property": "$", "consumerField": "Pets", "consumerPropertyType": "array<object>", "providerPropertyType": "object"}}]}}
		}}}`),
		Counterparts: []breakreport.StoredCounterpart{{Name: "petstore_web", Role: model.CounterpartConsumer, Version: "2.2.0", Registered: true}},
	})

	field := report.Counterparts[0].Fields[0]
	assert.Equal(t, "Pets", field.Field)
	assert.Equal(t, "petstore_api provides it as object, but petstore_web reads array<object>", field.Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_api", Action: "provide it as array<object>", Checked: true},
		{Participant: "petstore_web", Action: "read it as object there"},
	}, field.Fixes)
	assert.Equal(t, []breakreport.Place{{Method: "GET", Endpoint: "/pets", Interaction: "response 200", Property: "the body", CounterpartField: "Pets"}}, field.Places)
}

func TestDistinctFieldBreaksCountSeparately(t *testing.T) {
	report := fromStoredCheck(t, breakreport.StoredCheck{
		Participant: checkedParticipant,
		Environment: checkEnvironment,
		Report: []byte(`{"petstore_api": {"deployable": false, "participantVersion": "1.4.0", "endpoints": {
			"/pets": {"get": {"200": [
				{"reason": "property_missing_in_provider", "role": "consumer", "details": {"property": "$[].status", "consumerField": "Pet.status", "propertyType": "string"}},
				{"reason": "property_missing_in_provider", "role": "consumer", "details": {"property": "$[].tag", "consumerField": "Pet.tag", "propertyType": "string"}}
			]}}
		}}}`),
		Counterparts: []breakreport.StoredCounterpart{{Name: "petstore_api", Role: model.CounterpartProvider, Version: "1.4.0", Registered: true}},
	})

	assert.Equal(t, 2, report.Breaks)
	assert.Equal(t, 2, report.Places)
	assert.Empty(t, report.Counterparts[0].Endpoints)
	assert.Equal(t, "Pet.status", report.Counterparts[0].Fields[0].Field)
	assert.Equal(t, "provide it", report.Counterparts[0].Fields[0].Fixes[1].Action)
}

func TestNotFoundReasonDependsOnTheStoredVersionAndRegistration(t *testing.T) {
	notFound := `{"petstore_api": {"deployable": false, "participantVersion": null, "endpoints": {
		"/pets": {"get": {"200": [{"reason": "provider_resource_not_found", "role": "consumer"}]}}
	}}}`
	reasonFor := func(counterpart breakreport.StoredCounterpart) string {
		counterpart.Name = "petstore_api"
		report := fromStoredCheck(t, breakreport.StoredCheck{
			Participant:  checkedParticipant,
			Environment:  checkEnvironment,
			Report:       []byte(notFound),
			Counterparts: []breakreport.StoredCounterpart{counterpart},
		})
		return report.Counterparts[0].Endpoints[0].Breaks[0].ReasonLine
	}

	assert.Equal(t, "petstore_api 1.4.0, deployed in production, doesn't provide this endpoint",
		reasonFor(breakreport.StoredCounterpart{Version: "1.4.0", Registered: true}))
	assert.Equal(t, "petstore_api is not deployed in production, and no version it published provides this endpoint",
		reasonFor(breakreport.StoredCounterpart{Registered: true}))
	assert.Equal(t, "petstore_api is not a participant of this broker",
		reasonFor(breakreport.StoredCounterpart{}))
}

func TestUnknownBreakFallsBackToItsReasonAndDetails(t *testing.T) {
	report := fromStoredCheck(t, breakreport.StoredCheck{
		Participant: checkedParticipant,
		Environment: checkEnvironment,
		Report: []byte(`{"petstore_api": {"deployable": false, "participantVersion": "1.4.0", "endpoints": {
			"/pets": {"get": {"200": [{"reason": "property_missing_in_provider", "role": "consumer", "details": {"propertyType": "string"}}]}}
		}}}`),
		Counterparts: []breakreport.StoredCounterpart{{Name: "petstore_api", Version: "1.4.0", Registered: true}},
	})

	assert.Equal(t, 1, report.Breaks)
	broken := report.Counterparts[0].Endpoints[0].Breaks[0]
	assert.Equal(t, "property_missing_in_provider (propertyType: string)", broken.Sentence.String())
	assert.Empty(t, broken.Fixes)
}

func TestCheckWithoutAStoredReportListsTheCounterpartsWithoutCounts(t *testing.T) {
	report := fromStoredCheck(t, breakreport.StoredCheck{
		Participant: checkedParticipant,
		Environment: checkEnvironment,
		Counterparts: []breakreport.StoredCounterpart{
			{Name: "petstore_reviews", Deployable: true},
			{Name: "petstore_api", Version: "1.4.0", Registered: true},
		},
	})

	assert.False(t, report.Stored)
	assert.Zero(t, report.Breaks)
	assert.Equal(t, []breakreport.Counterpart{
		{Name: "petstore_api", Version: "1.4.0", Registered: true},
		{Name: "petstore_reviews", Deployable: true},
	}, report.Counterparts)
}

func TestUndecodableReportIsAnError(t *testing.T) {
	_, err := breakreport.FromStoredCheck(breakreport.StoredCheck{Report: []byte(`[`)})

	assert.Error(t, err)
}

package compatibility_checker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

const extraComposedName = "pets_composed"

type extraWithoutVerdict struct{}

func (this extraWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type extraHooks struct {
	restcontracts.Plugin
	pairs func(contract *model.PersistedContract) compatibility_checker.ExtraPairs
}

func (this extraHooks) ExtraCounterparts(_ context.Context, contract *model.PersistedContract, _ *model.Environment) compatibility_checker.ExtraPairs {
	return this.pairs(contract)
}

func extraPetsResource(direction model.Direction, participant string) model.PersistedResource {
	resource := model.PersistedResource{
		Resource: model.Resource{
			ParticipantName: participant,
			Direction:       direction,
			Protocol:        restmodel.Protocol,
			Body:            model.Output,
			Identity:        restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"},
			Bodies:          []model.ResourceBody{{Source: participant + ".yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("Pet")}},
		},
		ProviderHash: "get-pets",
		Schemas:      map[string]model.Schemas{model.YAMLNamespace: {"Pet": {"": {Type: model.ObjectType}, "name": {Type: model.StringType}}}},
	}

	if direction == model.Consumes {
		resource.ConsumedProvider = null.StringFrom(extraComposedName)
		resource.ConsumerHash = null.StringFrom("web-get-pets")
	}

	return resource
}

func extraCheck(contract *model.PersistedContract, pairs func(*model.PersistedContract) compatibility_checker.ExtraPairs) *compatibility_checker.ContractCompatibilityReport {
	hooks := extraHooks{Plugin: restcontracts.New(), pairs: pairs}

	return compatibility_checker.NewCompatibilityChecker(extraWithoutVerdict{}, []compatibility_checker.ProtocolHooks{hooks}).Check(
		context.Background(),
		contract,
		model.NewEnvironment("production"),
		model.ResourceCounterparts{},
	)
}

func extraConsumerContract() *model.PersistedContract {
	consumer := extraPetsResource(model.Consumes, "petstore_web")

	return &model.PersistedContract{
		ParticipantName: "petstore_web",
		Version:         "3.0.0",
		Resources:       map[string]model.PersistedResource{consumer.PrimaryHash(): consumer},
	}
}

func TestExtraCounterparts_AComposedCounterpart_IsComparedAndWritesItsMembersForItsVersion(t *testing.T) {
	contract := extraConsumerContract()
	provider := extraPetsResource(model.Provides, extraComposedName)
	provider.Schemas = map[string]model.Schemas{model.YAMLNamespace: {"Pet": {"": {Type: model.ObjectType}}}}

	report := extraCheck(contract, func(contract *model.PersistedContract) compatibility_checker.ExtraPairs {
		return compatibility_checker.ExtraPairs{
			Providers: map[string]bool{extraComposedName: true},
			Pairs: []compatibility_checker.ExtraPair{{
				Resources: []model.PersistedResource{contract.Resources["web-get-pets"]},
				Contract:  &model.PersistedContract{Resources: map[string]model.PersistedResource{"get-pets": provider}},
				Counterpart: compatibility_checker.Counterpart{
					ParticipantName: extraComposedName,
					Role:            model.CounterpartProvider,
					Members:         map[string]string{"petstore_reviews": "2.0.0", "petstore_api": "1.4.0"},
				},
			}},
		}
	})

	wire, err := json.Marshal(report.Hierarchical)
	require.NoError(t, err)

	assert.JSONEq(t, `{"pets_composed": {
		"deployable": false,
		"members": {"petstore_api": "1.4.0", "petstore_reviews": "2.0.0"},
		"endpoints": {"/pets": {"get": {"200": [
			{"reason": "property_missing_in_provider", "role": "consumer", "details": {"consumerName": "petstore_web", "providerName": "pets_composed", "property": "$.name", "propertyType": "string", "consumerField": "Pet.name"}}
		]}}}
	}}`, string(wire))
	assert.Equal(t, null.StringFrom("petstore_api 1.4.0 + petstore_reviews 2.0.0"), report.Results[extraComposedName].Counterpart.Version())
	assert.Empty(t, compatibility_checker.ToVerdictBreaks(report.Results[extraComposedName]))
}

func TestExtraCounterparts_AReadyBreak_CarriesTheRoleOfTheSideUnderCheck(t *testing.T) {
	contract := extraConsumerContract()
	counterpartConsumer := extraPetsResource(model.Consumes, "petstore_web")

	report := extraCheck(contract, func(*model.PersistedContract) compatibility_checker.ExtraPairs {
		return compatibility_checker.ExtraPairs{
			Pairs: []compatibility_checker.ExtraPair{{
				Resources:   []model.PersistedResource{counterpartConsumer},
				Counterpart: compatibility_checker.Counterpart{ParticipantName: "petstore_admin", Role: model.CounterpartConsumer},
				ReadyBreak: compatibility_checker.ReadyBreak{
					Reason:  model.ReasonProviderResourceRemovedButStillConsumed,
					Details: map[string]string{"composed": extraComposedName},
				},
			}},
		}
	})

	assert.Equal(t, []model.ReportBreak{{
		Reason:  model.ReasonProviderResourceRemovedButStillConsumed,
		Role:    model.RoleProvider,
		Details: map[string]string{"composed": extraComposedName},
	}}, report.Hierarchical["petstore_admin"].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["get"]["200"])
	assert.False(t, report.Results["petstore_admin"].Deployable)
}

func TestExtraCounterparts_AProviderThePortTakesOver_IsNeverCheckedAsADeployment(t *testing.T) {
	report := extraCheck(extraConsumerContract(), func(*model.PersistedContract) compatibility_checker.ExtraPairs {
		return compatibility_checker.ExtraPairs{Providers: map[string]bool{extraComposedName: true}}
	})

	assert.Empty(t, report.Results)
	assert.True(t, report.Deployable())
}

func TestExtraCounterparts_TheProviderUnderCheck_ReportsOnlyTheBreaksItsBaseDoesNotHave(t *testing.T) {
	contract := extraConsumerContract()
	counterpartConsumer := extraPetsResource(model.Consumes, "petstore_web")
	counterpartConsumer.Schemas = map[string]model.Schemas{model.YAMLNamespace: {"Pet": {
		"":     {Type: model.ObjectType},
		"name": {Type: model.StringType},
		"age":  {Type: model.IntegerType},
	}}}

	withName := extraPetsResource(model.Provides, extraComposedName)
	withoutName := extraPetsResource(model.Provides, extraComposedName)
	withoutName.Schemas = map[string]model.Schemas{model.YAMLNamespace: {"Pet": {"": {Type: model.ObjectType}}}}

	report := extraCheck(contract, func(*model.PersistedContract) compatibility_checker.ExtraPairs {
		return compatibility_checker.ExtraPairs{
			Pairs: []compatibility_checker.ExtraPair{{
				Resources:   []model.PersistedResource{counterpartConsumer},
				Contract:    &model.PersistedContract{Resources: map[string]model.PersistedResource{"get-pets": withoutName}},
				Base:        &model.PersistedContract{Resources: map[string]model.PersistedResource{"get-pets": withName}},
				Counterpart: compatibility_checker.Counterpart{ParticipantName: "petstore_admin", Role: model.CounterpartConsumer},
				Details:     map[string]string{"composed": extraComposedName},
			}},
		}
	})

	assert.Equal(t, []model.ReportBreak{{
		Reason: model.ReasonPropertyMissingInProvider,
		Role:   model.RoleProvider,
		Details: map[string]string{
			"consumerName":  "petstore_web",
			"providerName":  extraComposedName,
			"property":      "$.name",
			"propertyType":  "string",
			"consumerField": "Pet.name",
			"composed":      extraComposedName,
		},
	}}, report.Hierarchical["petstore_admin"].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["get"]["200"])
	assert.Empty(t, compatibility_checker.ToVerdictBreaks(report.Results["petstore_admin"]))
}

func TestExtraCounterparts_AResourceTheProviderUnderCheckDropsAndItsBaseHad_IsRemovedButStillConsumed(t *testing.T) {
	counterpartConsumer := extraPetsResource(model.Consumes, "petstore_web")

	report := extraCheck(extraConsumerContract(), func(*model.PersistedContract) compatibility_checker.ExtraPairs {
		return compatibility_checker.ExtraPairs{
			Pairs: []compatibility_checker.ExtraPair{{
				Resources:   []model.PersistedResource{counterpartConsumer},
				Contract:    &model.PersistedContract{Resources: map[string]model.PersistedResource{}},
				Base:        &model.PersistedContract{Resources: map[string]model.PersistedResource{"get-pets": extraPetsResource(model.Provides, extraComposedName)}},
				Counterpart: compatibility_checker.Counterpart{ParticipantName: "petstore_admin", Role: model.CounterpartConsumer},
			}},
		}
	})

	assert.Equal(t, []model.ReportBreak{{
		Reason: model.ReasonProviderResourceRemovedButStillConsumed,
		Role:   model.RoleProvider,
	}}, report.Hierarchical["petstore_admin"].Trees[restmodel.Protocol].(restmodel.ReportTree)["/pets"]["get"]["200"])
}

func TestExtraCounterparts_ABreakTheBaseReportsAtAnotherPath_IsNotChargedToTheProviderUnderCheck(t *testing.T) {
	counterpartConsumer := extraPetsResource(model.Consumes, "petstore_web")
	counterpartConsumer.Schemas = map[string]model.Schemas{model.YAMLNamespace: {
		"Pet":   {"": {Type: model.ObjectType}, "a": {Ref: "Money"}, "b": {Ref: "Money"}},
		"Money": {"": {Type: model.ObjectType}, "amount": {Type: model.StringType}},
	}}

	base := extraPetsResource(model.Provides, extraComposedName)
	base.Schemas = map[string]model.Schemas{model.YAMLNamespace: {
		"Pet":   {"": {Type: model.ObjectType}, "a": {Ref: "Money"}, "b": {Ref: "Money"}},
		"Money": {"": {Type: model.ObjectType}, "amount": {Type: model.IntegerType}},
	}}

	fixesA := extraPetsResource(model.Provides, extraComposedName)
	fixesA.Schemas = map[string]model.Schemas{model.YAMLNamespace: {
		"Pet":   {"": {Type: model.ObjectType}, "a": {Ref: "Price"}, "b": {Ref: "Money"}},
		"Money": {"": {Type: model.ObjectType}, "amount": {Type: model.IntegerType}},
		"Price": {"": {Type: model.ObjectType}, "amount": {Type: model.StringType}},
	}}

	report := extraCheck(extraConsumerContract(), func(*model.PersistedContract) compatibility_checker.ExtraPairs {
		return compatibility_checker.ExtraPairs{
			Pairs: []compatibility_checker.ExtraPair{{
				Resources:   []model.PersistedResource{counterpartConsumer},
				Contract:    &model.PersistedContract{Resources: map[string]model.PersistedResource{"get-pets": fixesA}},
				Base:        &model.PersistedContract{Resources: map[string]model.PersistedResource{"get-pets": base}},
				Counterpart: compatibility_checker.Counterpart{ParticipantName: "petstore_admin", Role: model.CounterpartConsumer},
			}},
		}
	})

	assert.Empty(t, report.Hierarchical["petstore_admin"].Trees[restmodel.Protocol])
	assert.True(t, report.Results["petstore_admin"].Deployable)
}

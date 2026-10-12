package compatibility_checker_test

import (
	"context"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	billingConsumerName = "spa"
	billingProviderName = "api"
	billingEnvironment  = "production"
	billingProviderHash = "put-api-billing"
	billingEndpoint     = "/api/billing"
	billingMethod       = "put"
)

type billingWithoutVerdict struct{}

func (this billingWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type billingSide struct {
	bodies  []model.ResourceBody
	schemas model.Schemas
}

func billingBody(schemaName string, schemas model.Schemas) billingSide {
	return billingSide{
		bodies:  []model.ResourceBody{{Source: "billing.yaml", SchemaName: null.StringFrom(schemaName)}},
		schemas: schemas,
	}
}

func billingFieldsCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []map[string]string {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(billingWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(billingEnvironment),
		counterparts,
	)

	var fields []map[string]string
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints[billingEndpoint][billingMethod][interaction] {
		named := map[string]string{"property": leaf.Details["property"]}
		for _, key := range []string{"consumerField", "providerField"} {
			if field, present := leaf.Details[key]; present {
				named[key] = field
			}
		}

		fields = append(fields, named)
	}

	return fields
}

func billingFields(t *testing.T, interaction string, consumerSide, providerSide billingSide) []map[string]string {
	t.Helper()

	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumer := model.PersistedResource{
		ParticipantName:    billingConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(billingProviderName),
		ConsumerHash:       null.StringFrom("spa-" + billingProviderHash),
		ProviderHash:       billingProviderHash,
		Endpoint:           billingEndpoint,
		Method:             billingMethod,
		ResponseStatusCode: status,
		Bodies:             consumerSide.bodies,
		Schemas:            consumerSide.schemas,
	}
	provider := model.PersistedResource{
		ParticipantName:    billingProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       billingProviderHash,
		Endpoint:           billingEndpoint,
		Method:             billingMethod,
		ResponseStatusCode: status,
		Bodies:             providerSide.bodies,
		Schemas:            providerSide.schemas,
		DeployedVersions:   map[string]string{billingEnvironment: "1.0.0"},
	}

	fromTheConsumer := billingFieldsCheckedFrom(consumer, model.ResourceCounterparts{
		Providers: map[string]model.PersistedResource{billingProviderHash: provider},
	}, billingProviderName, interaction)

	fromTheProvider := billingFieldsCheckedFrom(provider, model.ResourceCounterparts{
		Consumers: map[string][]model.PersistedResource{billingProviderHash: {consumer}},
	}, billingConsumerName, interaction)

	require.Equal(t, fromTheConsumer, fromTheProvider, "the same pair checked from either side")

	return fromTheConsumer
}

func TestFieldNames_BodyRoot_IsTheSchemaName(t *testing.T) {
	readByTheConsumer := model.Schemas{"Invoice": {"": {Type: "object"}}}
	providedByTheProvider := model.Schemas{"InvoiceList": {"": {Type: "array"}, "[]": {Type: "object"}}}

	assert.Equal(t, []map[string]string{
		{"property": "$", "consumerField": "Invoice", "providerField": "InvoiceList"},
	}, billingFields(t, "200", billingBody("Invoice", readByTheConsumer), billingBody("InvoiceList", providedByTheProvider)))
}

func TestFieldNames_RefField_NamesTheRefAndItsTargetNamesItsChildren(t *testing.T) {
	readByTheConsumer := model.Schemas{
		"Invoice": {"": {Type: "object"}, "billing": {Ref: "Address"}},
		"Address": {"": {Type: "object"}, "city": {Type: "string"}, "zip": {Type: "string"}},
	}
	providedByTheProvider := model.Schemas{
		"Invoice":       {"": {Type: "object"}, "billing": {Ref: "PostalAddress", Nullable: true}},
		"PostalAddress": {"": {Type: "object"}, "city": {Type: "integer"}},
	}

	assert.Equal(t, []map[string]string{
		{"property": "$.billing", "consumerField": "Invoice.billing", "providerField": "Invoice.billing"},
		{"property": "$.billing.city", "consumerField": "Address.city", "providerField": "PostalAddress.city"},
		{"property": "$.billing.zip", "consumerField": "Address.zip"},
	}, billingFields(t, "200", billingBody("Invoice", readByTheConsumer), billingBody("Invoice", providedByTheProvider)))
}

func TestFieldNames_ArrayItemsAndMapValues_KeepTheirSegments(t *testing.T) {
	readByTheConsumer := model.Schemas{"Invoice": {
		"":            {Type: "object"},
		"lines":       {Type: "array"},
		"lines[]":     {Type: "object"},
		"lines[].sku": {Type: "string"},
	}}
	providedByTheProvider := model.Schemas{
		"Invoice": {
			"":        {Type: "object"},
			"lines":   {Type: "array"},
			"lines[]": {Ref: "Line"},
		},
		"Line": {"": {Type: "object"}, "sku": {Type: "integer"}},
	}

	assert.Equal(t, []map[string]string{
		{"property": "$.lines[].sku", "consumerField": "Invoice.lines[].sku", "providerField": "Line.sku"},
	}, billingFields(t, "200", billingBody("Invoice", readByTheConsumer), billingBody("Invoice", providedByTheProvider)))

	sentByTheConsumer := model.Schemas{"Pets": {
		"":        {Type: "object"},
		"{}":      {Type: "object"},
		"{}.name": {Type: "string", Nullable: true},
	}}
	readByTheProvider := model.Schemas{"Pets": {
		"":        {Type: "object"},
		"{}":      {Type: "object"},
		"{}.name": {Type: "string"},
		"{}.age":  {Type: "integer"},
	}}

	assert.Equal(t, []map[string]string{
		{"property": "${}.age", "providerField": "Pets{}.age"},
		{"property": "${}.name", "consumerField": "Pets{}.name", "providerField": "Pets{}.name"},
	}, billingFields(t, "request", billingBody("Pets", sentByTheConsumer), billingBody("Pets", readByTheProvider)))
}

func TestFieldNames_UndescribedBody_LeavesThatSideOut(t *testing.T) {
	invoice := model.Schemas{"Invoice": {"": {Type: "object"}}}
	undescribed := billingSide{bodies: []model.ResourceBody{{Source: "billing.yaml"}}, schemas: model.Schemas{}}

	assert.Equal(t, []map[string]string{
		{"property": "$", "consumerField": "Invoice"},
	}, billingFields(t, "200", billingBody("Invoice", invoice), undescribed))

	assert.Equal(t, []map[string]string{
		{"property": "$", "providerField": "Invoice"},
	}, billingFields(t, "request", undescribed, billingBody("Invoice", invoice)))
}

func TestFieldNames_ConsumerWithSeveralModules_NamesTheFirstModuleDeclaringTheField(t *testing.T) {
	twoModules := billingSide{
		bodies: []model.ResourceBody{
			{Source: "a.yaml", SchemaName: null.StringFrom("InvoiceSummary")},
			{Source: "b.yaml", SchemaName: null.StringFrom("InvoiceDetail")},
		},
		schemas: model.Schemas{
			"InvoiceSummary": {"": {Type: "object"}, "total": {Type: "integer"}},
			"InvoiceDetail":  {"": {Type: "object"}, "total": {Type: "integer"}, "note": {Type: "string"}},
		},
	}
	providedByTheProvider := model.Schemas{"Invoice": {"": {Type: "object"}, "total": {Type: "integer", Optional: true}}}

	assert.Equal(t, []map[string]string{
		{"property": "$.note", "consumerField": "InvoiceDetail.note"},
		{"property": "$.total", "consumerField": "InvoiceSummary.total", "providerField": "Invoice.total"},
	}, billingFields(t, "200", twoModules, billingBody("Invoice", providedByTheProvider)))
}

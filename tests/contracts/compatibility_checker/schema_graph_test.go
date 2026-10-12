package compatibility_checker_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ordersConsumerName = "spa"
	ordersProviderName = "api"
	ordersEnvironment  = "production"
	ordersProviderHash = "post-api-orders"
	ordersEndpoint     = "/api/orders"
	ordersMethod       = "post"
)

type ordersWithoutVerdict struct{}

func (this ordersWithoutVerdict) GetVerdict(context.Context, int64, int64) (*model.CompatibilityVerdict, bool) {
	return nil, false
}

type ordersBreak struct {
	reason  compatibility_checker.BreakingReason
	details map[string]string
}

type ordersSide struct {
	bodies  []model.ResourceBody
	schemas model.Schemas
}

func ordersBody(schemaName string, schemas model.Schemas) ordersSide {
	return ordersSide{
		bodies:  []model.ResourceBody{{Source: "orders.yaml", SchemaName: null.StringFrom(schemaName)}},
		schemas: schemas,
	}
}

func ordersUndescribedBody() ordersSide {
	return ordersSide{bodies: []model.ResourceBody{{Source: "orders.yaml"}}, schemas: model.Schemas{}}
}

func ordersBreaksCheckedFrom(
	checked model.PersistedResource,
	counterparts model.ResourceCounterparts,
	counterpartName, interaction string,
) []ordersBreak {
	checkedContract := &model.PersistedContract{
		ParticipantName: checked.ParticipantName,
		Version:         "2.0.0",
		Resources:       map[string]model.PersistedResource{checked.PrimaryHash(): checked},
	}

	report := compatibility_checker.NewCompatibilityChecker(ordersWithoutVerdict{}).Check(
		context.Background(),
		checkedContract,
		model.NewEnvironment(ordersEnvironment),
		counterparts,
	)

	var breaks []ordersBreak
	for _, leaf := range report.Hierarchical[counterpartName].Endpoints[ordersEndpoint][ordersMethod][interaction] {
		breaks = append(breaks, ordersBreak{leaf.Reason, leaf.Details})
	}

	return breaks
}

func ordersBreaks(t *testing.T, interaction string, consumerSide, providerSide ordersSide) []ordersBreak {
	t.Helper()

	kind, status := model.RestResponse, null.StringFrom(interaction)
	if interaction == "request" {
		kind, status = model.RestRequest, null.String{}
	}

	consumer := model.PersistedResource{
		ParticipantName:    ordersConsumerName,
		Direction:          model.Consumes,
		Interaction:        kind,
		ConsumedProvider:   null.StringFrom(ordersProviderName),
		ConsumerHash:       null.StringFrom("spa-" + ordersProviderHash),
		ProviderHash:       ordersProviderHash,
		Endpoint:           ordersEndpoint,
		Method:             ordersMethod,
		ResponseStatusCode: status,
		Bodies:             consumerSide.bodies,
		Schemas:            consumerSide.schemas,
	}
	provider := model.PersistedResource{
		ParticipantName:    ordersProviderName,
		Direction:          model.Provides,
		Interaction:        kind,
		ProviderHash:       ordersProviderHash,
		Endpoint:           ordersEndpoint,
		Method:             ordersMethod,
		ResponseStatusCode: status,
		Bodies:             providerSide.bodies,
		Schemas:            providerSide.schemas,
		DeployedVersions:   map[string]string{ordersEnvironment: "1.0.0"},
	}

	fromTheConsumer := ordersBreaksCheckedFrom(consumer, model.ResourceCounterparts{
		Providers: map[string]model.PersistedResource{ordersProviderHash: provider},
	}, ordersProviderName, interaction)

	fromTheProvider := ordersBreaksCheckedFrom(provider, model.ResourceCounterparts{
		Consumers: map[string][]model.PersistedResource{ordersProviderHash: {consumer}},
	}, ordersConsumerName, interaction)

	require.Equal(t, fromTheConsumer, fromTheProvider, "the same pair checked from either side")

	return fromTheConsumer
}

func ordersDetails(property string, specific map[string]string) map[string]string {
	details := map[string]string{"property": property, "consumerName": ordersConsumerName, "providerName": ordersProviderName}
	for key, value := range specific {
		details[key] = value
	}

	return details
}

func TestTypeMismatch_StopsTheDescentAtThatField(t *testing.T) {
	readByTheConsumer := model.Schemas{"Order": {
		"":             {Type: "object"},
		"address":      {Type: "object"},
		"address.city": {Type: "string"},
		"address.zip":  {Type: "string"},
		"items":        {Type: "array"},
		"items[]":      {Type: "object"},
		"items[].sku":  {Type: "string"},
	}}
	providedByTheProvider := model.Schemas{"Order": {
		"":            {Type: "object"},
		"address":     {Type: "string"},
		"items":       {Type: "object"},
		"items.sku":   {Type: "string"},
		"items{}":     {Type: "integer"},
		"items{}.sku": {Type: "string"},
	}}

	t.Run("in a response", func(t *testing.T) {
		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.address", map[string]string{
				"consumerField": "Order.address", "providerField": "Order.address", "consumerPropertyType": "object", "providerPropertyType": "string",
			})},
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.items", map[string]string{
				"consumerField": "Order.items", "providerField": "Order.items", "consumerPropertyType": "array<object>", "providerPropertyType": "map<integer>",
			})},
		}, ordersBreaks(t, "201", ordersBody("Order", readByTheConsumer), ordersBody("Order", providedByTheProvider)))
	})

	t.Run("in a request", func(t *testing.T) {
		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.address", map[string]string{
				"consumerField": "Order.address", "providerField": "Order.address", "consumerPropertyType": "string", "providerPropertyType": "object",
			})},
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.items", map[string]string{
				"consumerField": "Order.items", "providerField": "Order.items", "consumerPropertyType": "map<integer>", "providerPropertyType": "array<object>",
			})},
		}, ordersBreaks(t, "request", ordersBody("Order", providedByTheProvider), ordersBody("Order", readByTheConsumer)))
	})
}

func addressCatalog(cityType string, withZip bool) model.Schemas {
	address := model.Schema{
		"":     {Type: "object"},
		"city": {Type: cityType},
	}
	if withZip {
		address["zip"] = model.Field{Type: "string"}
	}

	return model.Schemas{
		"Order": {
			"":          {Type: "object"},
			"shipping":  {Ref: "Address"},
			"billing":   {Ref: "Address"},
			"history":   {Type: "array"},
			"history[]": {Ref: "Address"},
		},
		"Address": address,
	}
}

func TestBreakInsideASchemaUsedInManyPlaces_ComesOutOncePerPair(t *testing.T) {
	assert.Equal(t, []ordersBreak{
		{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.billing.city", map[string]string{
			"consumerField": "Address.city", "providerField": "Address.city", "consumerPropertyType": "string", "providerPropertyType": "integer",
		})},
		{compatibility_checker.ReasonPropertyMissingInProvider, ordersDetails("$.billing.zip", map[string]string{
			"consumerField": "Address.zip", "propertyType": "string",
		})},
	}, ordersBreaks(t, "201",
		ordersBody("Order", addressCatalog("string", true)),
		ordersBody("Order", addressCatalog("integer", false)),
	))
}

func TestSameSchemaReachedThroughFieldsWithOtherFlags_ChecksEachField(t *testing.T) {
	readByTheConsumer := addressCatalog("string", true)
	providedByTheProvider := addressCatalog("string", true)
	providedByTheProvider["Order"]["shipping"] = model.Field{Ref: "Address", Nullable: true}

	assert.Equal(t, []ordersBreak{
		{compatibility_checker.ReasonPropertyNullableInProviderNotNullableInConsumer, ordersDetails("$.shipping", map[string]string{
			"consumerField": "Order.shipping", "providerField": "Order.shipping", "propertyType": "object",
		})},
	}, ordersBreaks(t, "201", ordersBody("Order", readByTheConsumer), ordersBody("Order", providedByTheProvider)))
}

func exponentialCatalog(prefix, leafType string) model.Schemas {
	const levels = 22

	catalog := model.Schemas{}
	for level := range levels {
		next := fmt.Sprintf("%s%d", prefix, level+1)
		catalog[fmt.Sprintf("%s%d", prefix, level)] = model.Schema{
			"":      {Type: "object"},
			"left":  {Ref: next},
			"right": {Ref: next},
		}
	}

	catalog[fmt.Sprintf("%s%d", prefix, levels)] = model.Schema{"": {Type: leafType}}

	return catalog
}

func TestExponentialSchema_ComparesFastAndBreaksOnce(t *testing.T) {
	t.Run("identical on both sides", func(t *testing.T) {
		started := time.Now()

		breaks := ordersBreaks(t, "201",
			ordersBody("S0", exponentialCatalog("S", "string")),
			ordersBody("S0", exponentialCatalog("S", "string")),
		)

		assert.Empty(t, breaks)
		assert.Less(t, time.Since(started), 500*time.Millisecond)
	})

	t.Run("with the leaf type changed", func(t *testing.T) {
		started := time.Now()

		breaks := ordersBreaks(t, "201",
			ordersBody("S0", exponentialCatalog("S", "string")),
			ordersBody("S0", exponentialCatalog("S", "integer")),
		)

		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$"+strings.Repeat(".left", 22), map[string]string{
				"consumerField": "S21.left", "providerField": "S21.left", "consumerPropertyType": "string", "providerPropertyType": "integer",
			})},
		}, breaks)
		assert.Less(t, time.Since(started), 500*time.Millisecond)
	})

	t.Run("read by two consumer modules", func(t *testing.T) {
		catalog := exponentialCatalog("S", "string")
		for name, schema := range exponentialCatalog("T", "string") {
			catalog[name] = schema
		}

		twoModules := ordersSide{
			bodies: []model.ResourceBody{
				{Source: "a.yaml", SchemaName: null.StringFrom("S0")},
				{Source: "b.yaml", SchemaName: null.StringFrom("T0")},
			},
			schemas: catalog,
		}

		started := time.Now()

		assert.Empty(t, ordersBreaks(t, "201", twoModules, ordersBody("S0", exponentialCatalog("S", "string"))))
		assert.Empty(t, ordersBreaks(t, "request", twoModules, ordersBody("S0", exponentialCatalog("S", "string"))))
		assert.Less(t, time.Since(started), 500*time.Millisecond)
	})
}

func commentCatalog(textType string) model.Schemas {
	return model.Schemas{"Comment": {
		"":          {Type: "object"},
		"text":      {Type: textType},
		"replies":   {Type: "array"},
		"replies[]": {Ref: "Comment"},
	}}
}

func postCatalog(nameType string) model.Schemas {
	return model.Schemas{
		"Post": {
			"":       {Type: "object"},
			"title":  {Type: "string"},
			"author": {Ref: "User"},
		},
		"User": {
			"":        {Type: "object"},
			"name":    {Type: nameType},
			"posts":   {Type: "array"},
			"posts[]": {Ref: "Post"},
		},
	}
}

func TestRecursiveSchemas_IdenticalOnBothSides_AreCompatible(t *testing.T) {
	cases := []struct {
		name    string
		root    string
		catalog func() model.Schemas
	}{
		{"a comment replying to comments", "Comment", func() model.Schemas { return commentCatalog("string") }},
		{"a post and its author", "Post", func() model.Schemas { return postCatalog("string") }},
		{"a ref to itself", "Loop", func() model.Schemas { return model.Schemas{"Loop": {"": {Ref: "Loop"}}} }},
		{"an array of itself", "Nested", func() model.Schemas {
			return model.Schemas{"Nested": {"": {Type: "array"}, "[]": {Ref: "Nested"}}}
		}},
	}

	for _, tc := range cases {
		for _, interaction := range []string{"201", "request"} {
			t.Run(tc.name+" in "+interaction, func(t *testing.T) {
				assert.Empty(t, ordersBreaks(t, interaction, ordersBody(tc.root, tc.catalog()), ordersBody(tc.root, tc.catalog())))
			})
		}
	}
}

func TestRecursiveSchemas_OneDifferenceInside_BreaksOnce(t *testing.T) {
	t.Run("a comment replying to comments", func(t *testing.T) {
		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.text", map[string]string{
				"consumerField": "Comment.text", "providerField": "Comment.text", "consumerPropertyType": "string", "providerPropertyType": "integer",
			})},
		}, ordersBreaks(t, "201", ordersBody("Comment", commentCatalog("string")), ordersBody("Comment", commentCatalog("integer"))))
	})

	t.Run("a post and its author", func(t *testing.T) {
		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyTypeMismatch, ordersDetails("$.author.name", map[string]string{
				"consumerField": "User.name", "providerField": "User.name", "consumerPropertyType": "string", "providerPropertyType": "integer",
			})},
		}, ordersBreaks(t, "201", ordersBody("Post", postCatalog("string")), ordersBody("Post", postCatalog("integer"))))
	})
}

func TestRecursiveArray_RendersItsTypeWithoutLooping(t *testing.T) {
	readByTheConsumer := model.Schemas{
		"Order":  {"": {Type: "object"}, "nested": {Ref: "Nested"}},
		"Nested": {"": {Type: "array"}, "[]": {Ref: "Nested"}},
	}

	assert.Equal(t, []ordersBreak{
		{compatibility_checker.ReasonPropertyMissingInProvider, ordersDetails("$.nested", map[string]string{
			"consumerField": "Order.nested", "propertyType": "array<array>",
		})},
	}, ordersBreaks(t, "201", ordersBody("Order", readByTheConsumer), ordersBody("Order", model.Schemas{"Order": {"": {Type: "object"}}})))
}

func TestUndescribedBody_BreaksOnlyWhereTheReaderNeedsABody(t *testing.T) {
	order := model.Schemas{"Order": {"": {Type: "object"}, "id": {Type: "string"}}}

	t.Run("provider undescribed, consumer reads a schema", func(t *testing.T) {
		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyMissingInProvider, ordersDetails("$", map[string]string{"propertyType": "object", "consumerField": "Order"})},
		}, ordersBreaks(t, "201", ordersBody("Order", order), ordersUndescribedBody()))
	})

	t.Run("consumer undescribed, provider provides a schema", func(t *testing.T) {
		assert.Empty(t, ordersBreaks(t, "201", ordersUndescribedBody(), ordersBody("Order", order)))
	})

	t.Run("undescribed on both sides", func(t *testing.T) {
		assert.Empty(t, ordersBreaks(t, "201", ordersUndescribedBody(), ordersUndescribedBody()))
		assert.Empty(t, ordersBreaks(t, "request", ordersUndescribedBody(), ordersUndescribedBody()))
	})

	t.Run("provider accepts an undescribed request", func(t *testing.T) {
		assert.Empty(t, ordersBreaks(t, "request", ordersBody("Order", order), ordersUndescribedBody()))
	})

	t.Run("consumer sends an undescribed request the provider reads", func(t *testing.T) {
		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyMissingInConsumer, ordersDetails("$", map[string]string{"propertyType": "object", "providerField": "Order"})},
		}, ordersBreaks(t, "request", ordersUndescribedBody(), ordersBody("Order", order)))
	})

	t.Run("one consumer module sends an undescribed request", func(t *testing.T) {
		oneUndescribed := ordersSide{
			bodies: []model.ResourceBody{
				{Source: "a.yaml"},
				{Source: "b.yaml", SchemaName: null.StringFrom("Order")},
			},
			schemas: order,
		}

		assert.Equal(t, []ordersBreak{
			{compatibility_checker.ReasonPropertyOptionalInConsumerRequiredInProvider, ordersDetails("$", map[string]string{
				"propertyType": "object", "consumerField": "Order", "providerField": "Order",
			})},
		}, ordersBreaks(t, "request", oneUndescribed, ordersBody("Order", order)))
	})
}

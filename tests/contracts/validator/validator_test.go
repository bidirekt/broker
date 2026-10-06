package validator_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/validator"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

const everyRuleYAMLA = `provides:
  rest:
    /dup:
      get:
        responses:
          200: Pet
    /dup/:
      get:
        responses:
          200: Pet
    /pets:
      get:
        responses:
          200: Missing
consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Invoice
schemas:
  Invoice:
    type: object
    properties:
      id:
        type: string
  Loop:
    ref: Loop
  Pet:
    type: object
    properties:
      owner:
        ref: Ghost
`

const everyRuleYAMLB = `provides:
  rest:
    /pets/:
      get:
        responses:
          200: Pet
consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Charge
schemas:
  Charge:
    type: object
    properties:
      id:
        type: integer
  Pet:
    type: object
    properties:
      id:
        type: string
`

const petsProviderYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
`

const petSchemaYAML = `schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const stringIDConsumerYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceString
schemas:
  InvoiceString:
    type: object
    properties:
      id:
        type: string
`

const integerIDConsumerYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceInteger
schemas:
  InvoiceInteger:
    type: object
    properties:
      id:
        type: integer
`

func validatorFragment(t *testing.T, source, raw string) contract.Fragment {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(raw), &document))

	return contract.Fragment{Source: source, Document: document}
}

func validateFragments(fragments ...contract.Fragment) []violation.Violation {
	return validator.Validate(fragmentmapper.ToDeclarations(fragments))
}

func TestValidator_NoFragments_ReportsNothing(t *testing.T) {
	assert.Empty(t, validateFragments())
}

func TestValidator_EveryRule_FiresAtItsLocationSortedBySourceThenPath(t *testing.T) {
	violations := validateFragments(
		validatorFragment(t, "b.yaml", everyRuleYAMLB),
		validatorFragment(t, "a.yaml", everyRuleYAMLA),
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/dup;get;responses;200",
			Source:    "a.yaml",
			Details:   map[string]string{"resource": "provides GET /dup 200", "declaredIn": "a.yaml"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "a.yaml",
			Details:   map[string]string{"schema": "Missing", "resource": "provides GET /pets 200"},
		},
		{
			ErrorCode: "schema.too_deep",
			Path:      "schemas;Loop",
			Source:    "a.yaml",
			Details:   map[string]string{"schema": "Loop", "maxDepth": "10"},
		},
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Pet;properties;owner",
			Source:    "a.yaml",
			Details:   map[string]string{"schema": "Ghost", "property": "Pet.owner"},
		},
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "b.yaml",
			Details: map[string]string{
				"resource":     "consumes payments GET /invoices 200",
				"property":     "$.id",
				"type":         "integer",
				"declaredIn":   "a.yaml",
				"declaredType": "string",
			},
		},
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "b.yaml",
			Details:   map[string]string{"resource": "provides GET /pets 200", "declaredIn": "a.yaml"},
		},
		{
			ErrorCode: "schema.duplicate",
			Path:      "schemas;Pet",
			Source:    "b.yaml",
			Details:   map[string]string{"schema": "Pet", "declaredIn": "a.yaml"},
		},
	}, violations)
}

func TestValidator_FragmentOrder_DoesNotChangeTheReport(t *testing.T) {
	fragments := []contract.Fragment{
		validatorFragment(t, "a.yaml", petsProviderYAML),
		validatorFragment(t, "b.yaml", petsProviderYAML),
		validatorFragment(t, "c.yaml", petSchemaYAML),
		validatorFragment(t, "d.yaml", petSchemaYAML),
		validatorFragment(t, "e.yaml", stringIDConsumerYAML),
		validatorFragment(t, "f.yaml", integerIDConsumerYAML),
	}

	reversed := make([]contract.Fragment, 0, len(fragments))
	for index := len(fragments) - 1; index >= 0; index-- {
		reversed = append(reversed, fragments[index])
	}

	expected := []violation.Violation{
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "b.yaml",
			Details:   map[string]string{"resource": "provides GET /pets 200", "declaredIn": "a.yaml"},
		},
		{
			ErrorCode: "schema.duplicate",
			Path:      "schemas;Pet",
			Source:    "d.yaml",
			Details:   map[string]string{"schema": "Pet", "declaredIn": "c.yaml"},
		},
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "f.yaml",
			Details: map[string]string{
				"resource":     "consumes payments GET /invoices 200",
				"property":     "$.id",
				"type":         "integer",
				"declaredIn":   "e.yaml",
				"declaredType": "string",
			},
		},
	}

	assert.Equal(t, expected, validateFragments(fragments...))
	assert.Equal(t, expected, validateFragments(reversed...))
}

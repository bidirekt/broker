package validator_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/validator"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

const validEndpointsYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
      post:
        request: Pet
        responses:
          201: Pet
consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Invoice
`

const validSchemasYAML = `schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
  Invoice:
    type: object
    properties:
      total:
        type: integer
`

const unresolvedNamesYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Missing
      post:
        request: AlsoMissing
consumes:
  payments:
    rest:
      /invoices:
        put:
          request: Gone
          responses:
            200: Vanished
`

const unreachedRefYAML = `schemas:
  Invoice:
    type: object
    properties:
      payment:
        ref: Payment
      lines:
        type: array
        items:
          ref: Line
`

const refBehindRefYAML = `schemas:
  Pets:
    type: array
    items:
      ref: Pet
  Pet:
    type: object
    properties:
      owner:
        ref: Ghost
`

const cyclicSchemasYAML = `schemas:
  Pet:
    type: object
    properties:
      owner:
        ref: Owner
  Owner:
    type: object
    properties:
      pet:
        ref: Pet
`

const petsResponseYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
`

const petsRequestYAML = `provides:
  rest:
    /pets:
      post:
        request: Pet
        responses:
          201: Pet
`

const invoicesConsumerYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Invoice
`

const invoiceStringConsumerYAML = `consumes:
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

const invoiceIntegerConsumerYAML = `consumes:
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

type validatedFile struct {
	source string
	raw    string
}

func validateFiles(t *testing.T, files ...validatedFile) []violation.Violation {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		var document any
		require.NoError(t, yaml.Unmarshal([]byte(file.raw), &document))

		fragments = append(fragments, contract.Fragment{Source: file.source, Document: document})
	}

	return validator.Validate(fragmentmapper.ToDeclarations(fragments))
}

func nestedPetYAML(levels int) string {
	var source strings.Builder

	source.WriteString("schemas:\n  Pet:\n")
	indent := "    "

	for level := 1; level <= levels; level++ {
		source.WriteString(indent + "type: object\n")
		source.WriteString(indent + "properties:\n")
		indent += "  "
		fmt.Fprintf(&source, "%sl%d:\n", indent, level)
		indent += "  "
	}

	source.WriteString(indent + "type: string\n")

	return source.String()
}

func TestValidate_ValidContractAcrossFragments_ReportsNothing(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"endpoints.yaml", validEndpointsYAML},
		validatedFile{"schemas.yaml", validSchemasYAML},
	)

	assert.Empty(t, violations)
}

func TestValidate_UnresolvedSchemaNames_NameTheResourceThatCitesThem(t *testing.T) {
	violations := validateFiles(t, validatedFile{"api.yaml", unresolvedNamesYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "consumes;payments;rest;/invoices;put;request",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "Gone", "resource": "consumes payments PUT /invoices request"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "consumes;payments;rest;/invoices;put;responses;200",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "Vanished", "resource": "consumes payments PUT /invoices 200"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "Missing", "resource": "provides GET /pets 200"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;post;request",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "AlsoMissing", "resource": "provides POST /pets request"},
		},
	}, violations)
}

func TestValidate_UnresolvedRefInNeverReferencedSchema_ReportsEachRefSite(t *testing.T) {
	violations := validateFiles(t, validatedFile{"billing.yaml", unreachedRefYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Invoice;properties;lines;items",
			Source:    "billing.yaml",
			Details:   map[string]string{"schema": "Line", "property": "Invoice.lines[]"},
		},
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Invoice;properties;payment",
			Source:    "billing.yaml",
			Details:   map[string]string{"schema": "Payment", "property": "Invoice.payment"},
		},
	}, violations)
}

func TestValidate_UnresolvedRefBehindAResolvedRef_ReportedOnceWhereWritten(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", refBehindRefYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Pet;properties;owner",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Ghost", "property": "Pet.owner"},
		},
	}, violations)
}

func TestValidate_CyclicSchemas_ReportOneTooDeepPerRoot(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", cyclicSchemasYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.too_deep",
			Path:      "schemas;Owner",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Owner", "maxDepth": "10"},
		},
		{
			ErrorCode: "schema.too_deep",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Pet", "maxDepth": "10"},
		},
	}, violations)
}

func TestValidate_NestingAtMaxDepth_ReportsNothing(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", nestedPetYAML(9)})

	assert.Empty(t, violations)
}

func TestValidate_NestingPastMaxDepth_ReportsTooDeep(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", nestedPetYAML(10)})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.too_deep",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Pet", "maxDepth": "10"},
		},
	}, violations)
}

func TestValidate_DuplicateSchema_NamesBothSources(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"schemas.yaml", validSchemasYAML},
		validatedFile{"billing.yaml", validSchemasYAML},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.duplicate",
			Path:      "schemas;Invoice",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Invoice", "declaredIn": "billing.yaml"},
		},
		{
			ErrorCode: "schema.duplicate",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Pet", "declaredIn": "billing.yaml"},
		},
	}, violations)
}

func TestValidate_DuplicateProvidedResources_NameTheResourceAndBothSources(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"a.yaml", petsResponseYAML},
		validatedFile{"b.yaml", petsResponseYAML},
		validatedFile{"c.yaml", petsRequestYAML},
		validatedFile{"d.yaml", petsRequestYAML},
		validatedFile{"schemas.yaml", validSchemasYAML},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "b.yaml",
			Details:   map[string]string{"resource": "provides GET /pets 200", "declaredIn": "a.yaml"},
		},
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;post;request",
			Source:    "d.yaml",
			Details:   map[string]string{"resource": "provides POST /pets request", "declaredIn": "c.yaml"},
		},
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;post;responses;201",
			Source:    "d.yaml",
			Details:   map[string]string{"resource": "provides POST /pets 201", "declaredIn": "c.yaml"},
		},
	}, violations)
}

func TestValidate_DuplicateConsumedResource_ReportsNothing(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"e.yaml", invoicesConsumerYAML},
		validatedFile{"f.yaml", invoicesConsumerYAML},
		validatedFile{"schemas.yaml", validSchemasYAML},
	)

	assert.Empty(t, violations)
}

func TestValidate_BothProvidedSpellingsInOneFile_ReportsDuplicateResource(t *testing.T) {
	bothSpellings := `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
    /pets/:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

	violations := validateFiles(t, validatedFile{"pets.yaml", bothSpellings})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "pets.yaml",
			Details:   map[string]string{"resource": "provides GET /pets 200", "declaredIn": "pets.yaml"},
		},
	}, violations)
}

func TestValidate_BothConsumedSpellingsInOneFile_ReportsNothing(t *testing.T) {
	bothSpellings := `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Invoice
      /invoices/:
        get:
          responses:
            200: Invoice
schemas:
  Invoice:
    type: object
    properties:
      total:
        type: integer
`

	violations := validateFiles(t, validatedFile{"invoices.yaml", bothSpellings})

	assert.Empty(t, violations)
}

func TestValidate_ConsumedResourceWithConflictingTypes_NamesBothDeclarations(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"b.yaml", invoiceIntegerConsumerYAML},
		validatedFile{"a.yaml", invoiceStringConsumerYAML},
	)

	assert.Equal(t, []violation.Violation{
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
	}, violations)
}

func TestValidate_ConsumedResourceWithConflictingTypesInOneFile_NamesTheFileTwice(t *testing.T) {
	bothSpellings := `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceString
      /invoices/:
        get:
          responses:
            200: InvoiceInteger
schemas:
  InvoiceString:
    type: object
    properties:
      id:
        type: string
  InvoiceInteger:
    type: object
    properties:
      id:
        type: integer
`

	violations := validateFiles(t, validatedFile{"invoices.yaml", bothSpellings})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "invoices.yaml",
			Details: map[string]string{
				"resource":     "consumes payments GET /invoices 200",
				"property":     "$.id",
				"type":         "integer",
				"declaredIn":   "invoices.yaml",
				"declaredType": "string",
			},
		},
	}, violations)
}

func TestValidate_SeveralConflictingProperties_ReportEachInPropertyPathOrder(t *testing.T) {
	invoice := `consumes:
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
      total:
        type: integer
      id:
        type: string
`

	charge := `consumes:
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
      total:
        type: float
      id:
        type: integer
`

	violations := validateFiles(t,
		validatedFile{"a.yaml", invoice},
		validatedFile{"b.yaml", charge},
	)

	assert.Equal(t, []violation.Violation{
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
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "b.yaml",
			Details: map[string]string{
				"resource":     "consumes payments GET /invoices 200",
				"property":     "$.total",
				"type":         "float",
				"declaredIn":   "a.yaml",
				"declaredType": "integer",
			},
		},
	}, violations)
}

func TestValidate_ConflictAgainstUnresolvedSchema_ReportsOnlyTheUnresolvedName(t *testing.T) {
	ghost := `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: Ghost
`

	violations := validateFiles(t,
		validatedFile{"a.yaml", invoiceStringConsumerYAML},
		validatedFile{"b.yaml", ghost},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "b.yaml",
			Details:   map[string]string{"schema": "Ghost", "resource": "consumes payments GET /invoices 200"},
		},
	}, violations)
}

func TestValidate_TrailingSlashInAnotherFile_CollidesAsDuplicateResource(t *testing.T) {
	slashed := `provides:
  rest:
    /pets/:
      get:
        responses:
          200: Pet
`

	violations := validateFiles(t,
		validatedFile{"a.yaml", petsResponseYAML},
		validatedFile{"b.yaml", slashed},
		validatedFile{"schemas.yaml", validSchemasYAML},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "b.yaml",
			Details:   map[string]string{"resource": "provides GET /pets 200", "declaredIn": "a.yaml"},
		},
	}, violations)
}

func TestValidate_SameInputTwice_ReportsTheSameOrder(t *testing.T) {
	files := []validatedFile{
		{"api.yaml", unresolvedNamesYAML},
		{"billing.yaml", unreachedRefYAML},
		{"schemas.yaml", cyclicSchemasYAML},
	}

	first := validateFiles(t, files...)
	second := validateFiles(t, files...)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "consumes;payments;rest;/invoices;put;request",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "Gone", "resource": "consumes payments PUT /invoices request"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "consumes;payments;rest;/invoices;put;responses;200",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "Vanished", "resource": "consumes payments PUT /invoices 200"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "Missing", "resource": "provides GET /pets 200"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;post;request",
			Source:    "api.yaml",
			Details:   map[string]string{"schema": "AlsoMissing", "resource": "provides POST /pets request"},
		},
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Invoice;properties;lines;items",
			Source:    "billing.yaml",
			Details:   map[string]string{"schema": "Line", "property": "Invoice.lines[]"},
		},
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Invoice;properties;payment",
			Source:    "billing.yaml",
			Details:   map[string]string{"schema": "Payment", "property": "Invoice.payment"},
		},
		{
			ErrorCode: "schema.too_deep",
			Path:      "schemas;Owner",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Owner", "maxDepth": "10"},
		},
		{
			ErrorCode: "schema.too_deep",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Pet", "maxDepth": "10"},
		},
	}, first)
	assert.Equal(t, first, second)
}

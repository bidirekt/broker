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

const unresolvedMapValuesRefYAML = `schemas:
  Prices:
    type: object
    values:
      ref: Price
`

const unresolvedNestedMapValuesRefsYAML = `schemas:
  Catalog:
    type: object
    properties:
      prices:
        type: object
        values:
          ref: Price
  PriceHistory:
    type: object
    values:
      type: array
      items:
        ref: Price
  PriceTables:
    type: array
    items:
      type: object
      values:
        ref: Price
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

const selfReferencingCommentYAML = `schemas:
  Comment:
    type: object
    properties:
      replies:
        type: array
        items:
          ref: Comment
`

const nodeChildrenMapCycleYAML = `schemas:
  Node:
    type: object
    properties:
      children:
        type: object
        values:
          ref: Node
`

const postUserCycleYAML = `schemas:
  Post:
    type: object
    properties:
      author:
        ref: User
  User:
    type: object
    properties:
      posts:
        type: array
        items:
          ref: Post
`

const feedReachingCommentCycleYAML = `schemas:
  Feed:
    type: object
    properties:
      comment:
        ref: Comment
  Comment:
    type: object
    properties:
      reply:
        ref: Reply
  Reply:
    type: object
    properties:
      parent:
        ref: Comment
`

const loopYAML = `schemas:
  Loop:
    ref: Loop
`

const longRefChainYAML = `schemas:
  Feed:
    type: object
    properties:
      posts:
        type: array
        items:
          ref: Post
  Post:
    type: object
    properties:
      author:
        ref: Author
  Author:
    type: object
    properties:
      details:
        ref: Details
  Details:
    type: object
    properties:
      avatars:
        type: array
        items:
          ref: Avatar
  Avatar:
    type: object
    properties:
      url:
        type: string
`

const siblingBranchesYAML = `schemas:
  Post:
    type: object
    properties:
      author:
        ref: User
      editor:
        ref: User
  User:
    type: object
    properties:
      name:
        type: string
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

func deepInlinePetYAML(levels int) string {
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

func TestValidate_UnresolvedRefUnderMapValues_ReportedAtTheValues(t *testing.T) {
	violations := validateFiles(t, validatedFile{"prices.yaml", unresolvedMapValuesRefYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Prices;values",
			Source:    "prices.yaml",
			Details:   map[string]string{"schema": "Price", "property": "Prices{}"},
		},
	}, violations)
}

func TestValidate_UnresolvedRefsUnderNestedMapValues_ReportEachRefSite(t *testing.T) {
	violations := validateFiles(t, validatedFile{"prices.yaml", unresolvedNestedMapValuesRefsYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;Catalog;properties;prices;values",
			Source:    "prices.yaml",
			Details:   map[string]string{"schema": "Price", "property": "Catalog.prices{}"},
		},
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;PriceHistory;values;items",
			Source:    "prices.yaml",
			Details:   map[string]string{"schema": "Price", "property": "PriceHistory{}[]"},
		},
		{
			ErrorCode: "schema.unresolved_ref",
			Path:      "schemas;PriceTables;items;values",
			Source:    "prices.yaml",
			Details:   map[string]string{"schema": "Price", "property": "PriceTables[]{}"},
		},
	}, violations)
}

func TestValidate_RecursiveSchemas_ReportNothing(t *testing.T) {
	cases := map[string]string{
		"two schemas pointing at each other": cyclicSchemasYAML,
		"a comment replying to comments":     selfReferencingCommentYAML,
		"a node whose map values are nodes":  nodeChildrenMapCycleYAML,
		"a post and its author":              postUserCycleYAML,
		"a schema reaching a recursive pair": feedReachingCommentCycleYAML,
		"a ref to itself":                    loopYAML,
	}

	for name, schemas := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, validateFiles(t, validatedFile{"schemas.yaml", schemas}))
		})
	}
}

func TestValidate_LongRefChainWithoutLoop_ReportsNothing(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", longRefChainYAML})

	assert.Empty(t, violations)
}

func TestValidate_SameSchemaOnSiblingBranches_ReportsNothing(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", siblingBranchesYAML})

	assert.Empty(t, violations)
}

func TestValidate_DeepInlineNesting_ReportsNothing(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", deepInlinePetYAML(50)})

	assert.Empty(t, violations)
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
	}, first)
	assert.Equal(t, first, second)
}

const invoiceNumberConsumerYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceNumber
schemas:
  InvoiceNumber:
    type: object
    properties:
      id:
        type: number
`

func TestValidate_NumberProperty_ReportsNothing(t *testing.T) {
	violations := validateFiles(t, validatedFile{"invoices.yaml", invoiceNumberConsumerYAML})

	assert.Empty(t, violations)
}

func TestValidate_ConsumedResourceWithNumberAndIntegerTypes_IsATypeConflict(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"b.yaml", invoiceNumberConsumerYAML},
		validatedFile{"a.yaml", invoiceIntegerConsumerYAML},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "b.yaml",
			Details: map[string]string{
				"resource":     "consumes payments GET /invoices 200",
				"property":     "$.id",
				"type":         "number",
				"declaredIn":   "a.yaml",
				"declaredType": "integer",
			},
		},
	}, violations)
}

const undescribedBodiesYAML = `provides:
  rest:
    /files/*:
      get:
        responses:
          200: ~
      delete:
        responses:
          204:
    /files:
      post:
        request: ~
consumes:
  storage:
    rest:
      /files:
        post:
          request: ~
          responses:
            201:
`

const emptySchemaNamesYAML = `provides:
  rest:
    /files:
      post:
        request: ""
        responses:
          204: ""
`

const undescribedUploadYAML = `provides:
  rest:
    /files:
      post:
        request: ~
`

func TestValidate_UndescribedBodies_ReportNothing(t *testing.T) {
	violations := validateFiles(t, validatedFile{"files.yaml", undescribedBodiesYAML})

	assert.Empty(t, violations)
}

func TestValidate_EmptySchemaNames_ReportUnresolvedName(t *testing.T) {
	violations := validateFiles(t, validatedFile{"files.yaml", emptySchemaNamesYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/files;post;request",
			Source:    "files.yaml",
			Details:   map[string]string{"schema": "", "resource": "provides POST /files request"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/files;post;responses;204",
			Source:    "files.yaml",
			Details:   map[string]string{"schema": "", "resource": "provides POST /files 204"},
		},
	}, violations)
}

func TestValidate_UndescribedProvidedRequestDeclaredTwice_ReportsDuplicateResource(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"a.yaml", undescribedUploadYAML},
		validatedFile{"b.yaml", undescribedUploadYAML},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.duplicate",
			Path:      "provides;rest;/files;post;request",
			Source:    "b.yaml",
			Details:   map[string]string{"resource": "provides POST /files request", "declaredIn": "a.yaml"},
		},
	}, violations)
}

const orderWithStringCitiesYAML = `consumes:
  shop:
    rest:
      /orders:
        get:
          responses:
            200: OrderRead
schemas:
  OrderRead:
    type: object
    properties:
      billing: { ref: AddressRead }
      shipping: { ref: AddressRead }
      history:
        type: array
        items: { ref: AddressRead }
  AddressRead:
    type: object
    properties:
      city: { type: string }
`

const orderWithIntegerCitiesYAML = `consumes:
  shop:
    rest:
      /orders:
        get:
          responses:
            200: OrderSummary
schemas:
  OrderSummary:
    type: object
    properties:
      billing: { ref: AddressSummary }
      shipping: { ref: AddressSummary }
  AddressSummary:
    type: object
    properties:
      city: { type: integer }
`

func TestValidate_ConflictInsideASchemaUsedInManyPlaces_ReportedOnceAtItsFirstPath(t *testing.T) {
	violations := validateFiles(t,
		validatedFile{"a.yaml", orderWithStringCitiesYAML},
		validatedFile{"b.yaml", orderWithIntegerCitiesYAML},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;shop;rest;/orders;get;responses;200",
			Source:    "b.yaml",
			Details: map[string]string{
				"resource":     "consumes shop GET /orders 200",
				"property":     "$.billing.city",
				"type":         "integer",
				"declaredIn":   "a.yaml",
				"declaredType": "string",
			},
		},
	}, violations)
}

func TestValidate_ConflictingCollections_ReportedOnTheCollectionOnly(t *testing.T) {
	listOfLines := `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceLines
schemas:
  InvoiceLines:
    type: object
    properties:
      lines:
        type: array
        items:
          type: object
          properties:
            total: { type: integer }
`
	mapOfLines := `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceLineMap
schemas:
  InvoiceLineMap:
    type: object
    properties:
      lines:
        type: object
        properties:
          total: { type: string }
`

	violations := validateFiles(t,
		validatedFile{"a.yaml", listOfLines},
		validatedFile{"b.yaml", mapOfLines},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;payments;rest;/invoices;get;responses;200",
			Source:    "b.yaml",
			Details: map[string]string{
				"resource":     "consumes payments GET /invoices 200",
				"property":     "$.lines",
				"type":         "object",
				"declaredIn":   "a.yaml",
				"declaredType": "array",
			},
		},
	}, violations)
}

func TestValidate_ConflictInsideRecursiveSchemas_EndsAndIsReportedOnce(t *testing.T) {
	stringText := `consumes:
  blog:
    rest:
      /comments:
        get:
          responses:
            200: Comment
schemas:
  Comment:
    type: object
    properties:
      text: { type: string }
      replies:
        type: array
        items: { ref: Comment }
`
	integerText := `consumes:
  blog:
    rest:
      /comments:
        get:
          responses:
            200: Reply
schemas:
  Reply:
    type: object
    properties:
      text: { type: integer }
      replies:
        type: array
        items: { ref: Reply }
`

	var conflicts []violation.Violation
	for _, found := range validateFiles(t, validatedFile{"a.yaml", stringText}, validatedFile{"b.yaml", integerText}) {
		if found.ErrorCode == "resource.type_conflict" {
			conflicts = append(conflicts, found)
		}
	}

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "resource.type_conflict",
			Path:      "consumes;blog;rest;/comments;get;responses;200",
			Source:    "b.yaml",
			Details: map[string]string{
				"resource":     "consumes blog GET /comments 200",
				"property":     "$.text",
				"type":         "integer",
				"declaredIn":   "a.yaml",
				"declaredType": "string",
			},
		},
	}, conflicts)
}

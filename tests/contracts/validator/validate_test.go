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

func TestValidate_CyclicSchemas_ReportOneCyclePerSchemaInTheCycle(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", cyclicSchemasYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Owner",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Owner", "cycle": "Owner → Pet → Owner"},
		},
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Pet", "cycle": "Pet → Owner → Pet"},
		},
	}, violations)
}

func TestValidate_SelfReferenceThroughArray_ReportsCycle(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", selfReferencingCommentYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Comment",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Comment", "cycle": "Comment → Comment"},
		},
	}, violations)
}

func TestValidate_SelfReferenceThroughMapValues_ReportsCycle(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", nodeChildrenMapCycleYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Node",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Node", "cycle": "Node → Node"},
		},
	}, violations)
}

func TestValidate_CycleThroughArrayBackToTheParent_ReportsBothSchemas(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", postUserCycleYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Post",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Post", "cycle": "Post → User → Post"},
		},
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;User",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "User", "cycle": "User → Post → User"},
		},
	}, violations)
}

func TestValidate_SchemaThatOnlyReachesACycle_IsNotMarked(t *testing.T) {
	violations := validateFiles(t, validatedFile{"schemas.yaml", feedReachingCommentCycleYAML})

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Comment",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Comment", "cycle": "Comment → Reply → Comment"},
		},
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Reply",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Reply", "cycle": "Reply → Comment → Reply"},
		},
	}, violations)
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
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Owner",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Owner", "cycle": "Owner → Pet → Owner"},
		},
		{
			ErrorCode: "schema.cycle",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]string{"schema": "Pet", "cycle": "Pet → Owner → Pet"},
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

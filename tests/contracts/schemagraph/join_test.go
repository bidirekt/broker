package schemagraph_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/schemagraph"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const invoiceListModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceList
schemas:
  InvoiceList:
    type: object
    properties:
      id:   { type: string }
      name: { type: string }
`

const invoiceDetailModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceDetail
schemas:
  InvoiceDetail:
    type: object
    properties:
      id:         { type: string }
      created_at: { type: string }
`

const invoiceCreateModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        post:
          request: InvoiceCreate
          responses:
            201: InvoiceCreate
schemas:
  InvoiceCreate:
    type: object
    properties:
      id:   { type: string }
      name: { type: string }
`

const invoiceImportModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        post:
          request: InvoiceImport
          responses:
            201: InvoiceImport
schemas:
  InvoiceImport:
    type: object
    properties:
      id:    { type: string }
      email: { type: string }
`

const optionalTotalReaderYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceOptional
schemas:
  InvoiceOptional:
    type: object
    properties:
      total:  { type: integer, optional: true }
      status: { type: string, optional: true }
`

const requiredTotalReaderYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceRequired
schemas:
  InvoiceRequired:
    type: object
    properties:
      total: { type: integer }
`

const otherOptionalTotalReaderYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceOtherOptional
schemas:
  InvoiceOtherOptional:
    type: object
    properties:
      total: { type: integer, optional: true }
`

const optionalNoteSenderYAML = `consumes:
  payments:
    rest:
      /invoices:
        put:
          request: PutOptional
schemas:
  PutOptional:
    type: object
    properties:
      id:   { type: string }
      note: { type: string, optional: true }
`

const requiredNoteSenderYAML = `consumes:
  payments:
    rest:
      /invoices:
        put:
          request: PutRequired
schemas:
  PutRequired:
    type: object
    properties:
      id:   { type: string }
      note: { type: string }
`

const groupWithAddressYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: GroupWithAddress
          responses:
            201: GroupWithAddress
schemas:
  GroupWithAddress:
    type: object
    properties:
      name: { type: string }
      address:
        type: object
        properties:
          city: { type: string }
`

const groupNameYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: GroupName
          responses:
            201: GroupName
schemas:
  GroupName:
    type: object
    properties:
      name: { type: string }
`

const groupWithCityAndStateYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: GroupWithCityAndState
schemas:
  GroupWithCityAndState:
    type: object
    properties:
      name: { type: string }
      address:
        type: object
        properties:
          city:  { type: string }
          state: { type: string }
`

const groupUndescribedBodyYAML = `consumes:
  api:
    rest:
      /api/groups:
        post:
          request: ~
          responses:
            201: ~
`

const profileNullableModuleYAML = `consumes:
  api:
    rest:
      /api/profiles:
        post:
          request: ProfileNullable
          responses:
            201: ProfileNullable
schemas:
  ProfileNullable:
    type: object
    properties:
      avatar:   { type: string, nullable: true }
      bio:      { type: string, nullable: true }
      nickname: { type: string, nullable: true }
`

const profileNotNullableModuleYAML = `consumes:
  api:
    rest:
      /api/profiles:
        post:
          request: ProfileNotNullable
          responses:
            201: ProfileNotNullable
schemas:
  ProfileNotNullable:
    type: object
    properties:
      avatar: { type: string, nullable: true }
      bio:    { type: string }
      tag:    { type: string }
`

type joinedField struct {
	Type     string
	Optional bool
	Nullable bool
}

type joinedFile struct {
	source  string
	content string
}

type joinOrder struct {
	name  string
	files []joinedFile
}

func bothJoinOrders(first, last string) []joinOrder {
	return []joinOrder{
		{"given order", []joinedFile{{"a.yaml", first}, {"b.yaml", last}}},
		{"reversed order", []joinedFile{{"a.yaml", last}, {"b.yaml", first}}},
	}
}

func joinedRoot(t *testing.T, interaction model.Interaction, files ...joinedFile) schemagraph.Node {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		fragment, err := contract.DecodeFragment(file.source, file.content)
		require.NoError(t, err)

		fragments = append(fragments, fragment)
	}

	declarations := fragmentmapper.ToDeclarations(fragments)
	graph := schemagraph.New(fragmentmapper.ToSchemaModels(declarations))

	join := schemagraph.Readers
	if interaction == model.RestRequest {
		join = schemagraph.Senders
	}

	for _, resource := range fragmentmapper.ToResourceModels(declarations) {
		if resource.Interaction == interaction {
			return graph.Root(resource.Bodies, join)
		}
	}

	t.Fatalf("no %v resource was built", interaction)

	return schemagraph.Node{}
}

func joinedFields(node schemagraph.Node) map[string]joinedField {
	fields := map[string]joinedField{}
	collectJoinedFields(node, "$", fields)

	return fields
}

func collectJoinedFields(node schemagraph.Node, path string, fields map[string]joinedField) {
	if !node.Exists() {
		return
	}

	fields[path] = joinedField{Type: node.Type(), Optional: node.Optional(), Nullable: node.Nullable()}

	for _, segment := range node.Segments() {
		collectJoinedFields(node.Child(segment), schemagraph.ChildPath(path, segment), fields)
	}
}

func TestJoin_TwoReadersOfOneResponse_UnionOfTheirFields(t *testing.T) {
	response := joinedRoot(t, model.RestResponse,
		joinedFile{"list.yaml", invoiceListModuleYAML},
		joinedFile{"detail.yaml", invoiceDetailModuleYAML},
	)

	assert.Equal(t, map[string]joinedField{
		"$":            {Type: "object"},
		"$.id":         {Type: "string"},
		"$.name":       {Type: "string"},
		"$.created_at": {Type: "string"},
	}, joinedFields(response))
}

func TestJoin_TwoSendersOfOneRequest_FieldSentByOneIsOptional(t *testing.T) {
	request := joinedRoot(t, model.RestRequest,
		joinedFile{"create.yaml", invoiceCreateModuleYAML},
		joinedFile{"import.yaml", invoiceImportModuleYAML},
	)

	assert.Equal(t, map[string]joinedField{
		"$":       {Type: "object"},
		"$.id":    {Type: "string"},
		"$.name":  {Type: "string", Optional: true},
		"$.email": {Type: "string", Optional: true},
	}, joinedFields(request))
}

func TestJoin_ResponseRequiredByOneReader_StaysRequired(t *testing.T) {
	response := joinedFields(joinedRoot(t, model.RestResponse,
		joinedFile{"optional.yaml", optionalTotalReaderYAML},
		joinedFile{"required.yaml", requiredTotalReaderYAML},
	))

	assert.False(t, response["$.total"].Optional)
	assert.True(t, response["$.status"].Optional)
}

func TestJoin_ResponseOptionalForEveryReader_StaysOptional(t *testing.T) {
	response := joinedFields(joinedRoot(t, model.RestResponse,
		joinedFile{"optional.yaml", optionalTotalReaderYAML},
		joinedFile{"other_optional.yaml", otherOptionalTotalReaderYAML},
	))

	assert.True(t, response["$.total"].Optional)
	assert.True(t, response["$.status"].Optional)
}

func TestJoin_RequestOptionalForOneSender_IsOptionalForAll(t *testing.T) {
	request := joinedFields(joinedRoot(t, model.RestRequest,
		joinedFile{"optional.yaml", optionalNoteSenderYAML},
		joinedFile{"required.yaml", requiredNoteSenderYAML},
	))

	assert.False(t, request["$.id"].Optional)
	assert.True(t, request["$.note"].Optional)
}

func TestJoin_RequestFieldWhoseParentAnotherSenderOmits_KeepsItsOwnOptional(t *testing.T) {
	for _, order := range bothJoinOrders(groupWithAddressYAML, groupNameYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":              {Type: "object"},
				"$.name":         {Type: "string"},
				"$.address":      {Type: "object", Optional: true},
				"$.address.city": {Type: "string"},
			}, joinedFields(joinedRoot(t, model.RestRequest, order.files...)))
		})
	}
}

func TestJoin_RequestFieldWhoseParentAnotherSenderSends_IsOptional(t *testing.T) {
	for _, order := range bothJoinOrders(groupWithCityAndStateYAML, groupWithAddressYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":               {Type: "object"},
				"$.name":          {Type: "string"},
				"$.address":       {Type: "object"},
				"$.address.city":  {Type: "string"},
				"$.address.state": {Type: "string", Optional: true},
			}, joinedFields(joinedRoot(t, model.RestRequest, order.files...)))
		})
	}
}

func TestJoin_ResponseFieldOfOneReader_KeepsItsDeclaration(t *testing.T) {
	for _, order := range bothJoinOrders(groupWithAddressYAML, groupNameYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":              {Type: "object"},
				"$.name":         {Type: "string"},
				"$.address":      {Type: "object"},
				"$.address.city": {Type: "string"},
			}, joinedFields(joinedRoot(t, model.RestResponse, order.files...)))
		})
	}
}

func TestJoin_RequestUndescribedByOneSender_MakesTheBodyOptional(t *testing.T) {
	for _, order := range bothJoinOrders(groupUndescribedBodyYAML, groupNameYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":      {Type: "object", Optional: true},
				"$.name": {Type: "string"},
			}, joinedFields(joinedRoot(t, model.RestRequest, order.files...)))
		})
	}
}

func TestJoin_ResponseUndescribedByOneReader_KeepsTheOtherReadersDeclaration(t *testing.T) {
	for _, order := range bothJoinOrders(groupUndescribedBodyYAML, groupNameYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":      {Type: "object"},
				"$.name": {Type: "string"},
			}, joinedFields(joinedRoot(t, model.RestResponse, order.files...)))
		})
	}
}

func TestJoin_UndescribedBodyInEveryModule_HasNoBody(t *testing.T) {
	files := []joinedFile{{"a.yaml", groupUndescribedBodyYAML}, {"b.yaml", groupUndescribedBodyYAML}}

	assert.False(t, joinedRoot(t, model.RestRequest, files...).Exists())
	assert.False(t, joinedRoot(t, model.RestResponse, files...).Exists())
}

func TestJoin_IdenticalDeclarationInTwoFiles_ReadsLikeOne(t *testing.T) {
	twice := joinedRoot(t, model.RestResponse,
		joinedFile{"a.yaml", invoiceListModuleYAML},
		joinedFile{"b.yaml", invoiceListModuleYAML},
	)
	once := joinedRoot(t, model.RestResponse, joinedFile{"a.yaml", invoiceListModuleYAML})

	assert.Equal(t, joinedFields(once), joinedFields(twice))
}

func TestJoin_FragmentOrderReversed_JoinsTheSameFields(t *testing.T) {
	forward := []joinedFile{
		{"list.yaml", invoiceListModuleYAML},
		{"detail.yaml", invoiceDetailModuleYAML},
		{"create.yaml", invoiceCreateModuleYAML},
		{"import.yaml", invoiceImportModuleYAML},
	}
	backward := []joinedFile{forward[3], forward[2], forward[1], forward[0]}

	for _, interaction := range []model.Interaction{model.RestRequest, model.RestResponse} {
		assert.Equal(t,
			joinedFields(joinedRoot(t, interaction, forward...)),
			joinedFields(joinedRoot(t, interaction, backward...)),
		)
	}
}

func TestJoin_RequestNullableForOneSender_IsNullable(t *testing.T) {
	for _, order := range bothJoinOrders(profileNullableModuleYAML, profileNotNullableModuleYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":          {Type: "object"},
				"$.avatar":   {Type: "string", Nullable: true},
				"$.bio":      {Type: "string", Nullable: true},
				"$.nickname": {Type: "string", Optional: true, Nullable: true},
				"$.tag":      {Type: "string", Optional: true},
			}, joinedFields(joinedRoot(t, model.RestRequest, order.files...)))
		})
	}
}

func TestJoin_ResponseNullableOnlyWhenEveryReaderAllowsNull(t *testing.T) {
	for _, order := range bothJoinOrders(profileNullableModuleYAML, profileNotNullableModuleYAML) {
		t.Run(order.name, func(t *testing.T) {
			assert.Equal(t, map[string]joinedField{
				"$":          {Type: "object"},
				"$.avatar":   {Type: "string", Nullable: true},
				"$.bio":      {Type: "string"},
				"$.nickname": {Type: "string", Nullable: true},
				"$.tag":      {Type: "string"},
			}, joinedFields(joinedRoot(t, model.RestResponse, order.files...)))
		})
	}
}

func TestJoin_TypeOfAFieldDeclaredDifferently_IsTheFirstBodys(t *testing.T) {
	stringID := `consumes:
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
      id: { type: string }
`
	integerID := `consumes:
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
      id: { type: integer }
`

	response := joinedRoot(t, model.RestResponse, joinedFile{"a.yaml", stringID}, joinedFile{"b.yaml", integerID})

	assert.Equal(t, "string", response.Child("id").Type())
	assert.Equal(t, []schemagraph.Member{
		{Source: "a.yaml", Type: "string"},
		{Source: "b.yaml", Type: "integer"},
	}, response.Child("id").Members())
}

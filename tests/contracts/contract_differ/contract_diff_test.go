package contract_differ_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

const participantName = "pets-service"

func petBody() []model.ResourceBody {
	return []model.ResourceBody{{Source: "pets.yaml", SchemaName: null.StringFrom("Pet")}}
}

func petSchemas() model.Schemas {
	return model.Schemas{
		"Pet": {
			"":   {Type: "object"},
			"id": {Type: "string"},
		},
	}
}

func persistedPets(schemas model.Schemas, resources ...*model.UploadedResource) *model.PersistedContract {
	contract := &model.PersistedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.PersistedResource{},
		Schemas:         schemas,
	}

	for _, resource := range resources {
		resource.ParticipantName = participantName
		contract.Resources[resource.PrimaryHash()] = model.PersistedResource{
			ParticipantName:    participantName,
			Direction:          resource.Direction,
			Interaction:        resource.Interaction,
			ProviderHash:       resource.ProviderHash(),
			Endpoint:           resource.Endpoint,
			Method:             resource.Method,
			ResponseStatusCode: resource.ResponseStatusCode,
			Bodies:             resource.Bodies,
			Schemas:            schemas,
		}
	}

	return contract
}

func uploadedPets(schemas model.Schemas, resources ...*model.UploadedResource) *model.UploadedContract {
	contract := model.NewUploadedContract(0, participantName, "1", "raw")
	contract.Schemas = schemas

	for _, resource := range resources {
		_ = contract.AddResource(resource)
	}

	return contract
}

func listPets() *model.UploadedResource {
	return model.NewRestResponseProvider("/pets", "get", "200", petBody())
}

func getPet() *model.UploadedResource {
	return model.NewRestResponseProvider("/pets/*", "get", "200", petBody())
}

func TestDiff_EquivalentContracts_HaveNoChanges(t *testing.T) {
	diff := contract_differ.DiffContracts(persistedPets(petSchemas(), listPets()), uploadedPets(petSchemas(), listPets()))

	assert.Empty(t, diff.Resources)
	assert.Empty(t, diff.Schemas)
}

func TestDiff_AddedResource_IsAdded(t *testing.T) {
	added := getPet()

	diff := contract_differ.DiffContracts(persistedPets(petSchemas(), listPets()), uploadedPets(petSchemas(), listPets(), added))

	assert.Equal(t, map[string]model.ChangeKind{added.PrimaryHash(): model.ChangeAdded}, diff.Resources)
	assert.Empty(t, diff.Schemas)
}

func TestDiff_RemovedResource_IsRemoved(t *testing.T) {
	removed := getPet()
	current := persistedPets(petSchemas(), listPets(), removed)

	diff := contract_differ.DiffContracts(current, uploadedPets(petSchemas(), listPets()))

	assert.Equal(t, map[string]model.ChangeKind{removed.PrimaryHash(): model.ChangeRemoved}, diff.Resources)
}

func TestDiff_ResourceWhoseBodyChanged_IsNotAResourceChange(t *testing.T) {
	undescribed := model.NewRestResponseProvider("/pets", "get", "200", []model.ResourceBody{{Source: "pets.yaml"}})

	diff := contract_differ.DiffContracts(persistedPets(petSchemas(), listPets()), uploadedPets(model.Schemas{}, undescribed))

	assert.Empty(t, diff.Resources)
	assert.Equal(t, model.ChangeRemoved, diff.Schemas["Pet"].Kind)
}

func TestDiff_NewSchema_IsAddedWithEveryField(t *testing.T) {
	next := petSchemas()
	next["Owner"] = model.Schema{
		"":     {Type: "object"},
		"name": {Type: "string", Optional: true},
	}

	diff := contract_differ.DiffContracts(persistedPets(petSchemas(), listPets()), uploadedPets(next, listPets()))

	assert.Equal(t, map[string]model.SchemaChange{
		"Owner": {Kind: model.ChangeAdded, Fields: map[string]model.FieldChange{
			"":     {Kind: model.ChangeAdded, After: model.Field{Type: "object"}},
			"name": {Kind: model.ChangeAdded, After: model.Field{Type: "string", Optional: true}},
		}},
	}, diff.Schemas)
}

func TestDiff_SchemaNoLongerReachable_IsRemovedWithEveryField(t *testing.T) {
	diff := contract_differ.DiffContracts(persistedPets(petSchemas(), listPets()), uploadedPets(model.Schemas{}))

	assert.Equal(t, map[string]model.SchemaChange{
		"Pet": {Kind: model.ChangeRemoved, Fields: map[string]model.FieldChange{
			"":   {Kind: model.ChangeRemoved, Before: model.Field{Type: "object"}},
			"id": {Kind: model.ChangeRemoved, Before: model.Field{Type: "string"}},
		}},
	}, diff.Schemas)
}

func TestDiff_RenamedSchema_IsRemovedAndAdded(t *testing.T) {
	current := model.Schemas{"Address": {"": {Type: "object"}, "city": {Type: "string"}}}
	next := model.Schemas{"PostalAddress": {"": {Type: "object"}, "city": {Type: "string"}}}

	diff := contract_differ.DiffContracts(persistedPets(current), uploadedPets(next))

	assert.Len(t, diff.Schemas, 2)
	assert.Equal(t, model.ChangeRemoved, diff.Schemas["Address"].Kind)
	assert.Equal(t, model.ChangeAdded, diff.Schemas["PostalAddress"].Kind)
}

func TestDiff_FieldAddedAndRemoved_ModifyTheSchema(t *testing.T) {
	next := model.Schemas{"Pet": {"": {Type: "object"}, "name": {Type: "string"}}}

	diff := contract_differ.DiffContracts(persistedPets(petSchemas()), uploadedPets(next))

	assert.Equal(t, map[string]model.SchemaChange{
		"Pet": {Kind: model.ChangeModified, Fields: map[string]model.FieldChange{
			"id":   {Kind: model.ChangeRemoved, Before: model.Field{Type: "string"}},
			"name": {Kind: model.ChangeAdded, After: model.Field{Type: "string"}},
		}},
	}, diff.Schemas)
}

func TestDiff_FieldWhoseAttributeChanged_IsModified(t *testing.T) {
	cases := []struct {
		name  string
		after model.Field
	}{
		{name: "type", after: model.Field{Type: "integer"}},
		{name: "optional", after: model.Field{Type: "string", Optional: true}},
		{name: "nullable", after: model.Field{Type: "string", Nullable: true}},
		{name: "ref", after: model.Field{Ref: "PetId"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			next := petSchemas()
			next["Pet"]["id"] = testCase.after

			diff := contract_differ.DiffContracts(persistedPets(petSchemas()), uploadedPets(next))

			assert.Equal(t, map[string]model.SchemaChange{
				"Pet": {Kind: model.ChangeModified, Fields: map[string]model.FieldChange{
					"id": {Kind: model.ChangeModified, Before: model.Field{Type: "string"}, After: testCase.after},
				}},
			}, diff.Schemas)
		})
	}
}

func TestDiff_RefTargetRenamed_ModifiesTheRefField(t *testing.T) {
	current := model.Schemas{
		"Order":   {"": {Type: "object"}, "billing": {Ref: "Address"}},
		"Address": {"": {Type: "object"}},
	}
	next := model.Schemas{
		"Order":         {"": {Type: "object"}, "billing": {Ref: "PostalAddress"}},
		"PostalAddress": {"": {Type: "object"}},
	}

	diff := contract_differ.DiffContracts(persistedPets(current), uploadedPets(next))

	assert.Equal(t, model.SchemaChange{Kind: model.ChangeModified, Fields: map[string]model.FieldChange{
		"billing": {Kind: model.ChangeModified, Before: model.Field{Ref: "Address"}, After: model.Field{Ref: "PostalAddress"}},
	}}, diff.Schemas["Order"])
}

func TestRemovedSinceDeployed_ResourceGoneFromTheFiles_Counts(t *testing.T) {
	gone := model.PersistedResource{
		ParticipantName:    participantName,
		Direction:          model.Provides,
		Interaction:        model.RestResponse,
		ProviderHash:       "get-pet",
		Endpoint:           "/pets/*",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		Bodies:             petBody(),
	}
	deployed := &model.PersistedContract{
		ParticipantName: participantName,
		Resources: map[string]model.PersistedResource{
			"list-pets": {Bodies: petBody()},
			"get-pet":   gone,
		},
	}
	next := &model.UploadedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.UploadedResource{"list-pets": {Bodies: petBody()}},
	}

	removed := contract_differ.RemovedSinceDeployed(deployed, next)

	gone.Removed = true
	assert.Equal(t, map[string]model.PersistedResource{"get-pet": gone}, removed)
}

func TestRemovedSinceDeployed_ResourceThatOnlyLostFields_DoesNotCount(t *testing.T) {
	deployed := &model.PersistedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.PersistedResource{"list-pets": {Bodies: petBody()}},
		Schemas:         petSchemas(),
	}
	next := &model.UploadedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.UploadedResource{"list-pets": {Bodies: petBody()}},
		Schemas:         model.Schemas{"Pet": {"": {Type: "object"}}},
	}

	assert.Empty(t, contract_differ.RemovedSinceDeployed(deployed, next))
}

func TestRemovedSinceDeployed_ResourceAlreadyRemovedInTheDeployedVersion_DoesNotCount(t *testing.T) {
	deployed := &model.PersistedContract{
		ParticipantName: participantName,
		Resources: map[string]model.PersistedResource{
			"list-pets": {Bodies: petBody()},
			"get-pet":   {Removed: true},
		},
	}
	next := &model.UploadedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.UploadedResource{"list-pets": {Bodies: petBody()}},
	}

	assert.Empty(t, contract_differ.RemovedSinceDeployed(deployed, next))
}

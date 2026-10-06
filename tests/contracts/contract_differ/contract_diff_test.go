package contract_differ_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
)

const participantName = "pets-service"

func newContractWithOnePetsResource() *model.UploadedContract {
	contract := model.NewUploadedContract(0, participantName, "1", "raw")
	_ = contract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
		"$":    model.NewProperty("$", "object", false),
		"$.id": model.NewProperty("$.id", "string", false),
	}))
	return contract
}

// props projects a contract's resources to the properties-by-hash shape the differ consumes.
func props(contract *model.UploadedContract) map[string]model.ResourceProperties {
	out := make(map[string]model.ResourceProperties, len(contract.Resources))
	for key, resource := range contract.Resources {
		out[key] = resource.Properties
	}
	return out
}

func TestDiff_NoChanges_BetweenEquivalentContracts(t *testing.T) {
	prev := newContractWithOnePetsResource()
	next := newContractWithOnePetsResource()
	diff := contract_differ.DiffResourceProperties(props(prev), props(next))
	assert.Empty(t, diff.Resources)
}

func TestDiff_ReportsAddedResource(t *testing.T) {
	prev := newContractWithOnePetsResource()
	next := newContractWithOnePetsResource()
	added := model.NewRestResponseProvider("/pets/*", "get", "200", map[string]model.Property{
		"$":    model.NewProperty("$", "object", false),
		"$.id": model.NewProperty("$.id", "string", false),
	})
	_ = next.AddResource(added)
	key := added.PrimaryHash()

	diff := contract_differ.DiffResourceProperties(props(prev), props(next))

	assert.Len(t, diff.Resources, 1)
	assert.Equal(t, model.ChangeAdded, diff.Resources[key].Kind)
}

func TestDiff_NextNil_AllResourcesRemoved(t *testing.T) {
	prev := newContractWithOnePetsResource()
	diff := contract_differ.DiffResourceProperties(props(prev), nil)
	assert.Len(t, diff.Resources, 1)
	for _, change := range diff.Resources {
		assert.Equal(t, model.ChangeRemoved, change.Kind)
	}
}

func TestDiff_RemovedResource(t *testing.T) {
	oldContract := newContractWithOnePetsResource()
	removed := model.NewRestResponseProvider("/pets/*", "get", "200", map[string]model.Property{
		"$":    model.NewProperty("$", "object", false),
		"$.id": model.NewProperty("$.id", "string", false),
	})
	_ = oldContract.AddResource(removed)
	key := removed.PrimaryHash()

	newContract := newContractWithOnePetsResource()
	diff := contract_differ.DiffResourceProperties(props(oldContract), props(newContract))

	assert.Len(t, diff.Resources, 1)
	assert.Equal(t, model.ChangeRemoved, diff.Resources[key].Kind)
	assert.Len(t, diff.Resources[key].Properties, 2)
	for _, propChange := range diff.Resources[key].Properties {
		assert.Equal(t, model.ChangeRemoved, propChange.Kind)
	}
}

func TestDiff_ModifiedResource_PropertyAdded(t *testing.T) {
	oldContract := newContractWithOnePetsResource()
	newContract := model.NewUploadedContract(0, participantName, "1", "raw")
	_ = newContract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
		"$":      model.NewProperty("$", "object", false),
		"$.id":   model.NewProperty("$.id", "string", false),
		"$.name": model.NewProperty("$.name", "string", false),
	}))

	diff := contract_differ.DiffResourceProperties(props(oldContract), props(newContract))

	assert.Len(t, diff.Resources, 1)
	for _, change := range diff.Resources {
		assert.Equal(t, model.ChangeModified, change.Kind)
		assert.Len(t, change.Properties, 1)
		assert.Equal(t, model.ChangeAdded, change.Properties["$.name"].Kind)
	}
}

func TestDiff_ModifiedResource_PropertyRemoved(t *testing.T) {
	oldContract := model.NewUploadedContract(0, participantName, "1", "raw")
	_ = oldContract.AddResource(
		model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
			"$":      model.NewProperty("$", "object", false),
			"$.id":   model.NewProperty("$.id", "string", false),
			"$.name": model.NewProperty("$.name", "string", false),
		}),
	)
	newContract := newContractWithOnePetsResource()

	diff := contract_differ.DiffResourceProperties(props(oldContract), props(newContract))

	assert.Len(t, diff.Resources, 1)
	for _, change := range diff.Resources {
		assert.Equal(t, model.ChangeModified, change.Kind)
		assert.Len(t, change.Properties, 1)
		assert.Equal(t, model.ChangeRemoved, change.Properties["$.name"].Kind)
	}
}

func TestDiff_ModifiedResource_PropertyTypeChanged(t *testing.T) {
	oldContract := newContractWithOnePetsResource()
	newContract := model.NewUploadedContract(0, participantName, "1", "raw")
	_ = newContract.AddResource(model.NewRestResponseProvider("/pets", "get", "200", map[string]model.Property{
		"$":    model.NewProperty("$", "object", false),
		"$.id": model.NewProperty("$.id", "int", false),
	}))

	diff := contract_differ.DiffResourceProperties(props(oldContract), props(newContract))

	assert.Len(t, diff.Resources, 1)
	for _, resourceChange := range diff.Resources {
		assert.Equal(t, model.ChangeModified, resourceChange.Kind)
		assert.Equal(t, model.ChangeModified, resourceChange.Properties["$.id"].Kind)
		assert.Equal(t, "string", resourceChange.Properties["$.id"].Before.Type)
		assert.Equal(t, "int", resourceChange.Properties["$.id"].After.Type)
	}
}

func TestRemovedSinceDeployed_ResourceGoneFromTheFiles_Counts(t *testing.T) {
	properties := map[string]model.Property{"$.id": model.NewProperty("$.id", "string", false)}
	gone := model.PersistedResource{
		ParticipantName:    participantName,
		Direction:          model.Provides,
		Interaction:        model.RestResponse,
		ProviderHash:       "get-pet",
		Endpoint:           "/pets/*",
		Method:             "get",
		ResponseStatusCode: null.StringFrom("200"),
		Properties:         properties,
	}
	deployed := &model.PersistedContract{
		ParticipantName: participantName,
		Resources: map[string]model.PersistedResource{
			"list-pets": {Properties: properties},
			"get-pet":   gone,
		},
	}
	next := &model.UploadedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.UploadedResource{"list-pets": {Properties: properties}},
	}

	removed := contract_differ.RemovedSinceDeployed(deployed, next)

	gone.Removed = true
	assert.Equal(t, map[string]model.PersistedResource{"get-pet": gone}, removed)
}

func TestRemovedSinceDeployed_ResourceThatOnlyLostProperties_DoesNotCount(t *testing.T) {
	deployed := &model.PersistedContract{
		ParticipantName: participantName,
		Resources: map[string]model.PersistedResource{
			"list-pets": {Properties: map[string]model.Property{
				"$.id":   model.NewProperty("$.id", "string", false),
				"$.name": model.NewProperty("$.name", "string", false),
			}},
		},
	}
	next := &model.UploadedContract{
		ParticipantName: participantName,
		Resources: map[string]model.UploadedResource{
			"list-pets": {Properties: map[string]model.Property{
				"$.id": model.NewProperty("$.id", "string", false),
			}},
		},
	}

	assert.Empty(t, contract_differ.RemovedSinceDeployed(deployed, next))
}

func TestRemovedSinceDeployed_ResourceAlreadyRemovedInTheDeployedVersion_DoesNotCount(t *testing.T) {
	properties := map[string]model.Property{"$.id": model.NewProperty("$.id", "string", false)}
	deployed := &model.PersistedContract{
		ParticipantName: participantName,
		Resources: map[string]model.PersistedResource{
			"list-pets": {Properties: properties},
			"get-pet":   {Properties: map[string]model.Property{}, Removed: true},
		},
	}
	next := &model.UploadedContract{
		ParticipantName: participantName,
		Resources:       map[string]model.UploadedResource{"list-pets": {Properties: properties}},
	}

	assert.Empty(t, contract_differ.RemovedSinceDeployed(deployed, next))
}

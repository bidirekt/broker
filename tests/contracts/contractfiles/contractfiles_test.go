package contractfiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

var interpreter = contractfiles.NewInterpreter(
	protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
	[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
)

const unsupportedNotesError = "unsupported contract file: notes.txt (expected .yaml, .yml, .graphql, .graphqls or .gql)"

var participant = &model.Participant{ID: 7, Name: "pets_service"}

const petsEndpointYAML = `provides:
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

const statusOutOfRangeYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          999: Pet
`

const malformedYAML = "provides: {"

const malformedYAMLError = "malformed contract file: broken.yaml: [1:11] could not find flow mapping end token '}'\n>  1 | provides: {\n                 ^\n"

func TestHasBlankSource_BlankSourceInAnyFile_ReturnsTrue(t *testing.T) {
	for _, source := range []string{"", "   "} {
		assert.True(t, contractfiles.HasBlankSource([]model.ContractFile{
			{Source: source, Content: petsEndpointYAML},
			{Source: "broken.yaml", Content: malformedYAML},
		}))
		assert.True(t, contractfiles.HasBlankSource([]model.ContractFile{
			{Source: "broken.yaml", Content: malformedYAML},
			{Source: source, Content: petsEndpointYAML},
		}))
	}
}

func TestHasBlankSource_EverySourceSet_ReturnsFalse(t *testing.T) {
	assert.False(t, contractfiles.HasBlankSource([]model.ContractFile{
		{Source: "schemas.yaml", Content: petSchemaYAML},
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
	}))
}

func TestToUploadedContract_ValidFiles_ReadsEveryFile(t *testing.T) {
	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "schemas.yaml", Content: petSchemaYAML},
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
	}, participant, "v1")

	require.NoError(t, readErr)
	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Contains(t, uploadedContract.Schemas[model.YAMLNamespace], "Pet")
	assert.Contains(t, uploadedContract.Resources, model.Hash("pets_service", "rest", "/pets", "get", "200"))
}

func TestToUploadedContract_UnsupportedExtension_ReturnsTheReadError(t *testing.T) {
	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "notes.txt", Content: petsEndpointYAML},
	}, participant, "v1")

	assert.EqualError(t, readErr, unsupportedNotesError)
	assert.NoError(t, err)
	assert.Nil(t, uploadedContract)
	assert.Nil(t, violations)
}

func TestToUploadedContract_MalformedYAML_ReturnsTheReadError(t *testing.T) {
	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "broken.yaml", Content: malformedYAML},
	}, participant, "v1")

	assert.EqualError(t, readErr, malformedYAMLError)
	assert.NoError(t, err)
	assert.Nil(t, uploadedContract)
	assert.Nil(t, violations)
}

func TestToUploadedContract_FirstFailingFileInRequestOrder_DecidesTheReadError(t *testing.T) {
	_, _, readErr, _ := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "broken.yaml", Content: malformedYAML},
		{Source: "notes.txt", Content: petsEndpointYAML},
	}, participant, "v1")
	assert.EqualError(t, readErr, malformedYAMLError)

	_, _, readErr, _ = interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "notes.txt", Content: petsEndpointYAML},
		{Source: "broken.yaml", Content: malformedYAML},
	}, participant, "v1")
	assert.EqualError(t, readErr, unsupportedNotesError)
}

func TestToUploadedContract_ReadErrorInAnyFile_WinsOverShapeViolations(t *testing.T) {
	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "a.yaml", Content: statusOutOfRangeYAML},
		{Source: "broken.yaml", Content: malformedYAML},
	}, participant, "v1")

	assert.EqualError(t, readErr, malformedYAMLError)
	assert.NoError(t, err)
	assert.Nil(t, uploadedContract)
	assert.Nil(t, violations)
}

func TestToUploadedContract_ShapeViolations_SortedBySourceWithoutContract(t *testing.T) {
	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "b.yaml", Content: statusOutOfRangeYAML},
		{Source: "a.yaml", Content: statusOutOfRangeYAML},
	}, participant, "v1")

	require.NoError(t, readErr)
	require.NoError(t, err)
	assert.Nil(t, uploadedContract)
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "status.out_of_range",
			Path:      "provides;rest;/pets;get;responses;999",
			Source:    "a.yaml",
			Details:   map[string]any{"key": "999", "error": "must be between 100 and 599"},
		},
		{
			ErrorCode: "status.out_of_range",
			Path:      "provides;rest;/pets;get;responses;999",
			Source:    "b.yaml",
			Details:   map[string]any{"key": "999", "error": "must be between 100 and 599"},
		},
	}, violations)
}

func TestToUploadedContract_ValidFiles_StampsTheParticipantVersionAndContent(t *testing.T) {
	files := []model.ContractFile{
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
		{Source: "schemas.yaml", Content: petSchemaYAML},
	}

	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract(files, participant, "v1")

	require.NoError(t, readErr)
	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, int64(7), uploadedContract.ParticipantID)
	assert.Equal(t, "pets_service", uploadedContract.ParticipantName)
	assert.Equal(t, "v1", uploadedContract.Version)
	assert.Equal(t, files, uploadedContract.Files)
}

func TestToUploadedContract_ValidFiles_AddsEachResourceUnderItsHash(t *testing.T) {
	uploadedContract, _, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
		{Source: "schemas.yaml", Content: petSchemaYAML},
	}, participant, "v1")

	require.NoError(t, readErr)
	require.NoError(t, err)
	require.Len(t, uploadedContract.Resources, 1)
	resource, added := uploadedContract.Resources[model.Hash("pets_service", "rest", "/pets", "get", "200")]
	require.True(t, added)
	assert.Equal(t, "pets_service", resource.ParticipantName)
	assert.Equal(t, "provides GET /pets 200", resource.Describe())
}

func TestToUploadedContract_RuleViolations_ReturnedWithoutContract(t *testing.T) {
	uploadedContract, violations, readErr, err := interpreter.ToUploadedContract([]model.ContractFile{
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
	}, participant, "v1")

	require.NoError(t, readErr)
	require.NoError(t, err)
	assert.Nil(t, uploadedContract)
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "endpoints.yaml",
			Details:   map[string]any{"schema": "Pet", "resource": "provides GET /pets 200"},
		},
	}, violations)
}

func TestToViolations_UnsupportedExtension_ReturnsTheReadError(t *testing.T) {
	_, violations, readErr := interpreter.ToViolations([]model.ContractFile{
		{Source: "notes.txt", Content: petsEndpointYAML},
	})

	assert.EqualError(t, readErr, unsupportedNotesError)
	assert.Nil(t, violations)
}

func TestToViolations_ShapeViolations_SameViolationsAsToUploadedContract(t *testing.T) {
	files := []model.ContractFile{
		{Source: "b.yaml", Content: statusOutOfRangeYAML},
		{Source: "a.yaml", Content: statusOutOfRangeYAML},
	}
	_, uploadedContractViolations, readErr, err := interpreter.ToUploadedContract(files, participant, "v1")
	require.NoError(t, readErr)
	require.NoError(t, err)

	_, violations, readErr := interpreter.ToViolations(files)

	require.NoError(t, readErr)
	assert.Len(t, violations, 2)
	assert.Equal(t, uploadedContractViolations, violations)
}

func TestToViolations_UnresolvedSchemaName_SameViolationsAsToUploadedContract(t *testing.T) {
	files := []model.ContractFile{{Source: "endpoints.yaml", Content: petsEndpointYAML}}
	_, uploadedContractViolations, readErr, err := interpreter.ToUploadedContract(files, participant, "v1")
	require.NoError(t, readErr)
	require.NoError(t, err)

	_, violations, readErr := interpreter.ToViolations(files)

	require.NoError(t, readErr)
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "endpoints.yaml",
			Details:   map[string]any{"schema": "Pet", "resource": "provides GET /pets 200"},
		},
	}, violations)
	assert.Equal(t, uploadedContractViolations, violations)
}

func TestToViolations_SchemaDeclaredInTwoFiles_SameViolationsAsToUploadedContract(t *testing.T) {
	files := []model.ContractFile{
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
		{Source: "schemas.yaml", Content: petSchemaYAML},
		{Source: "pets.yaml", Content: petSchemaYAML},
	}
	_, uploadedContractViolations, readErr, err := interpreter.ToUploadedContract(files, participant, "v1")
	require.NoError(t, readErr)
	require.NoError(t, err)

	_, violations, readErr := interpreter.ToViolations(files)

	require.NoError(t, readErr)
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.duplicate",
			Path:      "schemas;Pet",
			Source:    "schemas.yaml",
			Details:   map[string]any{"schema": "Pet", "declaredIn": "pets.yaml"},
		},
	}, violations)
	assert.Equal(t, uploadedContractViolations, violations)
}

func TestToViolations_ValidFiles_ReturnsNoViolation(t *testing.T) {
	_, violations, readErr := interpreter.ToViolations([]model.ContractFile{
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
		{Source: "schemas.yaml", Content: petSchemaYAML},
	})

	require.NoError(t, readErr)
	assert.Empty(t, violations)
}

package contractfiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

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

const contractContent = "raw contract content"

func decodeFragments(t *testing.T, files ...contractfiles.File) []contract.Fragment {
	t.Helper()

	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		fragment, err := contract.DecodeFragment(file.Source, file.Content)
		require.NoError(t, err)

		fragments = append(fragments, fragment)
	}

	return fragments
}

func TestToFragments_ValidFiles_DecodedInRequestOrder(t *testing.T) {
	fragments, violations, err := contractfiles.ToFragments([]contractfiles.File{
		{Source: "schemas.yaml", Content: petSchemaYAML},
		{Source: "endpoints.yaml", Content: petsEndpointYAML},
	})

	require.NoError(t, err)
	assert.Empty(t, violations)
	require.Len(t, fragments, 2)
	assert.Equal(t, "schemas.yaml", fragments[0].Source)
	assert.NotNil(t, fragments[0].Root().Mapping("schemas").Mapping("Pet"))
	assert.Equal(t, "endpoints.yaml", fragments[1].Source)
	assert.NotNil(t, fragments[1].Root().Mapping("provides").Mapping("rest").Mapping("/pets"))
}

func TestToFragments_BlankSource_ReturnsErrBlankSource(t *testing.T) {
	for _, source := range []string{"", "   "} {
		fragments, violations, err := contractfiles.ToFragments([]contractfiles.File{
			{Source: source, Content: petsEndpointYAML},
		})

		assert.ErrorIs(t, err, contractfiles.ErrBlankSource)
		assert.Nil(t, fragments)
		assert.Nil(t, violations)
	}
}

func TestToFragments_UnsupportedExtension_ReturnsTheDecodeError(t *testing.T) {
	fragments, violations, err := contractfiles.ToFragments([]contractfiles.File{
		{Source: "notes.txt", Content: petsEndpointYAML},
	})

	assert.EqualError(t, err, "unsupported contract file: notes.txt (expected .yaml or .yml)")
	assert.Nil(t, fragments)
	assert.Nil(t, violations)
}

func TestToFragments_MalformedYAML_ReturnsTheDecodeError(t *testing.T) {
	fragments, violations, err := contractfiles.ToFragments([]contractfiles.File{
		{Source: "broken.yaml", Content: malformedYAML},
	})

	assert.EqualError(t, err, malformedYAMLError)
	assert.Nil(t, fragments)
	assert.Nil(t, violations)
}

func TestToFragments_FirstFailingFileInRequestOrder_DecidesTheError(t *testing.T) {
	_, _, err := contractfiles.ToFragments([]contractfiles.File{
		{Source: "broken.yaml", Content: malformedYAML},
		{Source: " ", Content: petsEndpointYAML},
	})
	assert.EqualError(t, err, malformedYAMLError)

	_, _, err = contractfiles.ToFragments([]contractfiles.File{
		{Source: " ", Content: petsEndpointYAML},
		{Source: "broken.yaml", Content: malformedYAML},
	})
	assert.ErrorIs(t, err, contractfiles.ErrBlankSource)
}

func TestToFragments_DecodeErrorInAnyFile_WinsOverShapeViolations(t *testing.T) {
	fragments, violations, err := contractfiles.ToFragments([]contractfiles.File{
		{Source: "a.yaml", Content: statusOutOfRangeYAML},
		{Source: "broken.yaml", Content: malformedYAML},
	})

	assert.EqualError(t, err, malformedYAMLError)
	assert.Nil(t, fragments)
	assert.Nil(t, violations)
}

func TestToFragments_ShapeViolations_SortedBySourceWithoutFragments(t *testing.T) {
	fragments, violations, err := contractfiles.ToFragments([]contractfiles.File{
		{Source: "b.yaml", Content: statusOutOfRangeYAML},
		{Source: "a.yaml", Content: statusOutOfRangeYAML},
	})

	require.NoError(t, err)
	assert.Nil(t, fragments)
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "status.out_of_range",
			Path:      "provides;rest;/pets;get;responses;999",
			Source:    "a.yaml",
			Details:   map[string]string{"key": "999", "error": "must be between 100 and 599"},
		},
		{
			ErrorCode: "status.out_of_range",
			Path:      "provides;rest;/pets;get;responses;999",
			Source:    "b.yaml",
			Details:   map[string]string{"key": "999", "error": "must be between 100 and 599"},
		},
	}, violations)
}

func TestToUploadedContract_ValidFragments_StampsTheParticipantVersionAndContent(t *testing.T) {
	fragments := decodeFragments(t,
		contractfiles.File{Source: "endpoints.yaml", Content: petsEndpointYAML},
		contractfiles.File{Source: "schemas.yaml", Content: petSchemaYAML},
	)
	participant := &model.Participant{ID: 7, Name: "pets_service"}

	uploadedContract, violations, err := contractfiles.ToUploadedContract(fragments, participant, "v1", contractContent)

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, int64(7), uploadedContract.ParticipantID)
	assert.Equal(t, "pets_service", uploadedContract.ParticipantName)
	assert.Equal(t, "v1", uploadedContract.Version)
	assert.Equal(t, contractContent, uploadedContract.ContractContent)
}

func TestToUploadedContract_ValidFragments_AddsEachResourceUnderItsHash(t *testing.T) {
	fragments := decodeFragments(t,
		contractfiles.File{Source: "endpoints.yaml", Content: petsEndpointYAML},
		contractfiles.File{Source: "schemas.yaml", Content: petSchemaYAML},
	)
	participant := &model.Participant{ID: 7, Name: "pets_service"}

	uploadedContract, _, err := contractfiles.ToUploadedContract(fragments, participant, "v1", contractContent)

	require.NoError(t, err)
	require.Len(t, uploadedContract.Resources, 1)
	resource, added := uploadedContract.Resources[model.Hash("pets_service", "/pets", "get", "200")]
	require.True(t, added)
	assert.Equal(t, "pets_service", resource.ParticipantName)
	assert.Equal(t, "provides GET /pets 200", resource.Describe())
}

func TestToUploadedContract_RuleViolations_ReturnedWithoutContract(t *testing.T) {
	fragments := decodeFragments(t, contractfiles.File{Source: "endpoints.yaml", Content: petsEndpointYAML})
	participant := &model.Participant{ID: 7, Name: "pets_service"}

	uploadedContract, violations, err := contractfiles.ToUploadedContract(fragments, participant, "v1", contractContent)

	require.NoError(t, err)
	assert.Nil(t, uploadedContract)
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "endpoints.yaml",
			Details:   map[string]string{"schema": "Pet", "resource": "provides GET /pets 200"},
		},
	}, violations)
}

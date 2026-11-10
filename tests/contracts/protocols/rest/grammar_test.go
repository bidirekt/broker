package rest_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
)

func grammarDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestEndpoint_RejectsADynamicSegmentWithBraces(t *testing.T) {
	subject := descriptor.Map(restcontracts.Endpoint, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"/users/{id}": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "endpoint.syntax", violations[0].ErrorCode)
	assert.Equal(t, "/users/{id}", violations[0].Path)
	assert.Equal(t, "/users/{id}", violations[0].Details["key"])
	assert.Contains(t, violations[0].Details["error"], "dynamic path segments must use *")
}

func TestEndpoint_RejectsADynamicSegmentWithAColon(t *testing.T) {
	subject := descriptor.Map(restcontracts.Endpoint, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"/pets/:petId": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "endpoint.syntax", violations[0].ErrorCode)
	assert.Equal(t, "/pets/:petId", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "/pets/:petId", "error": "dynamic path segments must use *"}, violations[0].Details)
}

func TestEndpoint_NormalizesTheTrailingSlashBeforeChecking(t *testing.T) {
	subject := descriptor.Map(restcontracts.Endpoint, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"/pets/": nil}, "api.yaml")

	assert.Empty(t, violations)
}

func TestStatusCode_AcceptsQuotedAndUnquoted(t *testing.T) {
	subject := descriptor.Map(restcontracts.StatusCode, descriptor.String())

	violations := descriptor.Validate(subject, grammarDocument(t, "200: Pet\n\"404\": Error\n"), "api.yaml")

	assert.Empty(t, violations)
}

func TestStatusCode_RejectsOutOfRangeInStableOrder(t *testing.T) {
	subject := descriptor.Map(restcontracts.StatusCode, descriptor.String())

	violations := descriptor.Validate(subject, grammarDocument(t, "600: Nope\nabc: X\n007: Y\n"), "api.yaml")

	require.Len(t, violations, 3)
	assert.Equal(t, "status.out_of_range", violations[0].ErrorCode)
	assert.Equal(t, "600", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "600", "error": "must be between 100 and 599"}, violations[0].Details)
	assert.Equal(t, "7", violations[1].Details["key"])
	assert.Equal(t, "abc", violations[2].Details["key"])
}

func TestStatusCode_RejectsASignEvenWithinRange(t *testing.T) {
	subject := descriptor.Map(restcontracts.StatusCode, descriptor.String())

	violations := descriptor.Validate(subject, map[string]any{"+200": "Pet", "-1": "Error"}, "api.yaml")

	require.Len(t, violations, 2)
	assert.Equal(t, "+200", violations[0].Details["key"])
	assert.Equal(t, "-1", violations[1].Details["key"])
}

func TestStatusCode_RejectsLeadingZeros(t *testing.T) {
	subject := descriptor.Map(restcontracts.StatusCode, descriptor.String())

	violations := descriptor.Validate(subject, map[string]any{"0200": "Pet"}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "status.out_of_range", violations[0].ErrorCode)
	assert.Equal(t, "0200", violations[0].Details["key"])
}

func TestEndpoint_DoubleSlashAtTheEndStaysMalformed(t *testing.T) {
	subject := descriptor.Map(restcontracts.Endpoint, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"/users//": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "endpoint.syntax", violations[0].ErrorCode)
	assert.Equal(t, "/users//", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "/users//", "error": "malformed path"}, violations[0].Details)
}

func TestEndpoint_RejectsDoubleSlashAsAnEmptySegment(t *testing.T) {
	subject := descriptor.Map(restcontracts.Endpoint, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"//": nil}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "endpoint.syntax", violations[0].ErrorCode)
	assert.Equal(t, "//", violations[0].Path)
	assert.Equal(t, map[string]string{"key": "//", "error": "malformed path"}, violations[0].Details)
}

func TestEndpoint_AcceptsTheRoot(t *testing.T) {
	subject := descriptor.Map(restcontracts.Endpoint, descriptor.Object(descriptor.Fields{}))

	violations := descriptor.Validate(subject, map[string]any{"/": nil}, "api.yaml")

	assert.Empty(t, violations)
}

package violation_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/violation"
)

func TestViolation_SerializaEmCamelCase(t *testing.T) {
	subject := violation.Violation{
		ErrorCode: "key.unknown",
		Path:      "provides;rest;/pets;patch",
		Source:    "api.yaml",
		Details:   map[string]string{"key": "patch"},
	}

	raw, err := json.Marshal(subject)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"code": "key.unknown",
		"path": "provides;rest;/pets;patch",
		"source": "api.yaml",
		"details": {"key": "patch"}
	}`, string(raw))
}

func TestSortedByLocation_OrdenaPelaOrigemDepoisPeloCaminho(t *testing.T) {
	violations := []violation.Violation{
		{ErrorCode: "schema.duplicate", Path: "a", Source: "b.yaml"},
		{ErrorCode: "schema.duplicate", Path: "z", Source: "a.yaml"},
	}

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "schema.duplicate", Path: "z", Source: "a.yaml"},
		{ErrorCode: "schema.duplicate", Path: "a", Source: "b.yaml"},
	}, violation.SortedByLocation(violations))
}

func TestSortedByLocation_EmpateMantemAOrdemDeEntrada(t *testing.T) {
	violations := []violation.Violation{
		{ErrorCode: "schema.duplicate", Path: "schemas;Pet", Source: "a.yaml"},
		{ErrorCode: "resource.duplicate", Path: "provides;rest;/pets;get;responses;200", Source: "a.yaml"},
		{ErrorCode: "schema.unresolved_name", Path: "provides;rest;/pets;get;responses;200", Source: "a.yaml"},
	}

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "resource.duplicate", Path: "provides;rest;/pets;get;responses;200", Source: "a.yaml"},
		{ErrorCode: "schema.unresolved_name", Path: "provides;rest;/pets;get;responses;200", Source: "a.yaml"},
		{ErrorCode: "schema.duplicate", Path: "schemas;Pet", Source: "a.yaml"},
	}, violation.SortedByLocation(violations))
}

func TestSortedByLocation_NaoAlteraAEntrada(t *testing.T) {
	violations := []violation.Violation{{Source: "b.yaml"}, {Source: "a.yaml"}}

	violation.SortedByLocation(violations)

	assert.Equal(t, []violation.Violation{{Source: "b.yaml"}, {Source: "a.yaml"}}, violations)
}

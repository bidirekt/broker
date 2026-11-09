package model_test

import (
	"encoding/json"
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerdictBreak_MarshalJSON_PutsTheLocationUnderItsProtocol(t *testing.T) {
	verdictBreak := model.VerdictBreak{
		Protocol: "rest",
		Location: map[string]string{"endpoint": "/pets", "method": "get", "interaction": "200"},
		Reason:   model.ReasonPropertyMissingInProvider,
		Details:  map[string]string{"property": "name"},
	}

	encoded, err := json.Marshal(verdictBreak)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"rest": {"endpoint": "/pets", "method": "get", "interaction": "200"},
		"reason": "property_missing_in_provider",
		"details": {"property": "name"}
	}`, string(encoded))
}

func TestVerdictBreak_RoundTrip_KeepsEveryField(t *testing.T) {
	breaks := []model.VerdictBreak{{
		Protocol: "rest",
		Location: map[string]string{"endpoint": "/pets", "method": "post", "interaction": "request"},
		Reason:   model.ReasonPropertyTypeMismatch,
		Details:  map[string]string{"property": "id", "consumerName": "petstore_web"},
	}}

	encoded, err := json.Marshal(breaks)
	require.NoError(t, err)

	var decoded []model.VerdictBreak
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	assert.Equal(t, breaks, decoded)
}

func TestVerdictBreak_UnmarshalJSON_ReadsJsonbFormattedText(t *testing.T) {
	stored := `{"rest": {"method": "get", "endpoint": "/pets", "interaction": "200"}, "reason": "property_missing_in_provider", "details": null}`

	var decoded model.VerdictBreak
	require.NoError(t, json.Unmarshal([]byte(stored), &decoded))

	assert.Equal(t, model.VerdictBreak{
		Protocol: "rest",
		Location: map[string]string{"endpoint": "/pets", "method": "get", "interaction": "200"},
		Reason:   model.ReasonPropertyMissingInProvider,
	}, decoded)
}

func TestVerdictBreak_UnmarshalJSON_RejectsABreakWithoutProtocol(t *testing.T) {
	var decoded model.VerdictBreak

	err := json.Unmarshal([]byte(`{"reason": "property_missing_in_provider", "details": {}}`), &decoded)

	require.EqualError(t, err, "verdict break has no protocol")
}

func TestVerdictBreak_UnmarshalJSON_RejectsABreakWithTwoProtocols(t *testing.T) {
	var decoded model.VerdictBreak

	err := json.Unmarshal([]byte(`{"grpc": {}, "rest": {}, "reason": "property_missing_in_provider"}`), &decoded)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verdict break has more than one protocol")
}

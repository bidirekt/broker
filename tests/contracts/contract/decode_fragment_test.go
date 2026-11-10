package contract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
)

func TestDecodeFragment_KeepsNumericKeysAsWritten(t *testing.T) {
	fragment, err := contract.DecodeFragment("api.yaml", "responses:\n  0200: Pet\n  0o310: Pet\n  200: Pet\n  200.0: Pet\n")

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"responses": map[string]any{
		"0200":  "Pet",
		"0o310": "Pet",
		"200":   "Pet",
		"200.0": "Pet",
	}}, fragment.Document)
}

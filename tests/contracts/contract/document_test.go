package contract_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/stretchr/testify/assert"
)

func TestDocument_MappingDevolveOSubmapa(t *testing.T) {
	document := contract.Document{"schemas": map[string]any{"Pet": map[string]any{"type": "object"}}}

	assert.Equal(t, contract.Document{"Pet": map[string]any{"type": "object"}}, document.Mapping("schemas"))
	assert.Equal(t, "object", document.Mapping("schemas").Mapping("Pet").Text("type"))
}

func TestDocument_MappingDeChaveAusenteEhNil(t *testing.T) {
	document := contract.Document{"schemas": map[string]any{}}

	assert.Nil(t, document.Mapping("provides"))
	assert.Nil(t, document.Mapping("provides").Mapping("rest"))
}

func TestDocument_MappingDeValorQueNaoEhMapaEhNil(t *testing.T) {
	document := contract.Document{"provides": "rest", "consumes": []any{"billing_api"}, "schemas": 1}

	assert.Nil(t, document.Mapping("provides"))
	assert.Nil(t, document.Mapping("consumes"))
	assert.Nil(t, document.Mapping("schemas"))
}

func TestDocument_TextDevolveAString(t *testing.T) {
	document := contract.Document{"type": "string", "ref": "Pet"}

	assert.Equal(t, "string", document.Text("type"))
	assert.Equal(t, "Pet", document.Text("ref"))
}

func TestDocument_TextDeChaveAusenteOuNaoStringEhVazio(t *testing.T) {
	document := contract.Document{"type": 42, "ref": true, "items": map[string]any{}}

	assert.Equal(t, "", document.Text("description"))
	assert.Equal(t, "", document.Text("type"))
	assert.Equal(t, "", document.Text("ref"))
	assert.Equal(t, "", document.Text("items"))
}

func TestDocument_FlagDevolveOBooleano(t *testing.T) {
	document := contract.Document{"optional": true, "deprecated": false}

	assert.True(t, document.Flag("optional"))
	assert.False(t, document.Flag("deprecated"))
}

func TestDocument_FlagDeChaveAusenteOuNaoBooleanaEhFalse(t *testing.T) {
	document := contract.Document{"optional": "true", "nullable": 1}

	assert.False(t, document.Flag("optional"))
	assert.False(t, document.Flag("nullable"))
	assert.False(t, document.Flag("deprecated"))
}

func TestDocument_NilRespondeComValoresZero(t *testing.T) {
	var document contract.Document

	assert.Nil(t, document.Mapping("provides"))
	assert.Nil(t, document.Mapping("provides").Mapping("rest"))
	assert.Equal(t, "", document.Text("type"))
	assert.False(t, document.Flag("optional"))
	assert.Empty(t, document.Keys())
}

func TestDocument_KeysVemOrdenadas(t *testing.T) {
	document := contract.Document{"post": nil, "delete": nil, "get": nil, "put": nil, "/b": nil, "/a": nil}

	assert.Equal(t, []string{"/a", "/b", "delete", "get", "post", "put"}, document.Keys())
}

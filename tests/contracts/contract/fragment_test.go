package contract_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/stretchr/testify/assert"
)

func TestFragment_RootDevolveODocumentoQuandoEhMapa(t *testing.T) {
	fragment := contract.Fragment{Source: "api.yaml", Document: map[string]any{"provides": map[string]any{"rest": map[string]any{}}}}

	assert.Equal(t, contract.Document{"provides": map[string]any{"rest": map[string]any{}}}, fragment.Root())
	assert.NotNil(t, fragment.Root().Mapping("provides").Mapping("rest"))
}

func TestFragment_RootDeDocumentoNilEhNil(t *testing.T) {
	fragment := contract.Fragment{Source: "empty.yaml"}

	assert.Nil(t, fragment.Root())
	assert.Nil(t, fragment.Root().Mapping("provides"))
}

func TestFragment_RootDeDocumentoQueNaoEhMapaEhNil(t *testing.T) {
	for name, document := range map[string]any{
		"texto":  "just text",
		"lista":  []any{"a", "b"},
		"numero": 42,
	} {
		t.Run(name, func(t *testing.T) {
			fragment := contract.Fragment{Source: "odd.yaml", Document: document}

			assert.Nil(t, fragment.Root())
		})
	}
}

func TestSortedBySource_OrdenaPelaOrigem(t *testing.T) {
	fragments := []contract.Fragment{
		{Source: "c.yaml", Document: "c"},
		{Source: "a.yaml", Document: "a"},
		{Source: "b.yaml", Document: "b"},
	}

	assert.Equal(t, []contract.Fragment{
		{Source: "a.yaml", Document: "a"},
		{Source: "b.yaml", Document: "b"},
		{Source: "c.yaml", Document: "c"},
	}, contract.SortedBySource(fragments))
}

func TestSortedBySource_NaoAlteraAEntrada(t *testing.T) {
	fragments := []contract.Fragment{{Source: "b.yaml"}, {Source: "a.yaml"}}

	contract.SortedBySource(fragments)

	assert.Equal(t, []contract.Fragment{{Source: "b.yaml"}, {Source: "a.yaml"}}, fragments)
}

func TestSortedBySource_QualquerOrdemDeEntradaDaOMesmoResultado(t *testing.T) {
	forward := contract.SortedBySource([]contract.Fragment{{Source: "a.yaml"}, {Source: "b.yaml"}, {Source: "c.yaml"}})
	backward := contract.SortedBySource([]contract.Fragment{{Source: "c.yaml"}, {Source: "b.yaml"}, {Source: "a.yaml"}})

	assert.Equal(t, forward, backward)
}

func TestSortedBySource_EntradaVaziaDaListaVazia(t *testing.T) {
	assert.Empty(t, contract.SortedBySource(nil))
}

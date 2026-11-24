package validator_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/validator"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

const probeGeneratedNamespace = "probe_generated"

type probeProtocol struct{}

func (this probeProtocol) Protocol() model.Protocol {
	return "probe"
}

func (this probeProtocol) Grammar() protocol.Grammar {
	block := descriptor.Map(descriptor.Key{}, descriptor.String())

	return protocol.Grammar{Provides: block, Consumes: block}
}

func (this probeProtocol) ToDeclarations(block protocol.Block) protocol.BlockDeclarations {
	var declarations protocol.BlockDeclarations

	for _, key := range block.Document.Keys() {
		declarations.Schemas = append(declarations.Schemas, protocol.SchemaDeclaration{
			Source:    block.Source,
			Namespace: probeGeneratedNamespace,
			Name:      "Shape",
			Schema:    contract.Schema{Type: block.Document.Text(key)},
		})
	}

	if len(block.Document) > 0 {
		declarations.Extension = strings.Join(block.Document.Keys(), "+")
	}

	return declarations
}

func (this probeProtocol) Rules() []protocol.Rule {
	return []protocol.Rule{func(declarations protocol.Declarations) []violation.Violation {
		extension, present := declarations.Extensions["probe"].(string)
		if !present {
			return nil
		}

		return []violation.Violation{{ErrorCode: "probe.extension", Path: "provides;probe", Source: "a.yaml", Details: map[string]string{"extension": extension}}}
	}}
}

func (this probeProtocol) MergeExtensions(pieces []protocol.ExtensionPiece) (any, []violation.Violation) {
	var values []string
	for _, piece := range pieces {
		values = append(values, piece.Value.(string))
	}

	return strings.Join(values, ","), nil
}

func validateWithProbe(fragments ...contract.Fragment) (protocol.Declarations, []violation.Violation) {
	registry := protocol.NewRegistry([]protocol.Protocol{probeProtocol{}, restcontracts.New()})
	declarations := fragmentmapper.ToDeclarations(fragments, registry)
	declarations.Extensions, _ = registry.MergeExtensions(declarations.ExtensionPieces)

	return declarations, validator.Validate(declarations, registry.Rules()...)
}

func probeBlock(side string, keys map[string]any) map[string]any {
	if side == "provides" {
		return map[string]any{"provides": map[string]any{"probe": keys}}
	}

	return map[string]any{"consumes": map[string]any{"pets_api": map[string]any{"probe": keys}}}
}

func generatedShapes(declarations protocol.Declarations) []protocol.SchemaDeclaration {
	return slices.DeleteFunc(slices.Clone(declarations.Schemas), func(schema protocol.SchemaDeclaration) bool {
		return schema.Namespace != probeGeneratedNamespace
	})
}

func TestValidate_TwoBlocksGeneratingTheSameSchema_KeepOneCopy(t *testing.T) {
	declarations, violations := validateWithProbe(
		contract.Fragment{Source: "a.yaml", Document: probeBlock("provides", map[string]any{"pet": "object"})},
		contract.Fragment{Source: "b.yaml", Document: probeBlock("consumes", map[string]any{"pets": "object"})},
	)

	assert.Equal(t, []protocol.SchemaDeclaration{{
		Source:    "a.yaml",
		Namespace: probeGeneratedNamespace,
		Name:      "Shape",
		Schema:    contract.Schema{Type: "object"},
	}}, generatedShapes(declarations))
	assert.Equal(t, contract.Schema{Type: "object"}, declarations.Catalog[probeGeneratedNamespace]["Shape"])
	assert.NotContains(t, violationCodes(violations), "schema.duplicate")
}

func TestValidate_TwoBlocksGeneratingDifferentSchemasUnderOneName_IsADuplicate(t *testing.T) {
	declarations, violations := validateWithProbe(
		contract.Fragment{Source: "a.yaml", Document: probeBlock("provides", map[string]any{"pet": "object"})},
		contract.Fragment{Source: "b.yaml", Document: probeBlock("consumes", map[string]any{"pets": "array"})},
	)

	assert.Len(t, generatedShapes(declarations), 2)
	assert.Equal(t, contract.Schema{Type: "object"}, declarations.Catalog[probeGeneratedNamespace]["Shape"])
	assert.Contains(t, violations, violation.Violation{
		ErrorCode: "schema.duplicate",
		Path:      "schemas;Shape",
		Source:    "b.yaml",
		Details:   map[string]string{"schema": "Shape", "declaredIn": "a.yaml"},
	})
}

func TestValidate_YAMLSchemaUnderAGeneratedName_IsNotJoinedWithTheGeneratedOne(t *testing.T) {
	declarations, violations := validateWithProbe(contract.Fragment{Source: "a.yaml", Document: map[string]any{
		"provides": map[string]any{"probe": map[string]any{"pet": "object"}},
		"schemas":  map[string]any{"Shape": map[string]any{"type": "object"}},
	}})

	assert.Len(t, declarations.Schemas, 2)
	assert.Equal(t, contract.Schema{Type: "object"}, declarations.Catalog[model.YAMLNamespace]["Shape"])
	assert.Equal(t, contract.Schema{Type: "object"}, declarations.Catalog[probeGeneratedNamespace]["Shape"])
	assert.NotContains(t, violationCodes(violations), "schema.duplicate")
}

func TestValidate_ProtocolRulesJoinTheCoreRules_OverTheExtensionJoinedFromEveryBlock(t *testing.T) {
	_, violations := validateWithProbe(
		contract.Fragment{Source: "b.yaml", Document: probeBlock("consumes", map[string]any{"pets": "object"})},
		contract.Fragment{Source: "a.yaml", Document: map[string]any{
			"provides": map[string]any{
				"probe": map[string]any{"pet": "object"},
				"rest":  map[string]any{"/pets": map[string]any{"get": map[string]any{"responses": map[string]any{"200": "Missing"}}}},
			},
		}},
	)

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "probe.extension",
			Path:      "provides;probe",
			Source:    "a.yaml",
			Details:   map[string]string{"extension": "pet,pets"},
		},
		{
			ErrorCode: "schema.unresolved_name",
			Path:      "provides;rest;/pets;get;responses;200",
			Source:    "a.yaml",
			Details:   map[string]string{"schema": "Missing", "resource": "provides GET /pets 200"},
		},
	}, violations)
}

func violationCodes(violations []violation.Violation) []string {
	var codes []string
	for _, found := range violations {
		codes = append(codes, found.ErrorCode)
	}

	return codes
}

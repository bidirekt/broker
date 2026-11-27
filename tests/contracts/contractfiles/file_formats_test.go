package contractfiles_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

type probeProtocol struct{}

func (this probeProtocol) Protocol() model.Protocol {
	return "probe"
}

func (this probeProtocol) Grammar() protocol.Grammar {
	block := descriptor.Object(descriptor.Fields{"tag": descriptor.String()})

	return protocol.Grammar{Provides: block, Consumes: block}
}

func (this probeProtocol) ToDeclarations(block protocol.Block) protocol.BlockDeclarations {
	if block.Document == nil {
		return protocol.BlockDeclarations{}
	}

	return protocol.BlockDeclarations{Extension: block.Document.Text("tag")}
}

func (this probeProtocol) MergeExtensions(pieces []protocol.ExtensionPiece) (any, []violation.Violation) {
	var values []string
	for _, piece := range pieces {
		values = append(values, piece.Value.(string))
	}

	return strings.Join(values, ","), nil
}

func (this probeProtocol) Rules() []protocol.Rule {
	return []protocol.Rule{func(declarations protocol.Declarations) []violation.Violation {
		extension, _ := declarations.Extensions["probe"].(string)

		var schemas []string
		for _, schema := range declarations.Schemas {
			schemas = append(schemas, schema.Namespace+":"+schema.Name)
		}

		return []violation.Violation{{ErrorCode: "probe.seen", Details: map[string]any{
			"formats":   strings.Join(declarations.Formats, ", "),
			"extension": extension,
			"schemas":   strings.Join(schemas, ", "),
		}}}
	}}
}

type probeFormat struct{}

func (this probeFormat) Extensions() []string {
	return []string{".probe"}
}

func (this probeFormat) ToDeclarations(files []model.ContractFile) (protocol.Declarations, error) {
	declarations := protocol.Declarations{
		Catalog:         map[string]contract.SchemasMap{"probe": {}},
		ExtensionPieces: map[string][]protocol.ExtensionPiece{},
		Formats:         []string{"probe"},
	}

	for _, file := range files {
		if file.Content == "broken" {
			return protocol.Declarations{}, errors.New(file.Source + ": unreadable")
		}

		schema := contract.Schema{Type: "string"}
		declarations.Schemas = append(declarations.Schemas, protocol.SchemaDeclaration{Source: file.Source, Namespace: "probe", Name: file.Content, Schema: schema})
		declarations.Catalog["probe"][file.Content] = schema
		declarations.ExtensionPieces["probe"] = append(declarations.ExtensionPieces["probe"], protocol.ExtensionPiece{Source: file.Source, Value: file.Content})
	}

	return declarations, nil
}

var probeInterpreter = contractfiles.NewInterpreter(
	protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), probeProtocol{}}),
	[]contractfiles.FileFormat{probeFormat{}},
)

const probeTagYAML = "provides:\n  probe:\n    tag: Tagged\n"

func TestToViolations_FilesOfARegisteredFormat_AreReadByIt_AndJoinedWithTheYAMLBeforeTheRules(t *testing.T) {
	violations, readErr := probeInterpreter.ToViolations([]model.ContractFile{
		{Source: "api.yaml", Content: probeTagYAML},
		{Source: "Schema.PROBE", Content: "Scalar"},
	})

	require.NoError(t, readErr)
	assert.Equal(t, []violation.Violation{{ErrorCode: "probe.seen", Details: map[string]any{
		"formats":   "yaml, probe",
		"extension": "Scalar,Tagged",
		"schemas":   "probe:Scalar",
	}}}, violations)
}

func TestToViolations_AFormatReadError_IsTheReadError(t *testing.T) {
	violations, readErr := probeInterpreter.ToViolations([]model.ContractFile{
		{Source: "api.yaml", Content: probeTagYAML},
		{Source: "schema.probe", Content: "broken"},
	})

	assert.EqualError(t, readErr, "schema.probe: unreadable")
	assert.Nil(t, violations)
}

func TestToViolations_AFormatReadError_WinsOverYAMLShapeViolations(t *testing.T) {
	violations, readErr := probeInterpreter.ToViolations([]model.ContractFile{
		{Source: "api.yaml", Content: "provides:\n  probe:\n    unknown: x\n"},
		{Source: "schema.probe", Content: "broken"},
	})

	assert.EqualError(t, readErr, "schema.probe: unreadable")
	assert.Nil(t, violations)
}

func TestToViolations_UnsupportedFile_ListsTheExtensionOfEveryRegisteredFormat(t *testing.T) {
	_, readErr := probeInterpreter.ToViolations([]model.ContractFile{{Source: "notes.txt", Content: probeTagYAML}})

	assert.EqualError(t, readErr, "unsupported contract file: notes.txt (expected .yaml, .yml or .probe)")
}

func TestToViolations_UnsupportedFile_WithoutFormats_ListsTheYAMLExtensions(t *testing.T) {
	yamlOnly := contractfiles.NewInterpreter(protocol.NewRegistry([]protocol.Protocol{restcontracts.New()}), nil)

	_, readErr := yamlOnly.ToViolations([]model.ContractFile{{Source: "schema.probe", Content: "Scalar"}})

	assert.EqualError(t, readErr, "unsupported contract file: schema.probe (expected .yaml or .yml)")
}

func TestNewInterpreter_AnExtensionRegisteredTwice_Panics(t *testing.T) {
	assert.PanicsWithError(t, `contract file extension ".probe" is registered twice`, func() {
		contractfiles.NewInterpreter(protocol.NewRegistry(nil), []contractfiles.FileFormat{probeFormat{}, probeFormat{}})
	})
}

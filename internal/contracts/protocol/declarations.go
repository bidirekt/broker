package protocol

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/model"
)

type Declarations struct {
	Resources       []ResourceDeclaration
	Schemas         []SchemaDeclaration
	Catalog         map[string]contract.SchemasMap
	ExtensionPieces map[string][]ExtensionPiece
	Extensions      map[string]any
	Formats         []string
}

// BlockDeclarations are what one block declares; its Schemas are generated, each in a namespace the protocol chooses.
type BlockDeclarations struct {
	Resources []ResourceDeclaration
	Schemas   []SchemaDeclaration
	Extension any
}

type ExtensionPiece struct {
	Source string
	Value  any
}

type ResourceDeclaration struct {
	Source        string
	Path          contract.ResourcePath
	Namespace     string
	SchemaName    string
	WithoutSchema bool
	Resource      model.UploadedResource
}

type SchemaDeclaration struct {
	Source        string
	Namespace     string
	Name          string
	Schema        contract.Schema
	KeepUnreached bool
}

// AddSchema keeps the first schema of a name as the one the catalog resolves; a later one is left to the duplicate rule.
func (this *Declarations) AddSchema(schema SchemaDeclaration) {
	this.Schemas = append(this.Schemas, schema)

	if this.Catalog == nil {
		this.Catalog = map[string]contract.SchemasMap{}
	}

	if this.Catalog[schema.Namespace] == nil {
		this.Catalog[schema.Namespace] = contract.SchemasMap{}
	}

	if _, declared := this.Catalog[schema.Namespace][schema.Name]; !declared {
		this.Catalog[schema.Namespace][schema.Name] = schema.Schema
	}
}

func (this *Declarations) AddExtensionPiece(protocolName string, piece ExtensionPiece) {
	if this.ExtensionPieces == nil {
		this.ExtensionPieces = map[string][]ExtensionPiece{}
	}

	this.ExtensionPieces[protocolName] = append(this.ExtensionPieces[protocolName], piece)
}

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

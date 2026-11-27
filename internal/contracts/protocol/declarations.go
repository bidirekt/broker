package protocol

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/model"
)

type Declarations struct {
	Resources []ResourceDeclaration
	Schemas   []SchemaDeclaration
	Catalog   map[string]contract.SchemasMap
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
	Source    string
	Namespace string
	Name      string
	Schema    contract.Schema
}

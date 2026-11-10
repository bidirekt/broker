package protocol

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/model"
)

type Declarations struct {
	Resources []ResourceDeclaration
	Schemas   []SchemaDeclaration
	Catalog   contract.SchemasMap
}

type ResourceDeclaration struct {
	Source        string
	Path          contract.ResourcePath
	SchemaName    string
	WithoutSchema bool
	Resource      model.UploadedResource
}

type SchemaDeclaration struct {
	Source string
	Name   string
	Schema contract.Schema
}

package protocol

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

type Protocol interface {
	Protocol() model.Protocol
	Grammar() descriptor.Node
	ToResourceDeclarations(block Block) []ResourceDeclaration
}

// Block is one protocol's mapping under provides or under consumes.<service>; Document is nil when absent.
type Block struct {
	Source           string
	Path             contract.ResourcePath
	Document         contract.Document
	Direction        model.Direction
	ConsumedProvider null.String
	Protocol         model.Protocol
}

// schemaRef is the schema name, or nil for a body declared as `~`.
func (this Block) ToResourceDeclaration(
	path contract.ResourcePath,
	body model.BodyRole,
	identity model.ResourceIdentity,
	schemaRef any,
) ResourceDeclaration {
	resource := model.UploadedResource{Resource: model.Resource{
		Direction:        this.Direction,
		Protocol:         this.Protocol,
		Body:             body,
		Identity:         identity,
		ConsumedProvider: this.ConsumedProvider,
	}}

	if schemaRef == nil {
		resource.Bodies = []model.ResourceBody{{Source: this.Source}}

		return ResourceDeclaration{Source: this.Source, Path: path, WithoutSchema: true, Resource: resource}
	}

	schemaName, _ := schemaRef.(string)
	resource.Bodies = []model.ResourceBody{{Source: this.Source, SchemaName: null.StringFrom(schemaName)}}

	return ResourceDeclaration{Source: this.Source, Path: path, SchemaName: schemaName, Resource: resource}
}

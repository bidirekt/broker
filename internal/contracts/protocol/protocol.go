package protocol

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

type Protocol interface {
	Protocol() model.Protocol
	Grammar() Grammar
	ToDeclarations(block Block) BlockDeclarations
}

// Grammar is the protocol's block under provides and under consumes.<service>.
type Grammar struct {
	Provides descriptor.Node
	Consumes descriptor.Node
}

type RuleSet interface {
	// Each rule runs after the core rules, over the declarations of the whole publish.
	Rules() []Rule
}

type Rule func(declarations Declarations) []violation.Violation

type VocabularyExtender interface {
	// SchemaVocabulary holds only what the protocol adds to the core vocabulary.
	SchemaVocabulary() descriptor.Vocabulary
}

type ExtensionMerger interface {
	// pieces come in file order; the value goes to Declarations.Extensions under the protocol's name.
	MergeExtensions(pieces []ExtensionPiece) (any, []violation.Violation)
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
	resource.Bodies = []model.ResourceBody{{Source: this.Source, Namespace: model.YAMLNamespace, SchemaName: null.StringFrom(schemaName)}}

	return ResourceDeclaration{Source: this.Source, Path: path, Namespace: model.YAMLNamespace, SchemaName: schemaName, Resource: resource}
}

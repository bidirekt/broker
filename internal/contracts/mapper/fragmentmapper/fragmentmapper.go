package fragmentmapper

import (
	"reflect"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

func ToDeclarations(fragments []contract.Fragment, registry *protocol.Registry) protocol.Declarations {
	declarations := protocol.Declarations{Catalog: map[string]contract.SchemasMap{}}
	generated := map[[2]string]contract.Schema{}

	for _, fragment := range contract.SortedBySource(fragments) {
		schemas := fragment.Root().Mapping("schemas")

		for _, name := range schemas.Keys() {
			addSchema(&declarations, protocol.SchemaDeclaration{
				Source:    fragment.Source,
				Namespace: model.YAMLNamespace,
				Name:      name,
				Schema:    contract.SchemaFromDocument(schemas.Mapping(name)),
			})
		}

		for _, block := range blocksOf(fragment, registry) {
			blockDeclarations := block.plugin.ToDeclarations(block.Block)
			declarations.Resources = append(declarations.Resources, blockDeclarations.Resources...)

			for _, schema := range blockDeclarations.Schemas {
				key := [2]string{schema.Namespace, schema.Name}

				first, declared := generated[key]
				if declared && reflect.DeepEqual(first, schema.Schema) {
					continue
				}

				if !declared {
					generated[key] = schema.Schema
				}

				addSchema(&declarations, schema)
			}

			if blockDeclarations.Extension != nil {
				if declarations.ExtensionPieces == nil {
					declarations.ExtensionPieces = map[string][]protocol.ExtensionPiece{}
				}

				name := block.Protocol.String()
				declarations.ExtensionPieces[name] = append(declarations.ExtensionPieces[name], protocol.ExtensionPiece{
					Source: block.Source,
					Value:  blockDeclarations.Extension,
				})
			}
		}
	}

	return declarations
}

// The first declaration of a name is the one the catalog resolves; a later one is left to the duplicate rule.
func addSchema(declarations *protocol.Declarations, schema protocol.SchemaDeclaration) {
	declarations.Schemas = append(declarations.Schemas, schema)

	if declarations.Catalog[schema.Namespace] == nil {
		declarations.Catalog[schema.Namespace] = contract.SchemasMap{}
	}

	if _, declared := declarations.Catalog[schema.Namespace][schema.Name]; !declared {
		declarations.Catalog[schema.Namespace][schema.Name] = schema.Schema
	}
}

func ToResourceModels(declarations protocol.Declarations) []model.UploadedResource {
	merged := ToMergedResourceDeclarations(declarations)

	resources := make([]model.UploadedResource, 0, len(merged))
	for _, declaration := range merged {
		resources = append(resources, declaration.Resource)
	}

	return resources
}

func ToSchemaModels(declarations protocol.Declarations) map[string]model.Schemas {
	roots := map[string][]string{}
	for _, declaration := range declarations.Resources {
		if !declaration.WithoutSchema {
			roots[declaration.Namespace] = append(roots[declaration.Namespace], declaration.SchemaName)
		}
	}

	for _, declaration := range declarations.Schemas {
		if declaration.KeepUnreached {
			roots[declaration.Namespace] = append(roots[declaration.Namespace], declaration.Name)
		}
	}

	schemas := make(map[string]model.Schemas, len(declarations.Catalog))
	for namespace, catalog := range declarations.Catalog {
		schemas[namespace] = schemamapper.ToSchemaModels(catalog, roots[namespace])
	}

	return schemas
}

type pluginBlock struct {
	protocol.Block
	plugin protocol.Protocol
}

func blocksOf(fragment contract.Fragment, registry *protocol.Registry) []pluginBlock {
	document := fragment.Root()
	root := contract.NewResourcePath("")

	var blocks []pluginBlock

	consumes := document.Mapping("consumes")
	for _, serviceName := range consumes.Keys() {
		for _, plugin := range registry.Protocols() {
			name := plugin.Protocol().String()
			blocks = append(blocks, pluginBlock{plugin: plugin, Block: protocol.Block{
				Source:           fragment.Source,
				Path:             root.Append("consumes", serviceName, name),
				Document:         consumes.Mapping(serviceName).Mapping(name),
				Direction:        model.Consumes,
				ConsumedProvider: null.NewString(serviceName, serviceName != ""),
				Protocol:         plugin.Protocol(),
			}})
		}
	}

	provides := document.Mapping("provides")
	for _, plugin := range registry.Protocols() {
		name := plugin.Protocol().String()
		blocks = append(blocks, pluginBlock{plugin: plugin, Block: protocol.Block{
			Source:    fragment.Source,
			Path:      root.Append("provides", name),
			Document:  provides.Mapping(name),
			Direction: model.Provides,
			Protocol:  plugin.Protocol(),
		}})
	}

	return blocks
}

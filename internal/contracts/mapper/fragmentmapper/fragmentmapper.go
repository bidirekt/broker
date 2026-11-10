package fragmentmapper

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

func ToDeclarations(fragments []contract.Fragment, registry *protocol.Registry) protocol.Declarations {
	declarations := protocol.Declarations{Catalog: contract.SchemasMap{}}

	for _, fragment := range contract.SortedBySource(fragments) {
		schemas := fragment.Root().Mapping("schemas")

		for _, name := range schemas.Keys() {
			schema := contract.SchemaFromDocument(schemas.Mapping(name))
			declarations.Schemas = append(declarations.Schemas, protocol.SchemaDeclaration{Source: fragment.Source, Name: name, Schema: schema})

			if _, declared := declarations.Catalog[name]; !declared {
				declarations.Catalog[name] = schema
			}
		}

		declarations.Resources = append(declarations.Resources, resourceDeclarationsFromFragment(fragment, registry)...)
	}

	return declarations
}

func ToResourceModels(declarations protocol.Declarations) []model.UploadedResource {
	merged := ToMergedResourceDeclarations(declarations)

	resources := make([]model.UploadedResource, 0, len(merged))
	for _, declaration := range merged {
		resources = append(resources, declaration.Resource)
	}

	return resources
}

func ToSchemaModels(declarations protocol.Declarations) model.Schemas {
	roots := make([]string, 0, len(declarations.Resources))
	for _, declaration := range declarations.Resources {
		if !declaration.WithoutSchema {
			roots = append(roots, declaration.SchemaName)
		}
	}

	return schemamapper.ToSchemaModels(declarations.Catalog, roots)
}

func resourceDeclarationsFromFragment(fragment contract.Fragment, registry *protocol.Registry) []protocol.ResourceDeclaration {
	document := fragment.Root()
	root := contract.NewResourcePath("")

	var declarations []protocol.ResourceDeclaration

	consumes := document.Mapping("consumes")
	for _, serviceName := range consumes.Keys() {
		for _, plugin := range registry.Protocols() {
			name := plugin.Protocol().String()
			declarations = append(declarations, plugin.ToResourceDeclarations(protocol.Block{
				Source:           fragment.Source,
				Path:             root.Append("consumes", serviceName, name),
				Document:         consumes.Mapping(serviceName).Mapping(name),
				Direction:        model.Consumes,
				ConsumedProvider: null.NewString(serviceName, serviceName != ""),
				Protocol:         plugin.Protocol(),
			})...)
		}
	}

	provides := document.Mapping("provides")
	for _, plugin := range registry.Protocols() {
		name := plugin.Protocol().String()
		declarations = append(declarations, plugin.ToResourceDeclarations(protocol.Block{
			Source:    fragment.Source,
			Path:      root.Append("provides", name),
			Document:  provides.Mapping(name),
			Direction: model.Provides,
			Protocol:  plugin.Protocol(),
		})...)
	}

	return declarations
}

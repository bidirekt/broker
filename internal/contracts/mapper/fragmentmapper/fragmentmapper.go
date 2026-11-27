package fragmentmapper

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

func ToDeclarations(fragments []contract.Fragment, registry *protocol.Registry) protocol.Declarations {
	declarations := protocol.Declarations{Catalog: map[string]contract.SchemasMap{}}

	for _, fragment := range contract.SortedBySource(fragments) {
		schemas := fragment.Root().Mapping("schemas")

		for _, name := range schemas.Keys() {
			if declarations.Catalog[model.YAMLNamespace] == nil {
				declarations.Catalog[model.YAMLNamespace] = contract.SchemasMap{}
			}

			schema := contract.SchemaFromDocument(schemas.Mapping(name))
			declarations.Schemas = append(declarations.Schemas, protocol.SchemaDeclaration{
				Source:    fragment.Source,
				Namespace: model.YAMLNamespace,
				Name:      name,
				Schema:    schema,
			})

			if _, declared := declarations.Catalog[model.YAMLNamespace][name]; !declared {
				declarations.Catalog[model.YAMLNamespace][name] = schema
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

func ToSchemaModels(declarations protocol.Declarations) map[string]model.Schemas {
	roots := map[string][]string{}
	for _, declaration := range declarations.Resources {
		if !declaration.WithoutSchema {
			roots[declaration.Namespace] = append(roots[declaration.Namespace], declaration.SchemaName)
		}
	}

	schemas := make(map[string]model.Schemas, len(declarations.Catalog))
	for namespace, catalog := range declarations.Catalog {
		schemas[namespace] = schemamapper.ToSchemaModels(catalog, roots[namespace])
	}

	return schemas
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

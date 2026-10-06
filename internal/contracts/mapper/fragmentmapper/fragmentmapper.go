package fragmentmapper

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/resourcepathmapper"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
)

type Declarations struct {
	Resources []ResourceDeclaration
	Schemas   []SchemaDeclaration
	Catalog   contract.SchemasMap
}

type ResourceDeclaration struct {
	Source     string
	Path       contract.ResourcePath
	SchemaName string
	Resource   model.UploadedResource
}

type SchemaDeclaration struct {
	Source string
	Name   string
	Schema contract.Schema
}

var methodsInOrder = []string{"get", "post", "put", "delete"}

func ToDeclarations(fragments []contract.Fragment) Declarations {
	sorted := contract.SortedBySource(fragments)
	declarations := Declarations{Catalog: contract.SchemasMap{}}

	for _, fragment := range sorted {
		schemas := fragment.Root().Mapping("schemas")

		for _, name := range schemas.Keys() {
			schema := contract.SchemaFromDocument(schemas.Mapping(name))
			declarations.Schemas = append(declarations.Schemas, SchemaDeclaration{Source: fragment.Source, Name: name, Schema: schema})

			if _, declared := declarations.Catalog[name]; !declared {
				declarations.Catalog[name] = schema
			}
		}
	}

	for _, fragment := range sorted {
		declarations.Resources = append(declarations.Resources, resourceDeclarationsFromFragment(fragment, declarations.Catalog)...)
	}

	return declarations
}

func ToResourceModels(declarations Declarations) []model.UploadedResource {
	merged := mergeByResourcePath(declarations.Resources)

	resources := make([]model.UploadedResource, 0, len(merged))
	for _, declaration := range merged {
		resources = append(resources, declaration.Resource)
	}

	return resources
}

func resourceDeclarationsFromFragment(fragment contract.Fragment, catalog contract.SchemasMap) []ResourceDeclaration {
	document := fragment.Root()
	root := contract.NewResourcePath("")

	var declarations []ResourceDeclaration

	consumes := document.Mapping("consumes")
	for _, serviceName := range consumes.Keys() {
		declarations = append(declarations, resourceDeclarationsFromRest(
			fragment.Source,
			consumes.Mapping(serviceName).Mapping("rest"),
			root.Append("consumes", serviceName),
			catalog,
		)...)
	}

	return append(declarations, resourceDeclarationsFromRest(
		fragment.Source,
		document.Mapping("provides").Mapping("rest"),
		root.Append("provides"),
		catalog,
	)...)
}

func resourceDeclarationsFromRest(source string, rest contract.Document, resourcePath contract.ResourcePath, catalog contract.SchemasMap) []ResourceDeclaration {
	var declarations []ResourceDeclaration

	for _, endpoint := range rest.Keys() {
		endpointPath := resourcePath.Append("rest", contract.NormalizeEndpoint(endpoint))
		methods := rest.Mapping(endpoint)

		for _, method := range methodsInOrder {
			operation := methods.Mapping(method)
			methodPath := endpointPath.Append(method)

			if request := operation.Text("request"); request != "" {
				declarations = append(declarations, resourceDeclaration(source, request, methodPath.Append("request"), catalog))
			}

			responses := operation.Mapping("responses")
			for _, status := range responses.Keys() {
				declarations = append(declarations, resourceDeclaration(source, responses.Text(status), methodPath.Append("responses", status), catalog))
			}
		}
	}

	return declarations
}

func resourceDeclaration(source string, schemaName string, resourcePath contract.ResourcePath, catalog contract.SchemasMap) ResourceDeclaration {
	properties := schemamapper.ToPropertyModels(catalog, catalog[schemaName])

	return ResourceDeclaration{
		Source:     source,
		Path:       resourcePath,
		SchemaName: schemaName,
		Resource:   resourcepathmapper.ToResourceModel(resourcePath, properties),
	}
}

package fragmentmapper

import (
	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/resourcepathmapper"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
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

var methodsInOrder = []string{"get", "post", "put", "patch", "delete"}

func ToDeclarations(fragments []contract.Fragment) Declarations {
	declarations := Declarations{Catalog: contract.SchemasMap{}}

	for _, fragment := range contract.SortedBySource(fragments) {
		schemas := fragment.Root().Mapping("schemas")

		for _, name := range schemas.Keys() {
			schema := contract.SchemaFromDocument(schemas.Mapping(name))
			declarations.Schemas = append(declarations.Schemas, SchemaDeclaration{Source: fragment.Source, Name: name, Schema: schema})

			if _, declared := declarations.Catalog[name]; !declared {
				declarations.Catalog[name] = schema
			}
		}

		declarations.Resources = append(declarations.Resources, resourceDeclarationsFromFragment(fragment)...)
	}

	return declarations
}

func ToResourceModels(declarations Declarations) []model.UploadedResource {
	merged := ToMergedResourceDeclarations(declarations)

	resources := make([]model.UploadedResource, 0, len(merged))
	for _, declaration := range merged {
		resources = append(resources, declaration.Resource)
	}

	return resources
}

func ToSchemaModels(declarations Declarations) model.Schemas {
	roots := make([]string, 0, len(declarations.Resources))
	for _, declaration := range declarations.Resources {
		if !declaration.WithoutSchema {
			roots = append(roots, declaration.SchemaName)
		}
	}

	return schemamapper.ToSchemaModels(declarations.Catalog, roots)
}

func resourceDeclarationsFromFragment(fragment contract.Fragment) []ResourceDeclaration {
	document := fragment.Root()
	root := contract.NewResourcePath("")

	var declarations []ResourceDeclaration

	consumes := document.Mapping("consumes")
	for _, serviceName := range consumes.Keys() {
		declarations = append(declarations, resourceDeclarationsFromRest(
			fragment.Source,
			consumes.Mapping(serviceName).Mapping("rest"),
			root.Append("consumes", serviceName),
		)...)
	}

	return append(declarations, resourceDeclarationsFromRest(
		fragment.Source,
		document.Mapping("provides").Mapping("rest"),
		root.Append("provides"),
	)...)
}

func resourceDeclarationsFromRest(source string, rest contract.Document, resourcePath contract.ResourcePath) []ResourceDeclaration {
	var declarations []ResourceDeclaration

	for _, endpoint := range rest.Keys() {
		endpointPath := resourcePath.Append("rest", contract.NormalizeEndpoint(endpoint))
		methods := rest.Mapping(endpoint)

		for _, method := range methodsInOrder {
			operation := methods.Mapping(method)
			methodPath := endpointPath.Append(method)

			if request, present := operation["request"]; present {
				declarations = append(declarations, resourceDeclaration(source, request, methodPath.Append("request")))
			}

			responses := operation.Mapping("responses")
			for _, status := range responses.Keys() {
				declarations = append(declarations, resourceDeclaration(source, responses[status], methodPath.Append("responses", status)))
			}
		}
	}

	return declarations
}

func resourceDeclaration(source string, schemaRef any, resourcePath contract.ResourcePath) ResourceDeclaration {
	if schemaRef == nil {
		return ResourceDeclaration{
			Source:        source,
			Path:          resourcePath,
			WithoutSchema: true,
			Resource:      resourcepathmapper.ToResourceModel(resourcePath, []model.ResourceBody{{Source: source}}),
		}
	}

	schemaName, _ := schemaRef.(string)
	body := model.ResourceBody{Source: source, SchemaName: null.StringFrom(schemaName)}

	return ResourceDeclaration{
		Source:     source,
		Path:       resourcePath,
		SchemaName: schemaName,
		Resource:   resourcepathmapper.ToResourceModel(resourcePath, []model.ResourceBody{body}),
	}
}

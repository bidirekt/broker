package graphql

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

type Operations struct {
	Counterpart string
	Operations  []Operation
	Fields      []FieldBlock
}

type Operation struct {
	Operation string
	Field     string
	Breaks    []OperationBreak
}

type OperationBreak struct {
	Reason      model.BreakingReason
	Interaction string
	Sentence    breakreport.Phrase
	Fixes       []breakreport.Fix
	// Graph names the page of the graph deployed in the environment.
	Graph  string
	Causes []string
}

type FieldBlock struct {
	Field    string
	Reason   model.BreakingReason
	Sentence breakreport.Phrase
	Fixes    []breakreport.Fix
	Places   []Place
}

type Place struct {
	Operation        string
	Field            string
	Interaction      string
	Property         string
	Argument         string
	CounterpartField string
}

//go:embed graphql_operations.html operations.css
var sectionFiles embed.FS

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (this *Plugin) Protocol() model.Protocol {
	return graphqlmodel.Protocol
}

func (this *Plugin) ReportKey() string {
	return graphqlmodel.ReportKey
}

func (this *Plugin) Files() fs.FS {
	return sectionFiles
}

func (this *Plugin) Template() string {
	return "graphql_operations"
}

func (this *Plugin) ToSection(checked breakreport.CheckedCounterpart, tree json.RawMessage) (breakreport.Section, error) {
	var operations graphqlmodel.ReportTree
	if tree != nil {
		if err := json.Unmarshal(tree, &operations); err != nil {
			return breakreport.Section{}, fmt.Errorf("error decoding the operations tree: %w", err)
		}
	}

	display := toOperations(checked, operations)
	section := breakreport.Section{Display: display, Breaks: len(display.Fields)}
	for _, operation := range display.Operations {
		section.Breaks += len(operation.Breaks)
	}
	for _, field := range display.Fields {
		section.Places += len(field.Places)
	}

	return section, nil
}

func toOperations(checked breakreport.CheckedCounterpart, tree graphqlmodel.ReportTree) Operations {
	display := Operations{Counterpart: checked.Counterpart.Name}

	var blocks []fieldBlock
	for _, operation := range slices.Sorted(maps.Keys(tree)) {
		for _, field := range slices.Sorted(maps.Keys(tree[operation])) {
			checkedField := checkedOperation{CheckedCounterpart: checked, operation: operation, field: field}
			reported := Operation{Operation: operation, Field: field}

			interactions := tree[operation][field]
			for _, interaction := range slices.Sorted(maps.Keys(interactions)) {
				checkedField.interaction = interaction
				for _, contractBreak := range interactions[interaction] {
					if block, named := toFieldBlock(checkedField, contractBreak); named {
						blocks = append(blocks, block)
					} else {
						reported.Breaks = append(reported.Breaks, toOperationBreak(checkedField, contractBreak))
					}
				}
			}

			if len(reported.Breaks) > 0 {
				display.Operations = append(display.Operations, reported)
			}
		}
	}

	for _, block := range mergeFieldBlocks(blocks) {
		display.Fields = append(display.Fields, block.toFieldBlock())
	}

	return display
}

type checkedOperation struct {
	breakreport.CheckedCounterpart
	operation   string
	field       string
	interaction string
}

func (this checkedOperation) resource() string {
	return this.operation + " " + this.field
}

func propertyPathPhrase(path string) string {
	return fmt.Sprintf("%q", path)
}

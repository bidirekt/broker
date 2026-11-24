package protocol

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

type Registry struct {
	protocols []Protocol
	grammar   descriptor.Node
	rules     []Rule
}

func NewRegistry(protocols []Protocol) *Registry {
	sorted := slices.SortedFunc(slices.Values(protocols), func(a, b Protocol) int {
		return cmp.Compare(a.Protocol(), b.Protocol())
	})

	provides := make(descriptor.Fields, len(sorted))
	consumes := make(descriptor.Fields, len(sorted))
	vocabulary := descriptor.CoreVocabulary()

	var rules []Rule

	for _, protocol := range sorted {
		name := protocol.Protocol().String()
		if _, registered := provides[name]; registered {
			panic(fmt.Errorf("protocol %q is registered twice", name))
		}

		grammar := protocol.Grammar()
		provides[name] = grammar.Provides
		consumes[name] = grammar.Consumes

		if extender, extends := protocol.(VocabularyExtender); extends {
			vocabulary = vocabulary.With(extender.SchemaVocabulary())
		}

		if ruleSet, hasRules := protocol.(RuleSet); hasRules {
			rules = append(rules, ruleSet.Rules()...)
		}
	}

	return &Registry{
		protocols: sorted,
		grammar:   descriptor.Contract(provides, consumes, vocabulary),
		rules:     rules,
	}
}

func (this *Registry) Protocols() []Protocol {
	return this.protocols
}

func (this *Registry) Grammar() descriptor.Node {
	return this.grammar
}

func (this *Registry) Rules() []Rule {
	return this.rules
}

// MergeExtensions panics on pieces of a protocol that merges none.
func (this *Registry) MergeExtensions(pieces map[string][]ExtensionPiece) (map[string]any, []violation.Violation) {
	var extensions map[string]any
	var violations []violation.Violation

	for _, protocol := range this.protocols {
		name := protocol.Protocol().String()
		if len(pieces[name]) == 0 {
			continue
		}

		merger, merges := protocol.(ExtensionMerger)
		if !merges {
			panic(fmt.Errorf("protocol %q declares extension pieces but merges none", name))
		}

		inFileOrder := slices.Clone(pieces[name])
		slices.SortStableFunc(inFileOrder, func(a, b ExtensionPiece) int {
			return strings.Compare(a.Source, b.Source)
		})

		value, mergeViolations := merger.MergeExtensions(inFileOrder)
		if extensions == nil {
			extensions = map[string]any{}
		}

		extensions[name] = value
		violations = append(violations, mergeViolations...)
	}

	return extensions, violations
}

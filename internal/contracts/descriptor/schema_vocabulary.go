package descriptor

import (
	"fmt"
	"maps"
	"slices"

	"github.com/bidirekt/broker/internal/model"
)

// Keys builds each key's grammar from the vocabulary it belongs to, so a recursive key nests the same grammar.
type Keys map[string]func(vocabulary Vocabulary) Node

type TypeVocabulary struct {
	Name string
	Keys Keys
	// Takes names keys another type declares that this type accepts too, with the grammar they have there.
	Takes []string
	Rules []Rule
}

type Vocabulary struct {
	Types        []TypeVocabulary
	PropertyKeys Keys

	// built is set on the copy handed to the keys, so a recursive key resolves to the grammar
	// they are building instead of building another one at every validation.
	built *schemaGrammar
}

func CoreVocabulary() Vocabulary {
	return Vocabulary{Types: []TypeVocabulary{
		{
			Name: model.ObjectType,
			Keys: Keys{"properties": func(vocabulary Vocabulary) Node {
				return Map(PropertyName, Lazy(vocabulary.Property))
			}},
			Rules: []Rule{objectRequiresProperties},
		},
		{
			Name: model.ArrayType,
			Keys: Keys{"items": func(vocabulary Vocabulary) Node {
				return Lazy(func() Node { return vocabulary.SchemaAt("items") })
			}},
			Rules: []Rule{arrayRequiresItems},
		},
		{
			Name: model.MapType,
			Keys: Keys{"values": func(vocabulary Vocabulary) Node {
				return Lazy(func() Node { return vocabulary.SchemaAt("values") })
			}},
			Rules: []Rule{mapRequiresValues},
		},
		{Name: model.StringType},
		{Name: model.IntegerType},
		{Name: model.FloatType},
		{Name: model.NumberType},
		{Name: model.BooleanType},
		{
			Name:  model.EnumType,
			Keys:  Keys{"members": func(Vocabulary) Node { return Members(model.EnumType, EnumValues()) }},
			Rules: []Rule{membersRequired(model.EnumType)},
		},
		{
			Name:  model.UnionType,
			Keys:  Keys{"members": func(Vocabulary) Node { return Members(model.UnionType, UnionMembers()) }},
			Rules: []Rule{membersRequired(model.UnionType)},
		},
	}}
}

// With adds the keys, taken keys and rules of a type this vocabulary already has to it; it panics on a key either already has.
func (this Vocabulary) With(extension Vocabulary) Vocabulary {
	joined := Vocabulary{Types: slices.Clone(this.Types), PropertyKeys: maps.Clone(this.PropertyKeys)}

	for _, added := range extension.Types {
		index := slices.IndexFunc(joined.Types, func(schemaType TypeVocabulary) bool { return schemaType.Name == added.Name })
		if index == -1 {
			joined.Types = append(joined.Types, added)

			continue
		}

		existing := joined.Types[index]
		existing.Keys = maps.Clone(existing.Keys)
		if existing.Keys == nil {
			existing.Keys = Keys{}
		}

		for key, grammar := range added.Keys {
			if _, defined := existing.Keys[key]; defined {
				panic(fmt.Errorf("schema type %q key %q is defined twice", added.Name, key))
			}

			existing.Keys[key] = grammar
		}

		existing.Takes = append(slices.Clip(existing.Takes), added.Takes...)
		existing.Rules = append(slices.Clip(existing.Rules), added.Rules...)
		joined.Types[index] = existing
	}

	for _, key := range slices.Sorted(maps.Keys(extension.PropertyKeys)) {
		if _, defined := joined.PropertyKeys[key]; defined {
			panic(fmt.Errorf("property key %q is defined twice", key))
		}

		if joined.PropertyKeys == nil {
			joined.PropertyKeys = Keys{}
		}

		joined.PropertyKeys[key] = extension.PropertyKeys[key]
	}

	return joined
}

func (this Vocabulary) Type() Enum {
	allowed := make([]string, 0, len(this.Types))
	for _, schemaType := range this.Types {
		allowed = append(allowed, schemaType.Name)
	}

	return Enum{ErrorCode: "schema.invalid_type", Allowed: allowed}
}

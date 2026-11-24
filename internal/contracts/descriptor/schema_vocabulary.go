package descriptor

import (
	"github.com/bidirekt/broker/internal/model"
)

// Keys builds each key's grammar from the vocabulary it belongs to, so a recursive key nests the same vocabulary.
type Keys map[string]func(vocabulary Vocabulary) Node

type TypeVocabulary struct {
	Name  string
	Keys  Keys
	Rules []Rule
}

type Vocabulary struct {
	Types        []TypeVocabulary
	PropertyKeys Keys
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

func (this Vocabulary) Type() Enum {
	allowed := make([]string, 0, len(this.Types))
	for _, schemaType := range this.Types {
		allowed = append(allowed, schemaType.Name)
	}

	return Enum{ErrorCode: "schema.invalid_type", Allowed: allowed}
}

package contract

import (
	"maps"
	"slices"
)

type Document map[string]any

func (this Document) Mapping(key string) Document {
	mapping, _ := this[key].(map[string]any)

	return mapping
}

func (this Document) Text(key string) string {
	text, _ := this[key].(string)

	return text
}

func (this Document) Flag(key string) bool {
	flag, _ := this[key].(bool)

	return flag
}

func (this Document) Keys() []string {
	return slices.Sorted(maps.Keys(this))
}

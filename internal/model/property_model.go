package model

import "strings"

type Property struct {
	ID       int64 `json:"-"`
	Path     string
	Type     string
	Optional bool
}

func (this *Property) IsSame(other *Property) bool {
	return this.Path == other.Path &&
		this.Type == other.Type &&
		this.Optional == other.Optional
}

func NewProperty(
	propertyPath string,
	propertyType string,
	optional bool,
) Property {
	return Property{
		Path:     propertyPath,
		Type:     propertyType,
		Optional: optional,
	}
}

func ParentPath(path string) (string, bool) {
	if parent, isArrayItem := strings.CutSuffix(path, "[]"); isArrayItem {
		return parent, true
	}

	if parent, isMapValue := strings.CutSuffix(path, "{}"); isMapValue {
		return parent, true
	}

	separator := strings.LastIndex(path, ".")
	if separator < 0 {
		return "", false
	}

	return path[:separator], true
}

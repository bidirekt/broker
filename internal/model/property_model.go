package model

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

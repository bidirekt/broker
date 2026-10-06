package schemamapper

type propertyPath string

func (this propertyPath) String() string {
	return string(this)
}

func (this propertyPath) Append(chunk string) propertyPath {
	return propertyPath(string(this) + "." + chunk)
}

func (this propertyPath) AppendArray() propertyPath {
	return propertyPath(string(this) + "[]")
}

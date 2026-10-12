package schemamapper

type fieldPath string

func (this fieldPath) String() string {
	return string(this)
}

func (this fieldPath) Append(name string) fieldPath {
	if this == "" {
		return fieldPath(name)
	}

	return fieldPath(string(this) + "." + name)
}

func (this fieldPath) AppendArray() fieldPath {
	return fieldPath(string(this) + "[]")
}

func (this fieldPath) AppendMap() fieldPath {
	return fieldPath(string(this) + "{}")
}

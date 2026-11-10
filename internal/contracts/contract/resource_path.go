package contract

import "strings"

type ResourcePath string

func NewResourcePath(resourcePath string) ResourcePath {
	return ResourcePath(resourcePath)
}

func (this *ResourcePath) Append(parts ...string) ResourcePath {
	separator := ";"

	if string(*this) == "" {
		return ResourcePath(strings.Join(parts, separator))
	}

	chunks := strings.Join(parts, separator)

	return ResourcePath(strings.Join([]string{string(*this), chunks}, separator))
}

func (this *ResourcePath) String() string {
	return string(*this)
}

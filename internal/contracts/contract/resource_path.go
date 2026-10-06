package contract

import (
	"regexp"
	"strings"
)

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

func (this *ResourcePath) Split() []string {
	return strings.Split(this.String(), ";")
}

func (this *ResourcePath) IsConsumer() bool {
	return this.Split()[0] == "consumes"
}

func (this *ResourcePath) IsProvider() bool {
	return this.Split()[0] == "provides"
}

func (this *ResourcePath) ExtractNamedArgs(regex *regexp.Regexp) (map[string]string, bool) {
	match := regex.FindStringSubmatch(this.String())
	if match == nil {
		return nil, false
	}

	args := make(map[string]string, len(regex.SubexpNames()))
	for i, name := range regex.SubexpNames() {
		if name == "" {
			continue
		}

		args[name] = match[i]
	}

	return args, true
}

package contract

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

func DecodeFragment(source, content string) (Fragment, error) {
	extension := strings.ToLower(filepath.Ext(source))
	if extension != ".yaml" && extension != ".yml" {
		return Fragment{}, fmt.Errorf(
			"unsupported contract file: %s (expected .yaml or .yml)",
			source,
		)
	}

	decoded := Fragment{Source: source}

	trimmed := bytes.TrimSpace([]byte(content))
	if len(trimmed) == 0 {
		return decoded, nil
	}

	file, err := parser.ParseBytes(trimmed, 0)
	if err != nil {
		return Fragment{}, malformedContractFile(source, err)
	}

	if len(file.Docs) > 1 {
		return Fragment{}, malformedContractFile(source, errors.New("multiple documents are not supported"))
	}

	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return decoded, nil
	}

	body := file.Docs[0].Body

	if usesAnchors(body) {
		return Fragment{}, malformedContractFile(source, errors.New("anchors and aliases are not supported"))
	}

	if err := yaml.NodeToValue(body, &decoded.Document); err != nil {
		return Fragment{}, malformedContractFile(source, err)
	}

	return decoded, nil
}

func malformedContractFile(source string, err error) error {
	return fmt.Errorf("malformed contract file: %s: %v", source, err)
}

type anchorFinder struct {
	found bool
}

func (this *anchorFinder) Visit(node ast.Node) ast.Visitor {
	switch node.(type) {
	case *ast.AnchorNode, *ast.AliasNode:
		this.found = true

		return nil
	}

	return this
}

func usesAnchors(body ast.Node) bool {
	finder := &anchorFinder{}
	ast.Walk(finder, body)

	return finder.found
}

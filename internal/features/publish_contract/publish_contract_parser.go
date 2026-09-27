package publish_contract

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/bidirekt/broker/internal/features/publish_contract/contract"
)

func decodeFragment(fragment ContractFragment) (contract.Fragment, error) {
	extension := strings.ToLower(filepath.Ext(fragment.Source))
	if extension != ".yaml" && extension != ".yml" {
		return contract.Fragment{}, fmt.Errorf(
			"unsupported contract file: %s (expected .yaml or .yml)",
			fragment.Source,
		)
	}

	decoded := contract.Fragment{Source: fragment.Source}

	content := bytes.TrimSpace([]byte(fragment.Content))
	if len(content) == 0 {
		return decoded, nil
	}

	file, err := parser.ParseBytes(content, 0)
	if err != nil {
		return contract.Fragment{}, malformedContractFile(fragment.Source, err)
	}

	if len(file.Docs) > 1 {
		return contract.Fragment{}, malformedContractFile(fragment.Source, errors.New("multiple documents are not supported"))
	}

	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return decoded, nil
	}

	body := file.Docs[0].Body

	if usesAnchors(body) {
		return contract.Fragment{}, malformedContractFile(fragment.Source, errors.New("anchors and aliases are not supported"))
	}

	if err := yaml.NodeToValue(body, &decoded.Document); err != nil {
		return contract.Fragment{}, malformedContractFile(fragment.Source, err)
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

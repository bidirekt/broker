package schemamapper_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/mapper/schemamapper"
	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestToPropertyModels_Root_IsDollar(t *testing.T) {
	properties := schemamapper.ToPropertyModels(nil, contract.Schema{Type: "string"})

	assert.Equal(t, map[string]model.Property{
		"$": {Path: "$", Type: "string", Optional: false},
	}, properties)
}

func TestToPropertyModels_Property_AppendsDottedName(t *testing.T) {
	properties := schemamapper.ToPropertyModels(nil, contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"x": {Type: "integer"},
		},
	})

	assert.Equal(t, map[string]model.Property{
		"$":   {Path: "$", Type: "object", Optional: false},
		"$.x": {Path: "$.x", Type: "integer", Optional: false},
	}, properties)
}

func TestToPropertyModels_ArrayItem_SuffixesBrackets(t *testing.T) {
	properties := schemamapper.ToPropertyModels(nil, contract.Schema{
		Type: "object",
		Properties: map[string]contract.Schema{
			"x": {Type: "array", Items: &contract.Schema{Type: "string"}},
		},
	})

	assert.Equal(t, map[string]model.Property{
		"$":     {Path: "$", Type: "object", Optional: false},
		"$.x":   {Path: "$.x", Type: "array", Optional: false},
		"$.x[]": {Path: "$.x[]", Type: "string", Optional: false},
	}, properties)
}

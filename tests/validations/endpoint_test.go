package validations_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/validations"
	"github.com/stretchr/testify/assert"
)

func TestEndpoint_AcceptsWellFormedPaths(t *testing.T) {
	for _, endpoint := range []string{"/", "/users", "/users/*", "/users/*/orders/*", "/Pets/v1.2/a-b_c~d"} {
		t.Run(endpoint, func(t *testing.T) {
			assert.NoError(t, validations.Endpoint(endpoint))
		})
	}
}

func TestEndpoint_RejectsMalformedPaths(t *testing.T) {
	for _, endpoint := range []string{
		"", "users", "/users//orders", "/users/;/orders", "/a\x00b", "/a\tb", "/a\x7fb",
		"/a b", "/pets?x", "/a%20b", "/a+b", "/a@b", "/café",
	} {
		t.Run(endpoint, func(t *testing.T) {
			assert.EqualError(t, validations.Endpoint(endpoint), "malformed path")
		})
	}
}

func TestEndpoint_RejectsNamedDynamicSegments(t *testing.T) {
	for _, endpoint := range []string{
		"/users/{id}", "/users/*id",
		"/pets/:petId", "/pets/<id>", "/pets/[id]", "/v1/items:batchGet", "/pets/{petId};v=2",
	} {
		t.Run(endpoint, func(t *testing.T) {
			assert.EqualError(t, validations.Endpoint(endpoint), "dynamic path segments must use *")
		})
	}
}

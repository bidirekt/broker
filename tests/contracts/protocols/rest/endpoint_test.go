package rest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
)

func TestNormalizeEndpoint_TrimsTrailingSlash(t *testing.T) {
	assert.Equal(t, "/users", restcontracts.NormalizeEndpoint("/users/"))
}

func TestNormalizeEndpoint_KeepsRoot(t *testing.T) {
	assert.Equal(t, "/", restcontracts.NormalizeEndpoint("/"))
}

func TestNormalizeEndpoint_LeavesPlainEndpointAlone(t *testing.T) {
	assert.Equal(t, "/users", restcontracts.NormalizeEndpoint("/users"))
}

func TestNormalizeEndpoint_KeepsTheSlashThatClosesAnEmptySegment(t *testing.T) {
	for _, endpoint := range []string{"//", "/users//"} {
		t.Run(endpoint, func(t *testing.T) {
			assert.Equal(t, endpoint, restcontracts.NormalizeEndpoint(endpoint))
		})
	}
}

func TestCheckEndpoint_AcceptsWellFormedPaths(t *testing.T) {
	for _, endpoint := range []string{"/", "/users", "/users/*", "/users/*/orders/*", "/Pets/v1.2/a-b_c~d"} {
		t.Run(endpoint, func(t *testing.T) {
			assert.NoError(t, restcontracts.CheckEndpoint(endpoint))
		})
	}
}

func TestCheckEndpoint_RejectsMalformedPaths(t *testing.T) {
	for _, endpoint := range []string{
		"", "users", "/users//orders", "/users/;/orders", "/a\x00b", "/a\tb", "/a\x7fb",
		"/a b", "/pets?x", "/a%20b", "/a+b", "/a@b", "/café",
	} {
		t.Run(endpoint, func(t *testing.T) {
			assert.EqualError(t, restcontracts.CheckEndpoint(endpoint), "malformed path")
		})
	}
}

func TestCheckEndpoint_RejectsNamedDynamicSegments(t *testing.T) {
	for _, endpoint := range []string{
		"/users/{id}", "/users/*id",
		"/pets/:petId", "/pets/<id>", "/pets/[id]", "/v1/items:batchGet", "/pets/{petId};v=2",
	} {
		t.Run(endpoint, func(t *testing.T) {
			assert.EqualError(t, restcontracts.CheckEndpoint(endpoint), "dynamic path segments must use *")
		})
	}
}

func TestSuggestEndpoint(t *testing.T) {
	for _, test := range []struct {
		endpoint   string
		suggestion string
	}{
		{endpoint: "/users/{id}", suggestion: "/users/*"},
		{endpoint: "/pets/:petId", suggestion: "/pets/*"},
		{endpoint: "/pets/<id>", suggestion: "/pets/*"},
		{endpoint: "/pets/[id]", suggestion: "/pets/*"},
		{endpoint: "/pets/{id}/", suggestion: "/pets/*"},
		{endpoint: "/owners/{ownerId}/pets/:petId", suggestion: "/owners/*/pets/*"},
		{endpoint: "/users/*/{orderId}", suggestion: "/users/*/*"},
		{endpoint: "/pets/p*", suggestion: ""},
		{endpoint: "/users/*id", suggestion: ""},
		{endpoint: "/v1/items:batchGet", suggestion: ""},
		{endpoint: "/pets/{petId};v=2", suggestion: ""},
		{endpoint: "/pets/{}", suggestion: ""},
		{endpoint: "/pets/:", suggestion: ""},
		{endpoint: "pets/{id}", suggestion: ""},
		{endpoint: "/a b/{id}", suggestion: ""},
		{endpoint: "/{id}/a b", suggestion: ""},
		{endpoint: "/pets/{a\x00b}", suggestion: ""},
		{endpoint: "/users//{id}", suggestion: ""},
		{endpoint: "/users/*", suggestion: ""},
		{endpoint: "/users/", suggestion: ""},
	} {
		t.Run(test.endpoint, func(t *testing.T) {
			assert.Equal(t, test.suggestion, restcontracts.SuggestEndpoint(test.endpoint))
		})
	}
}

package model

import "github.com/guregu/null"

// ResourceBody is one declaration of a resource: every consumer module that declares the resource adds one.
type ResourceBody struct {
	Source     string
	Namespace  string      // empty for the `~` body
	SchemaName null.String // invalid for the `~` body
}

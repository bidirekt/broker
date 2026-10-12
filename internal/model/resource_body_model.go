package model

import "github.com/guregu/null"

// ResourceBody is one declaration of a resource: every consumer module that declares the resource adds one.
type ResourceBody struct {
	Source     string
	SchemaName null.String // invalid for the `~` body
}

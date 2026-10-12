package model

type ChangeKind string

const (
	ChangeAdded    ChangeKind = "added"
	ChangeModified ChangeKind = "modified"
	ChangeRemoved  ChangeKind = "removed"
)

type ContractDiff struct {
	Resources map[string]ChangeKind // resource hash → added | removed
	Schemas   map[string]SchemaChange
}

// SchemaChange is modified when the schema stays reachable and only its fields change.
type SchemaChange struct {
	Kind   ChangeKind
	Fields map[string]FieldChange
}

type FieldChange struct {
	Kind   ChangeKind
	Before Field
	After  Field
}

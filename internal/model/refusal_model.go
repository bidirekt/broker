package model

// Refusal is a stored-state reason not to take a contract; Source is the file the refused key came from.
type Refusal struct {
	Code    string
	Path    string
	Source  string
	Details map[string]any
}

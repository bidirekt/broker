package graphql

import "github.com/bidirekt/broker/internal/model"

// ReportTree keeps no break yet: no GraphQL consumer is declared, so no GraphQL resource breaks.
type ReportTree struct{}

func (this ReportTree) ReportKey() string {
	return ReportKey
}

func (this ReportTree) IsEmpty() bool {
	return true
}

func (this ReportTree) Append(map[string]string, model.ReportBreak) {}

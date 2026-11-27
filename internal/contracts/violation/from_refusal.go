package violation

import "github.com/bidirekt/broker/internal/model"

func FromRefusal(refusal model.Refusal) Violation {
	return Violation{ErrorCode: refusal.Code, Path: refusal.Path, Source: refusal.Source, Details: refusal.Details}
}

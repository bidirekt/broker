package validate_contract

import (
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

const (
	ContractValidatedMessage        string = "contract validated successfully"
	ContractInvalidInputMessage     string = "contract invalid input"
	ContractValidationFailedMessage string = "contract validation failed"
	ParticipantNotFoundMessage      string = "participant not found"
	EnvironmentNotFoundMessage      string = "environment not found"
)

type ValidateContractRequestBody struct {
	Participant string               `json:"participant"`
	Environment string               `json:"environment"`
	Contracts   []model.ContractFile `json:"contracts"`
}

type ValidateContractResponseBody struct {
	Message     string                                        `json:"message"`
	Participant string                                        `json:"participant"`
	Environment string                                        `json:"environment"`
	Deployable  bool                                          `json:"deployable"`
	Results     map[string]compatibility_checker.Hierarchical `json:"results"`
}

type ValidateContractErrorResponseBody struct {
	Message string `json:"message"`
}

type ValidateContractValidationResponseBody struct {
	Message    string                `json:"message"`
	Violations []violation.Violation `json:"violations"`
}

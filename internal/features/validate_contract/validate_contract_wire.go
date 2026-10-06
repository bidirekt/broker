package validate_contract

import (
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

const (
	ContractValidated        string = "contract validated successfully"
	ContractInvalidInput     string = "contract invalid input"
	ContractValidationFailed string = "contract validation failed"
	ParticipantNotFound      string = "participant not found"
	EnvironmentNotFound      string = "environment not found"
)

type ValidateContractRequestBody struct {
	Participant string               `json:"participant"`
	Environment string               `json:"environment"`
	Contracts   []contractfiles.File `json:"contracts"`
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

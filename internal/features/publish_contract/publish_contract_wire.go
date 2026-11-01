package publish_contract

import (
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

const (
	ContractPublishSuccessfulMessage   string = "contract publish successful"
	ContractInvalidInputMessage        string = "contract invalid input"
	ContractValidationFailedMessage    string = "contract validation failed"
	ContractVersionConflictMessage     string = "contract version already exists with different content"
	ContractParticipantNotFoundMessage string = "contract participant not found"
)

type PublishContractRequestBody struct {
	ServiceName string               `json:"participant"`
	Version     string               `json:"version"`
	Contracts   []contractfiles.File `json:"contracts"`
}

type PublishContractResponseBody struct {
	Message string `json:"message"`
}

type PublishContractValidationResponseBody struct {
	Message    string                `json:"message"`
	Violations []violation.Violation `json:"violations"`
}

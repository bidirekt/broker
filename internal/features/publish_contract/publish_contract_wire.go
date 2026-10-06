package publish_contract

import (
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

const (
	ContractPublishSuccessful   string = "contract publish successful"
	ContractInvalidInput        string = "contract invalid input"
	ContractValidationFailed    string = "contract validation failed"
	ContractVersionConflict     string = "contract version already exists with different content"
	ContractParticipantNotFound string = "contract participant not found"
	ContractPublishFailed       string = "contract publish failed"
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

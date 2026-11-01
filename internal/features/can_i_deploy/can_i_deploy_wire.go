package can_i_deploy

import (
	"github.com/bidirekt/broker/internal/contracts/compatibility_checker"
)

const ContractCheckedMessage = "contract checked successfully"
const CanIDeployInvalidInputMessage = "can-i-deploy invalid input"
const ContractNotFoundMessage = "contract not found"
const ParticipantNotFoundMessage = "participant not found"
const EnvironmentNotFoundMessage = "environment not found"

type CanIDeployRequestBody struct {
	Participant string `json:"participant"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
}

type CanIDeployResponseBody struct {
	Message     string                                        `json:"message"`
	Participant string                                        `json:"participant"`
	Version     string                                        `json:"version"`
	Environment string                                        `json:"environment"`
	Deployable  bool                                          `json:"deployable"`
	Results     map[string]compatibility_checker.Hierarchical `json:"results"`
}

type CanIDeployErrorResponseBody struct {
	Message string `json:"message"`
}

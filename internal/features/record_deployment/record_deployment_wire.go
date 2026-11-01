package record_deployment

const DeploymentRecordedMessage string = "deployment recorded"
const DeploymentRolledBackMessage string = "deployment rolled back"
const DeploymentInvalidInputMessage string = "deployment invalid input"
const ParticipantNotFoundMessage string = "participant not found"
const VersionNotFoundMessage string = "version not found"
const EnvironmentNotFoundMessage string = "environment not found"

type RecordDeploymentRequestBody struct {
	Participant string `json:"participant"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
}

type RecordDeploymentResponseBody struct {
	Message string `json:"message"`
}

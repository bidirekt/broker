package record_deployment

const DeploymentRecorded string = "deployment recorded"
const DeploymentRolledBack string = "deployment rolled back"
const DeploymentInvalidInput string = "deployment invalid input"
const ParticipantNotFound string = "participant not found"
const VersionNotFound string = "version not found"
const EnvironmentNotFound string = "environment not found"

type RecordDeploymentRequestBody struct {
	Participant string `json:"participant"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
}

type RecordDeploymentResponseBody struct {
	Message string `json:"message"`
}

package record_deployment

import (
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

type RecordDeploymentHandler struct {
	deploymentRepository  *repository.DeploymentRepository
	participantRepository *repository.ParticipantRepository
	contractRepository    *repository.ContractRepository
	environmentRepository *repository.EnvironmentRepository
}

func NewRecordDeploymentHandler(
	deploymentRepository *repository.DeploymentRepository,
	participantRepository *repository.ParticipantRepository,
	contractRepository *repository.ContractRepository,
	environmentRepository *repository.EnvironmentRepository,
) *RecordDeploymentHandler {
	return &RecordDeploymentHandler{
		deploymentRepository:  deploymentRepository,
		participantRepository: participantRepository,
		contractRepository:    contractRepository,
		environmentRepository: environmentRepository,
	}
}

func (this *RecordDeploymentHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &RecordDeploymentRequestBody{}
	if err := ctx.Bind().JSON(requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	if requestBody.Participant == "" || requestBody.Version == "" || requestBody.Environment == "" {
		return this.respondInvalidInput(ctx)
	}

	participant, exists := this.participantRepository.FindByName(ctx.Context(), requestBody.Participant)
	if !exists {
		return this.respondParticipantNotFound(ctx)
	}

	if !this.contractRepository.HasContractForVersion(ctx.Context(), participant.ID, requestBody.Version) {
		return this.respondVersionNotFound(ctx)
	}

	environment, exists := this.environmentRepository.FindByName(ctx.Context(), requestBody.Environment)
	if !exists {
		return this.respondEnvironmentNotFound(ctx)
	}

	this.deploymentRepository.Insert(ctx.Context(), model.NewDeployment(participant, requestBody.Version, environment))

	return ctx.Status(fiber.StatusOK).JSON(RecordDeploymentResponseBody{
		Message: DeploymentRecorded,
	})
}

func (this *RecordDeploymentHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(RecordDeploymentResponseBody{
		Message: DeploymentInvalidInput,
	})
}

func (this *RecordDeploymentHandler) respondEnvironmentNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RecordDeploymentResponseBody{
		Message: EnvironmentNotFound,
	})
}

func (this *RecordDeploymentHandler) respondParticipantNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RecordDeploymentResponseBody{
		Message: ParticipantNotFound,
	})
}

func (this *RecordDeploymentHandler) respondVersionNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(RecordDeploymentResponseBody{
		Message: VersionNotFound,
	})
}

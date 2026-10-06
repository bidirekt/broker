package publish_contract

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/gofiber/fiber/v3"
)

type PublishContractHandler struct {
	contractRepository    *repository.ContractRepository
	participantRepository *repository.ParticipantRepository
}

func NewPublishContractHandler(
	contractRepository *repository.ContractRepository,
	participantRepository *repository.ParticipantRepository,
) *PublishContractHandler {
	return &PublishContractHandler{
		contractRepository:    contractRepository,
		participantRepository: participantRepository,
	}
}

func (this *PublishContractHandler) Handle(ctx fiber.Ctx) error {
	requestBody := &PublishContractRequestBody{}
	if err := json.Unmarshal(ctx.Body(), requestBody); err != nil {
		return this.respondInvalidInput(ctx)
	}

	serviceName := strings.TrimSpace(requestBody.ServiceName)
	version := strings.TrimSpace(requestBody.Version)
	if serviceName == "" || version == "" || len(requestBody.Contracts) == 0 {
		return this.respondInvalidInput(ctx)
	}

	fragments, shapeViolations, err := contractfiles.ToFragments(requestBody.Contracts)
	if errors.Is(err, contractfiles.ErrBlankSource) {
		return this.respondInvalidInput(ctx)
	}

	if err != nil {
		return this.respondBadRequest(ctx, err)
	}

	if len(shapeViolations) > 0 {
		return this.respondValidationFailed(ctx, shapeViolations)
	}

	participant, exists := this.participantRepository.FindByName(ctx.Context(), serviceName)
	if !exists {
		return this.respondParticipantNotFound(ctx)
	}

	contractContent, _ := json.Marshal(requestBody.Contracts)

	uploadedContract, violations, err := contractfiles.ToUploadedContract(fragments, participant, version, string(contractContent))
	if len(violations) > 0 {
		return this.respondValidationFailed(ctx, violations)
	}

	if err != nil {
		return this.respondPublishFailed(ctx)
	}

	if existing, found := this.contractRepository.LoadChecksumForVersion(ctx.Context(), uploadedContract.ParticipantID, version); found {
		if existing == uploadedContract.Checksum() {
			return this.respondSuccess(ctx)
		}
		return this.respondVersionConflict(ctx)
	}

	if this.contractRepository.AliasVersionToSnapshot(ctx.Context(), uploadedContract) {
		return this.respondSuccess(ctx)
	}

	this.upsert(ctx, uploadedContract)

	return this.respondSuccess(ctx)
}

func (this *PublishContractHandler) respondParticipantNotFound(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusNotFound).JSON(PublishContractResponseBody{
		Message: ContractParticipantNotFound,
	})
}

func (this *PublishContractHandler) upsert(ctx fiber.Ctx, uploadedContract *model.UploadedContract) {
	current, existing := this.contractRepository.GetLatestContractByName(ctx.Context(), uploadedContract.ParticipantName)
	if !existing {
		this.contractRepository.Create(ctx.Context(), uploadedContract)

		return
	}

	this.contractRepository.Update(ctx.Context(), uploadedContract, current, contract_differ.DiffContracts(current, uploadedContract))
}

func (this *PublishContractHandler) respondBadRequest(ctx fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractResponseBody{
		Message: err.Error(),
	})
}

func (this *PublishContractHandler) respondValidationFailed(ctx fiber.Ctx, violations []violation.Violation) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractValidationResponseBody{
		Message:    ContractValidationFailed,
		Violations: violations,
	})
}

func (this *PublishContractHandler) respondPublishFailed(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusInternalServerError).JSON(PublishContractResponseBody{
		Message: ContractPublishFailed,
	})
}

func (this *PublishContractHandler) respondInvalidInput(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusBadRequest).JSON(PublishContractResponseBody{
		Message: ContractInvalidInput,
	})
}

func (this *PublishContractHandler) respondVersionConflict(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusConflict).JSON(PublishContractResponseBody{
		Message: ContractVersionConflict,
	})
}

func (this *PublishContractHandler) respondSuccess(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(PublishContractResponseBody{
		Message: ContractPublishSuccessful,
	})
}

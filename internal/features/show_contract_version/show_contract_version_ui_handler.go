package show_contract_version

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_contract_version.html
var pageFiles embed.FS

type ShowContractVersionUiHandler struct {
	useCase *ShowContractVersionUseCase
}

func NewShowContractVersionUiHandler(useCase *ShowContractVersionUseCase) *ShowContractVersionUiHandler {
	return &ShowContractVersionUiHandler{useCase: useCase}
}

func (this *ShowContractVersionUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	version, ok := pages.ParsePathSegment(ctx.Params(versionParam))
	if !ok {
		return respond.NotFound(ctx)
	}

	checksPage, ok := pages.RequestedPage(ctx, checksPageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowContractVersionInput{
		ParticipantName: ctx.Params(participantNameParam),
		Version:         version,
		ChecksPage:      checksPage,
	})
	if output.Status == ContractVersionNotFound {
		return respond.NotFound(ctx)
	}

	checksPager, ok := pages.NewPager(ctx, checksPageParam, checksPage, output.ChecksTotal)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showContractVersionTemplate, newShowContractVersionPage(output, checksPager))
}

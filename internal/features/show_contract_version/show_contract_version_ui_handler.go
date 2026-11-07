package show_contract_version

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
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

	participantName := ctx.Params(participantNameParam)
	if validations.ParticipantName(participantName) != nil {
		return respond.NotFound(ctx)
	}

	version, ok := pages.ParsePathSegment(ctx.Params(versionParam))
	if !ok {
		return respond.NotFound(ctx)
	}

	deploymentsPage, ok := pages.RequestedPage(ctx, deploymentsPageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	checksPage, ok := pages.RequestedPage(ctx, checksPageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowContractVersionInput{
		ParticipantName: participantName,
		Version:         version,
		DeploymentsPage: deploymentsPage,
		ChecksPage:      checksPage,
	})
	if output.Status == ContractVersionNotFound {
		return respond.NotFound(ctx)
	}

	deploymentsPager, ok := pages.NewPager(ctx, deploymentsPageParam, deploymentsPage, output.DeploymentsTotal)
	if !ok {
		return respond.NotFound(ctx)
	}

	checksPager, ok := pages.NewPager(ctx, checksPageParam, checksPage, output.ChecksTotal)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showContractVersionTemplate, newShowContractVersionPage(output, deploymentsPager, checksPager))
}

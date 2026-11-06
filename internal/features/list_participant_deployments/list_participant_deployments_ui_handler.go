package list_participant_deployments

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/validations"
	"github.com/gofiber/fiber/v3"
	"github.com/guregu/null"
)

//go:embed list_participant_deployments.html
var pageFiles embed.FS

type ListParticipantDeploymentsUiHandler struct {
	useCase *ListParticipantDeploymentsUseCase
}

func NewListParticipantDeploymentsUiHandler(useCase *ListParticipantDeploymentsUseCase) *ListParticipantDeploymentsUiHandler {
	return &ListParticipantDeploymentsUiHandler{useCase: useCase}
}

func (this *ListParticipantDeploymentsUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	participantName := ctx.Params(participantNameParam)
	if validations.ParticipantName(participantName) != nil {
		return respond.NotFound(ctx)
	}

	environmentID, ok := requestedEnvironmentID(ctx)
	if !ok {
		return respond.NotFound(ctx)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListParticipantDeploymentsInput{
		ParticipantName: participantName,
		EnvironmentID:   environmentID,
		Page:            page,
	})
	if output.Status != DeploymentsListed {
		return respond.NotFound(ctx)
	}

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Total)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, listParticipantDeploymentsTemplate, newListParticipantDeploymentsPage(output, pager))
}

func requestedEnvironmentID(ctx fiber.Ctx) (environmentID null.Int, ok bool) {
	query := ctx.Request().URI().QueryArgs()
	if !query.Has(environmentParam) {
		return null.Int{}, true
	}

	id, ok := pages.ParsePositiveInteger(string(query.Peek(environmentParam)))

	return null.IntFrom(id), ok
}

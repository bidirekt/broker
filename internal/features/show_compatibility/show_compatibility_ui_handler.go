package show_compatibility

import (
	"embed"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

//go:embed show_compatibility.html
var pageFiles embed.FS

type ShowCompatibilityUiHandler struct {
	useCase *ShowCompatibilityUseCase
}

func NewShowCompatibilityUiHandler(useCase *ShowCompatibilityUseCase) *ShowCompatibilityUiHandler {
	return &ShowCompatibilityUiHandler{useCase: useCase}
}

func (this *ShowCompatibilityUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.Allows(ctx, model.PermissionRead) {
		return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	query := ctx.Request().URI().QueryArgs()
	versions := defaultVersions
	if query.Has(versionsParam) {
		versions = string(query.Peek(versionsParam))
	}
	filter, ok := versionsFilter(versions, page)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ShowCompatibilityInput{
		Participant:      string(query.Peek(participantParam)),
		ParticipantGiven: query.Has(participantParam),
		Versions:         filter,
	})
	if output.Status == ParticipantNotFound {
		return respond.NotFound(ctx)
	}

	pager := pages.Pager{Page: page}
	if filter.All {
		pager, ok = pages.NewPagerOfSize(ctx, pageParam, page, output.VersionsTotal, allVersionsPageSize)
		if !ok {
			return respond.NotFound(ctx)
		}
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, showCompatibilityTemplate, newShowCompatibilityPage(output, versions, pager))
}

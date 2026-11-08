package list_user_api_keys

import (
	"embed"
	"time"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
)

//go:embed list_user_api_keys.html
var pageFiles embed.FS

type ListUserAPIKeysUiHandler struct {
	useCase *ListUserAPIKeysUseCase
}

func NewListUserAPIKeysUiHandler(useCase *ListUserAPIKeysUseCase) *ListUserAPIKeysUiHandler {
	return &ListUserAPIKeysUiHandler{useCase: useCase}
}

func (this *ListUserAPIKeysUiHandler) Handle(ctx fiber.Ctx) error {
	if !access.IsAdmin(ctx) {
		return this.respondForbidden(ctx)
	}

	userID, ok := pages.ParsePositiveInteger(ctx.Params(userIDParam))
	if !ok {
		return this.respondUserNotFound(ctx)
	}

	page, ok := pages.RequestedPage(ctx, pageParam)
	if !ok {
		return respond.NotFound(ctx)
	}

	output := this.useCase.Execute(ctx.Context(), ListUserAPIKeysInput{UserID: userID, Page: page})
	switch output.Status {
	case UserNotFound:
		return this.respondUserNotFound(ctx)
	case TargetIsTheAdmin:
		return this.respondForbidden(ctx)
	}

	pager, ok := pages.NewPager(ctx, pageParam, page, output.Total)
	if !ok {
		return respond.NotFound(ctx)
	}

	return pages.Render(ctx, fiber.StatusOK, pageFiles, listUserAPIKeysTemplate, ListUserAPIKeysPage{
		UserID:  output.User.ID,
		Email:   output.User.Email,
		APIKeys: toAPIKeyRows(output.Keys, output.User.Permission, time.Now()),
		Pager:   pager,
	})
}

func (this *ListUserAPIKeysUiHandler) respondForbidden(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusForbidden, access.NoAccessMessage)
}

func (this *ListUserAPIKeysUiHandler) respondUserNotFound(ctx fiber.Ctx) error {
	return respond.Problem(ctx, fiber.StatusNotFound, UserNotFoundMessage)
}

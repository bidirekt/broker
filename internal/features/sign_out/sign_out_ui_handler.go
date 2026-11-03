package sign_out

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

type SignOutUiHandler struct {
	useCase *SignOutUseCase
}

func NewSignOutUiHandler(useCase *SignOutUseCase) *SignOutUiHandler {
	return &SignOutUiHandler{useCase: useCase}
}

func (this *SignOutUiHandler) SignOut(ctx fiber.Ctx) error {
	session, _ := access.SessionFrom(ctx)
	this.useCase.Execute(ctx.Context(), SignOutInput{SessionIDHash: session.IDHash})

	sessions.ExpireCookie(ctx)

	return pages.Redirect(ctx, signInPath)
}

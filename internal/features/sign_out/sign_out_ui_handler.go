package sign_out

import (
	"github.com/bidirekt/broker/internal/components"
	"github.com/gofiber/fiber/v3"
)

type SignOutUiHandler struct {
	useCase *SignOutUseCase
}

func NewSignOutUiHandler(useCase *SignOutUseCase) *SignOutUiHandler {
	return &SignOutUiHandler{useCase: useCase}
}

func (this *SignOutUiHandler) SignOut(ctx fiber.Ctx) error {
	session, _ := components.SessionFrom(ctx)
	this.useCase.Execute(ctx.Context(), SignOutInput{SessionIDHash: session.IDHash})

	components.ExpireSessionCookie(ctx)

	return components.Redirect(ctx, signInPath)
}

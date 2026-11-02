package sign_out

import (
	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/sessions"
	"github.com/gofiber/fiber/v3"
)

type SignOutUiHandler struct {
	useCase       *SignOutUseCase
	sessionKeeper *sessions.Keeper
}

func NewSignOutUiHandler(useCase *SignOutUseCase, sessionKeeper *sessions.Keeper) *SignOutUiHandler {
	return &SignOutUiHandler{useCase: useCase, sessionKeeper: sessionKeeper}
}

func (this *SignOutUiHandler) SignOut(ctx fiber.Ctx) error {
	session, _ := access.SessionFrom(ctx)
	this.useCase.Execute(ctx.Context(), SignOutInput{SessionIDHash: session.IDHash})

	this.sessionKeeper.ExpireCookie(ctx)

	return pages.Redirect(ctx, signInPath)
}

package sessions

import (
	"errors"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/gofiber/fiber/v3"
)

func (this *Keeper) SetMessage(ctx fiber.Ctx, text string) {
	session, ok := access.SessionFrom(ctx)
	if !ok {
		panic(errors.New("error setting session message: the request has no session"))
	}

	this.sessions.SetMessage(ctx.Context(), session.IDHash, text)
}

package sessions

import (
	"errors"

	"github.com/bidirekt/broker/internal/components/access"
	"github.com/bidirekt/broker/internal/model"
	"github.com/gofiber/fiber/v3"
)

func (this *Keeper) AddMessage(ctx fiber.Ctx, kind model.MessageKind, text string) {
	session, ok := access.SessionFrom(ctx)
	if !ok {
		panic(errors.New("error adding session message: the request has no session"))
	}

	this.sessions.AddMessage(ctx.Context(), session.IDHash, model.NewSessionMessage(kind, text))
}

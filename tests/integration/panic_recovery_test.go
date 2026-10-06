package integration_test

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

const recoveredParticipantBody = `{"participant":"recovered_service"}`

func (this *IntegrationSuite) TestPanicInHandler_Returns500AndServerKeepsServing() {
	this.Components.Server.Post("/__panic", func(fiber.Ctx) error {
		panic("handler exploded")
	})

	status, body := this.post("/__panic", `{}`)
	this.Equal(http.StatusInternalServerError, status)
	this.JSONEq(`{"message":"internal error"}`, body)

	status, body = this.post("/api/participants", recoveredParticipantBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant created"}`, body)
}

func (this *IntegrationSuite) TestSqlFailureInRepository_Returns500AndServerKeepsServing() {
	_, err := this.Pool.Exec(context.Background(), `ALTER TABLE participants RENAME TO participants_moved_away`)
	this.Require().NoError(err)

	defer func() {
		_, _ = this.Pool.Exec(context.Background(), `ALTER TABLE IF EXISTS participants_moved_away RENAME TO participants`)
	}()

	status, body := this.post("/api/participants", recoveredParticipantBody)
	this.Equal(http.StatusInternalServerError, status)
	this.JSONEq(`{"message":"internal error"}`, body)

	for _, internal := range []string{"participants", "SELECT", "INSERT", ".go", "pgx", "panic"} {
		this.NotContains(body, internal)
	}

	_, err = this.Pool.Exec(context.Background(), `ALTER TABLE participants_moved_away RENAME TO participants`)
	this.Require().NoError(err)

	status, body = this.post("/api/participants", recoveredParticipantBody)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant created"}`, body)
}

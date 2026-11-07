package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

const (
	renamePetsBody   = `{"participant":"pets_service"}`
	renameOrdersBody = `{"participant":"orders_service"}`
)

const renameV1ContractBody = `
{
  "provides": {
    "rest": {
      "/things": {
        "get": {
          "responses": {
            "200": "Thing"
          }
        }
      }
    }
  },
  "schemas": {
    "Thing": {
      "type": "object",
      "properties": {
        "id": { "type": "string" }
      }
    }
  }
}`

const renameNewcomerContractBody = `
{
  "provides": {
    "rest": {
      "/pets": {
        "get": {
          "responses": {
            "204": null
          }
        }
      }
    }
  }
}`

func (this *IntegrationSuite) TestRenameParticipant_NewNameNotSnakeCase_Rejected() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"Pets-Service"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"participant name must be snake_case"}`, body)

	// the participant is still findable under its old name: nothing was renamed
	this.Positive(this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_BeforePublishPreservesIdentity() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant renamed"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("orders_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_AfterPublishIsRejected() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody}))
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"participant has published contracts, create a new participant and switch every consumer's consumes to it"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_OntoExistingNameIsRejectedNeverMerged() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	status, _ = this.post("/api/participants", renameOrdersBody)
	this.Require().Equal(http.StatusOK, status)

	petsID := this.renameParticipantID("pets_service")
	ordersID := this.renameParticipantID("orders_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"participant already exists"}`, body)

	this.Equal(2, this.countRows("participants"))
	this.Equal(petsID, this.renameParticipantID("pets_service"))
	this.Equal(ordersID, this.renameParticipantID("orders_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_UnknownParticipantReturns404() {
	status, body := this.post("/api/participants/rename", `{"oldName":"unknown_service","newName":"orders_service"}`)
	this.Equal(http.StatusNotFound, status)
	this.JSONEq(`{"message":"participant not found"}`, body)

	this.Equal(0, this.countRows("participants"))
}

func (this *IntegrationSuite) TestRenameParticipant_MissingNewNameReturns400() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service"}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"participant invalid input"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.NotZero(this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_EmptyNewNameReturns400() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":""}`)
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"participant invalid input"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.NotZero(this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_PaddedOldNameReturns400() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	cases := map[string]string{
		"trailing space in old name": `{"oldName":"pets_service ","newName":"orders_service"}`,
		"leading tab in old name":    `{"oldName":"\tpets_service","newName":"orders_service"}`,
	}

	for name, body := range cases {
		status, response := this.post("/api/participants/rename", body)
		this.Equal(http.StatusBadRequest, status, name)
		this.JSONEq(`{"message":"participant invalid input"}`, response, name)
	}

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_SameNameIsNoOpSuccess() {
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")

	status, body := this.post("/api/participants/rename", `{"oldName":"pets_service","newName":"pets_service"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"participant renamed"}`, body)

	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_AFirstPublishWaitingBehindIt_AnswersNotFoundAndWritesNothing() {
	ctx := context.Background()
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")
	publish := this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody})

	sameNewName, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = sameNewName.Rollback(ctx) }()
	_, err = sameNewName.Exec(ctx, `INSERT INTO participants (name) VALUES ('orders_service')`)
	this.Require().NoError(err)

	var renameStatus int
	renameFinished := renameParticipantInBackground(func() {
		renameStatus, _ = this.renameParticipantSend("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	})
	this.renameParticipantAwaitLockWaiters(1)

	var publishStatus int
	var publishResponse string
	publishFinished := renameParticipantInBackground(func() {
		publishStatus, publishResponse = this.renameParticipantSend("/api/contracts", publish)
	})
	this.renameParticipantAwaitLockWaiters(2)

	this.Require().NoError(sameNewName.Rollback(ctx))
	this.Require().Nil(<-renameFinished)
	this.Require().Nil(<-publishFinished)

	this.Equal(http.StatusOK, renameStatus)
	this.Equal(http.StatusNotFound, publishStatus)
	this.JSONEq(`{"message":"contract participant not found"}`, publishResponse)
	this.Equal(0, this.countRows("contracts"))
	this.Equal(originalID, this.renameParticipantID("orders_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_WaitingBehindAFirstPublish_IsRefusedAsPublishedAndKeepsTheName() {
	ctx := context.Background()
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")
	publish := this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody})

	resourceWritesOnHold, err := this.Pool.Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = resourceWritesOnHold.Rollback(ctx) }()
	_, err = resourceWritesOnHold.Exec(ctx, `LOCK TABLE resources IN SHARE MODE`)
	this.Require().NoError(err)

	var publishStatus int
	publishFinished := renameParticipantInBackground(func() {
		publishStatus, _ = this.renameParticipantSend("/api/contracts", publish)
	})
	this.renameParticipantAwaitLockWaiters(1)

	var renameStatus int
	var renameResponse string
	renameFinished := renameParticipantInBackground(func() {
		renameStatus, renameResponse = this.renameParticipantSend("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	})
	this.renameParticipantAwaitLockWaiters(2)

	this.Require().NoError(resourceWritesOnHold.Rollback(ctx))
	this.Require().Nil(<-publishFinished)
	this.Require().Nil(<-renameFinished)

	this.Equal(http.StatusOK, publishStatus)
	this.Equal(http.StatusConflict, renameStatus)
	this.JSONEq(`{"message":"participant has published contracts, create a new participant and switch every consumer's consumes to it"}`, renameResponse)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("participants"))
	this.Equal(originalID, this.renameParticipantID("pets_service"))
}

func (this *IntegrationSuite) TestRenameParticipant_AFirstPublishResumingAfterTheOldNameIsRecreatedAndPublished_AnswersNotFoundAndWritesNothing() {
	ctx := context.Background()
	status, _ := this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	originalID := this.renameParticipantID("pets_service")
	publish := this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameV1ContractBody})
	newcomerPublish := this.publishBody("pets_service", "v1", contractFragment{"api.yaml", renameNewcomerContractBody})

	renameOnHold, err := this.renameParticipantConnect().Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = renameOnHold.Rollback(ctx) }()
	_, err = renameOnHold.Exec(ctx, `SELECT 1 FROM participants WHERE id = $1 FOR UPDATE`, originalID)
	this.Require().NoError(err)

	var renameStatus int
	renameFinished := renameParticipantInBackground(func() {
		renameStatus, _ = this.renameParticipantSend("/api/participants/rename", `{"oldName":"pets_service","newName":"orders_service"}`)
	})
	this.renameParticipantAwaitLockWaiters(1)

	publishOnHold, err := this.renameParticipantConnect().Begin(ctx)
	this.Require().NoError(err)
	defer func() { _ = publishOnHold.Rollback(ctx) }()
	publishOnHoldLocked := renameParticipantInBackground(func() {
		if _, err := publishOnHold.Exec(ctx, `SELECT 1 FROM participants WHERE id = $1 FOR UPDATE`, originalID); err != nil {
			panic(err)
		}
	})
	this.renameParticipantAwaitLockWaiters(2)

	var publishStatus int
	var publishResponse string
	publishFinished := renameParticipantInBackground(func() {
		publishStatus, publishResponse = this.renameParticipantSend("/api/contracts", publish)
	})
	this.renameParticipantAwaitLockWaiters(3)

	this.Require().NoError(renameOnHold.Rollback(ctx))
	this.Require().Nil(<-renameFinished)
	this.Require().Equal(http.StatusOK, renameStatus)
	this.Require().Nil(<-publishOnHoldLocked)
	this.renameParticipantAwaitLockWaiters(1)

	status, _ = this.post("/api/participants", renamePetsBody)
	this.Require().Equal(http.StatusOK, status)
	status, _ = this.post("/api/contracts", newcomerPublish)
	this.Require().Equal(http.StatusOK, status)

	this.Require().NoError(publishOnHold.Rollback(ctx))
	this.Require().Nil(<-publishFinished)

	this.Equal(http.StatusNotFound, publishStatus)
	this.JSONEq(`{"message":"contract participant not found"}`, publishResponse)
	this.Equal(0, this.renameParticipantVersionCount(originalID))
}

func (this *IntegrationSuite) renameParticipantID(name string) int64 {
	var id int64
	err := this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = $1`, name,
	).Scan(&id)
	this.Require().NoError(err)
	return id
}

func renameParticipantInBackground(send func()) (finished <-chan any) {
	panicked := make(chan any, 1)
	go func() {
		defer func() { panicked <- recover() }()
		send()
	}()

	return panicked
}

func (this *IntegrationSuite) renameParticipantAwaitLockWaiters(count int) {
	this.Require().Eventually(func() bool {
		var waiting int
		err := this.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`,
		).Scan(&waiting)
		return err == nil && waiting == count
	}, 10*time.Second, 10*time.Millisecond)
}

func (this *IntegrationSuite) renameParticipantConnect() *pgx.Conn {
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, this.Pool.Config().ConnConfig)
	this.Require().NoError(err)
	this.T().Cleanup(func() { _ = conn.Close(ctx) })

	return conn
}

func (this *IntegrationSuite) renameParticipantVersionCount(participantID int64) int {
	var count int
	err := this.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM contract_versions WHERE participant_id = $1`, participantID,
	).Scan(&count)
	this.Require().NoError(err)
	return count
}

func (this *IntegrationSuite) renameParticipantSend(path, body string) (status int, response string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+this.writerToken)
	req.Header.Set("X-Forwarded-For", this.clientIP)

	resp, err := this.Components.Server.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		panic(err)
	}
	defer func() { _ = resp.Body.Close() }()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return resp.StatusCode, string(bytes)
}

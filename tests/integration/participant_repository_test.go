package integration_test

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

const duplicatedParticipantName = "invoices_service"

func (this *IntegrationSuite) TestParticipantRepository_CreateWithAnExistingName_ReturnsNotCreatedLeavesOneRowAndRaisesNoError() {
	ctx := context.Background()

	var queryErrors []any
	config := this.Pool.Config()
	config.ConnConfig.Tracer = &tracelog.TraceLog{
		LogLevel: tracelog.LogLevelError,
		Logger: tracelog.LoggerFunc(func(_ context.Context, _ tracelog.LogLevel, _ string, data map[string]any) {
			queryErrors = append(queryErrors, data["err"])
		}),
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	this.Require().NoError(err)
	defer pool.Close()

	repo := repository.NewParticipantRepository(pool)

	created := repo.Create(ctx, model.NewParticipant(duplicatedParticipantName))
	this.Require().True(created)

	created = repo.Create(ctx, model.NewParticipant(duplicatedParticipantName))
	this.False(created)

	this.Equal(1, this.countRows("participants"))
	this.Empty(queryErrors)
}

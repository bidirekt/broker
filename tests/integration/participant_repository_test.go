package integration_test

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const duplicatedParticipantName = "invoices_service"

func (this *IntegrationSuite) TestParticipantRepository_CreateWithAnExistingName_ReturnsNotCreatedAndLeavesOneRow() {
	ctx := context.Background()
	repo := repository.NewParticipantRepository(this.Pool)

	created := repo.Create(ctx, model.NewParticipant(duplicatedParticipantName))
	this.Require().True(created)

	created = repo.Create(ctx, model.NewParticipant(duplicatedParticipantName))
	this.False(created)

	this.Equal(1, this.countRows("participants"))
}

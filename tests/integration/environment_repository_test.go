package integration_test

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const duplicatedEnvironmentName = "sandbox"

func (this *IntegrationSuite) TestEnvironmentRepository_CreateWithAnExistingName_ReturnsNotCreatedAndLeavesOneRow() {
	ctx := context.Background()
	repo := repository.NewEnvironmentRepository(this.Pool)

	created := repo.Create(ctx, model.NewEnvironment(duplicatedEnvironmentName))
	this.Require().True(created)

	created = repo.Create(ctx, model.NewEnvironment(duplicatedEnvironmentName))
	this.False(created)

	this.Equal(1, this.countRows("environments"))
}

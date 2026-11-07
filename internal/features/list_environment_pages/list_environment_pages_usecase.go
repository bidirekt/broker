package list_environment_pages

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ListEnvironmentPagesStatus int

const (
	EnvironmentsListed ListEnvironmentPagesStatus = iota
)

type ListEnvironmentPagesInput struct {
	Page int64
}

type ListEnvironmentPagesOutput struct {
	Status       ListEnvironmentPagesStatus
	Environments []*model.EnvironmentListing
	Total        int
}

type ListEnvironmentPagesUseCase struct {
	environmentListingRepository *repository.EnvironmentListingRepository
}

func NewListEnvironmentPagesUseCase(environmentListingRepository *repository.EnvironmentListingRepository) *ListEnvironmentPagesUseCase {
	return &ListEnvironmentPagesUseCase{environmentListingRepository: environmentListingRepository}
}

func (this *ListEnvironmentPagesUseCase) Execute(ctx context.Context, input ListEnvironmentPagesInput) ListEnvironmentPagesOutput {
	environments, total := this.environmentListingRepository.List(ctx, pages.PageSize, pages.Offset(input.Page))

	return ListEnvironmentPagesOutput{Status: EnvironmentsListed, Environments: environments, Total: total}
}

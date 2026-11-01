package list_environments

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
)

type ListEnvironmentsStatus int

const (
	EnvironmentsListed ListEnvironmentsStatus = iota
)

type ListEnvironmentsInput struct{}

type ListEnvironmentsOutput struct {
	Status       ListEnvironmentsStatus
	Environments []string
}

type ListEnvironmentsUseCase struct {
	environmentRepository *repository.EnvironmentRepository
}

func NewListEnvironmentsUseCase(environmentRepository *repository.EnvironmentRepository) *ListEnvironmentsUseCase {
	return &ListEnvironmentsUseCase{environmentRepository: environmentRepository}
}

func (this *ListEnvironmentsUseCase) Execute(ctx context.Context, input ListEnvironmentsInput) ListEnvironmentsOutput {
	return ListEnvironmentsOutput{
		Status:       EnvironmentsListed,
		Environments: this.environmentRepository.ListNames(ctx),
	}
}

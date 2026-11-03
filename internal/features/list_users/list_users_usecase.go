package list_users

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ListUsersStatus int

const (
	UsersListed ListUsersStatus = iota
)

type ListUsersInput struct{}

type ListUsersOutput struct {
	Status ListUsersStatus
	Users  []*model.User
}

type ListUsersUseCase struct {
	userRepository *repository.UserRepository
}

func NewListUsersUseCase(userRepository *repository.UserRepository) *ListUsersUseCase {
	return &ListUsersUseCase{userRepository: userRepository}
}

func (this *ListUsersUseCase) Execute(ctx context.Context, input ListUsersInput) ListUsersOutput {
	return ListUsersOutput{
		Status: UsersListed,
		Users:  this.userRepository.List(ctx),
	}
}

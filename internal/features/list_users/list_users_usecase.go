package list_users

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ListUsersStatus int

const (
	UsersListed ListUsersStatus = iota
)

type ListUsersInput struct {
	Page int64
}

type ListUsersOutput struct {
	Status ListUsersStatus
	Users  []*model.User
	Total  int
}

type ListUsersUseCase struct {
	userRepository *repository.UserRepository
}

func NewListUsersUseCase(userRepository *repository.UserRepository) *ListUsersUseCase {
	return &ListUsersUseCase{userRepository: userRepository}
}

func (this *ListUsersUseCase) Execute(ctx context.Context, input ListUsersInput) ListUsersOutput {
	users, total := this.userRepository.List(ctx, pages.PageSize, pages.Offset(input.Page))

	return ListUsersOutput{Status: UsersListed, Users: users, Total: total}
}

package reactivate_user

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
)

type ReactivateUserStatus int

const (
	UserReactivated ReactivateUserStatus = iota
	UserNotFound
	UserAlreadyActive
)

type ReactivateUserInput struct {
	UserID int64
}

type ReactivateUserOutput struct {
	Status ReactivateUserStatus
}

type ReactivateUserUseCase struct {
	userRepository *repository.UserRepository
}

func NewReactivateUserUseCase(userRepository *repository.UserRepository) *ReactivateUserUseCase {
	return &ReactivateUserUseCase{userRepository: userRepository}
}

func (this *ReactivateUserUseCase) Execute(ctx context.Context, input ReactivateUserInput) ReactivateUserOutput {
	if _, found := this.userRepository.FindByID(ctx, input.UserID); !found {
		return ReactivateUserOutput{Status: UserNotFound}
	}

	if !this.userRepository.Reactivate(ctx, input.UserID) {
		return ReactivateUserOutput{Status: UserAlreadyActive}
	}

	return ReactivateUserOutput{Status: UserReactivated}
}

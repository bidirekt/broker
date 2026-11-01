package deactivate_user

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
)

type DeactivateUserStatus int

const (
	UserDeactivated DeactivateUserStatus = iota
	UserNotFound
	UserAlreadyDeactivated
)

type DeactivateUserInput struct {
	UserID int64
}

type DeactivateUserOutput struct {
	Status DeactivateUserStatus
}

type DeactivateUserUseCase struct {
	userRepository *repository.UserRepository
}

func NewDeactivateUserUseCase(userRepository *repository.UserRepository) *DeactivateUserUseCase {
	return &DeactivateUserUseCase{userRepository: userRepository}
}

func (this *DeactivateUserUseCase) Execute(ctx context.Context, input DeactivateUserInput) DeactivateUserOutput {
	if _, found := this.userRepository.FindByID(ctx, input.UserID); !found {
		return DeactivateUserOutput{Status: UserNotFound}
	}

	if !this.userRepository.Deactivate(ctx, input.UserID) {
		return DeactivateUserOutput{Status: UserAlreadyDeactivated}
	}

	return DeactivateUserOutput{Status: UserDeactivated}
}

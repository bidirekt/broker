package reactivate_user

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ReactivateUserStatus int

const (
	UserReactivated ReactivateUserStatus = iota
	UserNotFound
	UserAlreadyActive
	TargetIsTheAdmin
)

type ReactivateUserInput struct {
	UserID int64
}

type ReactivateUserOutput struct {
	Status ReactivateUserStatus
	User   *model.User
}

type ReactivateUserUseCase struct {
	userRepository *repository.UserRepository
}

func NewReactivateUserUseCase(userRepository *repository.UserRepository) *ReactivateUserUseCase {
	return &ReactivateUserUseCase{userRepository: userRepository}
}

func (this *ReactivateUserUseCase) Execute(ctx context.Context, input ReactivateUserInput) ReactivateUserOutput {
	user, found := this.userRepository.FindByID(ctx, input.UserID)
	if !found {
		return ReactivateUserOutput{Status: UserNotFound}
	}

	if user.Permission == model.PermissionAdmin {
		return ReactivateUserOutput{Status: TargetIsTheAdmin}
	}

	if !this.userRepository.Reactivate(ctx, input.UserID) {
		return ReactivateUserOutput{Status: UserAlreadyActive, User: user}
	}

	return ReactivateUserOutput{Status: UserReactivated, User: user}
}

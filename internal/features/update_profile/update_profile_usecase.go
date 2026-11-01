package update_profile

import (
	"context"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type UpdateProfileStatus int

const (
	PasswordChanged UpdateProfileStatus = iota
	CurrentPasswordWrong
	PasswordInvalid
)

type UpdateProfileInput struct {
	UserID          int64
	SessionIDHash   []byte
	CurrentPassword string
	NewPassword     string
}

type UpdateProfileOutput struct {
	Status UpdateProfileStatus
}

type UpdateProfileUseCase struct {
	userRepository *repository.UserRepository
}

func NewUpdateProfileUseCase(userRepository *repository.UserRepository) *UpdateProfileUseCase {
	return &UpdateProfileUseCase{userRepository: userRepository}
}

func (this *UpdateProfileUseCase) Execute(ctx context.Context, input UpdateProfileInput) (UpdateProfileOutput, error) {
	user, found := this.userRepository.FindByID(ctx, input.UserID)
	if !found || !components.VerifyPassword(input.CurrentPassword, user.PasswordHash.String) {
		return UpdateProfileOutput{Status: CurrentPasswordWrong}, nil
	}

	if validations.Password(input.NewPassword) != nil {
		return UpdateProfileOutput{Status: PasswordInvalid}, nil
	}

	passwordHash, err := components.HashPassword(input.NewPassword)
	if err != nil {
		return UpdateProfileOutput{}, err
	}

	if !this.userRepository.ChangePasswordKeepingSession(ctx, user.ID, user.PasswordHash.String, passwordHash, input.SessionIDHash) {
		return UpdateProfileOutput{Status: CurrentPasswordWrong}, nil
	}

	return UpdateProfileOutput{Status: PasswordChanged}, nil
}

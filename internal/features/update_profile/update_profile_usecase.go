package update_profile

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type UpdateProfileStatus int

const (
	PasswordChanged UpdateProfileStatus = iota
	CurrentPasswordWrong
	PasswordInvalid
	APIKeysListed
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

type ListAPIKeysInput struct {
	UserID int64
	Page   int64
}

type ListAPIKeysOutput struct {
	Status UpdateProfileStatus
	Keys   []*model.APIKey
	Total  int
}

type UpdateProfileUseCase struct {
	userRepository   *repository.UserRepository
	apiKeyRepository *repository.APIKeyRepository
}

func NewUpdateProfileUseCase(
	userRepository *repository.UserRepository,
	apiKeyRepository *repository.APIKeyRepository,
) *UpdateProfileUseCase {
	return &UpdateProfileUseCase{userRepository: userRepository, apiKeyRepository: apiKeyRepository}
}

func (this *UpdateProfileUseCase) ListAPIKeys(ctx context.Context, input ListAPIKeysInput) ListAPIKeysOutput {
	keys, total := this.apiKeyRepository.ListByUser(ctx, input.UserID, pages.PageSize, pages.Offset(input.Page))

	return ListAPIKeysOutput{Status: APIKeysListed, Keys: keys, Total: total}
}

func (this *UpdateProfileUseCase) Execute(ctx context.Context, input UpdateProfileInput) (UpdateProfileOutput, error) {
	user, found := this.userRepository.FindByID(ctx, input.UserID)
	if !found || !passwords.Verify(input.CurrentPassword, user.PasswordHash.String) {
		return UpdateProfileOutput{Status: CurrentPasswordWrong}, nil
	}

	if validations.Password(input.NewPassword) != nil {
		return UpdateProfileOutput{Status: PasswordInvalid}, nil
	}

	passwordHash, err := passwords.Hash(input.NewPassword)
	if err != nil {
		return UpdateProfileOutput{}, err
	}

	if !this.userRepository.ChangePasswordKeepingSession(ctx, user.ID, user.PasswordHash.String, passwordHash, input.SessionIDHash) {
		return UpdateProfileOutput{Status: CurrentPasswordWrong}, nil
	}

	return UpdateProfileOutput{Status: PasswordChanged}, nil
}

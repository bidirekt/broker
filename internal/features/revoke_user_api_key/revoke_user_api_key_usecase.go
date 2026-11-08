package revoke_user_api_key

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type RevokeUserAPIKeyStatus int

const (
	APIKeyRevoked RevokeUserAPIKeyStatus = iota
	APIKeyNotFound
	UserNotFound
	TargetIsTheAdmin
)

type RevokeUserAPIKeyInput struct {
	UserID   int64
	APIKeyID int64
}

type RevokeUserAPIKeyOutput struct {
	Status RevokeUserAPIKeyStatus
	Name   string
}

type RevokeUserAPIKeyUseCase struct {
	userRepository   *repository.UserRepository
	apiKeyRepository *repository.APIKeyRepository
}

func NewRevokeUserAPIKeyUseCase(
	userRepository *repository.UserRepository,
	apiKeyRepository *repository.APIKeyRepository,
) *RevokeUserAPIKeyUseCase {
	return &RevokeUserAPIKeyUseCase{userRepository: userRepository, apiKeyRepository: apiKeyRepository}
}

func (this *RevokeUserAPIKeyUseCase) Execute(ctx context.Context, input RevokeUserAPIKeyInput) RevokeUserAPIKeyOutput {
	user, found := this.userRepository.FindByID(ctx, input.UserID)
	if !found {
		return RevokeUserAPIKeyOutput{Status: UserNotFound}
	}

	if user.Permission == model.PermissionAdmin {
		return RevokeUserAPIKeyOutput{Status: TargetIsTheAdmin}
	}

	name, deleted := this.apiKeyRepository.DeleteByIDAndUser(ctx, input.APIKeyID, user.ID)
	if !deleted {
		return RevokeUserAPIKeyOutput{Status: APIKeyNotFound}
	}

	return RevokeUserAPIKeyOutput{Status: APIKeyRevoked, Name: name}
}

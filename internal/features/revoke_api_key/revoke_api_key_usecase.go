package revoke_api_key

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
)

type RevokeAPIKeyStatus int

const (
	APIKeyRevoked RevokeAPIKeyStatus = iota
	APIKeyNotFound
)

type RevokeAPIKeyInput struct {
	APIKeyID int64
	UserID   int64
}

type RevokeAPIKeyOutput struct {
	Status RevokeAPIKeyStatus
	Name   string
}

type RevokeAPIKeyUseCase struct {
	apiKeyRepository *repository.APIKeyRepository
}

func NewRevokeAPIKeyUseCase(apiKeyRepository *repository.APIKeyRepository) *RevokeAPIKeyUseCase {
	return &RevokeAPIKeyUseCase{apiKeyRepository: apiKeyRepository}
}

func (this *RevokeAPIKeyUseCase) Execute(ctx context.Context, input RevokeAPIKeyInput) RevokeAPIKeyOutput {
	name, deleted := this.apiKeyRepository.DeleteByIDAndUser(ctx, input.APIKeyID, input.UserID)
	if !deleted {
		return RevokeAPIKeyOutput{Status: APIKeyNotFound}
	}

	return RevokeAPIKeyOutput{Status: APIKeyRevoked, Name: name}
}

package list_user_api_keys

import (
	"context"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type ListUserAPIKeysStatus int

const (
	APIKeysListed ListUserAPIKeysStatus = iota
	UserNotFound
	TargetIsTheAdmin
)

type ListUserAPIKeysInput struct {
	UserID int64
	Page   int64
}

type ListUserAPIKeysOutput struct {
	Status ListUserAPIKeysStatus
	User   *model.User
	Keys   []*model.APIKey
	Total  int
}

type ListUserAPIKeysUseCase struct {
	userRepository   *repository.UserRepository
	apiKeyRepository *repository.APIKeyRepository
}

func NewListUserAPIKeysUseCase(
	userRepository *repository.UserRepository,
	apiKeyRepository *repository.APIKeyRepository,
) *ListUserAPIKeysUseCase {
	return &ListUserAPIKeysUseCase{userRepository: userRepository, apiKeyRepository: apiKeyRepository}
}

func (this *ListUserAPIKeysUseCase) Execute(ctx context.Context, input ListUserAPIKeysInput) ListUserAPIKeysOutput {
	user, found := this.userRepository.FindByID(ctx, input.UserID)
	if !found {
		return ListUserAPIKeysOutput{Status: UserNotFound}
	}

	if user.Permission == model.PermissionAdmin {
		return ListUserAPIKeysOutput{Status: TargetIsTheAdmin}
	}

	keys, total := this.apiKeyRepository.ListByUser(ctx, user.ID, pages.PageSize, pages.Offset(input.Page))

	return ListUserAPIKeysOutput{Status: APIKeysListed, User: user, Keys: keys, Total: total}
}

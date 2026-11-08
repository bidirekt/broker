package create_api_key

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

const (
	keyPrefix             = "bdk_"
	displayedRandomLength = 8
	maxNameCharacters     = 100
	day                   = 24 * time.Hour
	neverExpires          = "never"
)

var lifetimes = map[string]time.Duration{
	"30":  30 * day,
	"90":  90 * day,
	"365": 365 * day,
}

type CreateAPIKeyStatus int

const (
	APIKeyCreated CreateAPIKeyStatus = iota
	NameMissing
	NameTooLong
	PermissionInvalid
	PermissionAboveOwn
	ExpirationInvalid
	OwnerRefused
)

type CreateAPIKeyInput struct {
	UserID          int64
	OwnerPermission model.Permission
	Name            string
	Permission      string
	Expiration      string
}

type CreateAPIKeyOutput struct {
	Status CreateAPIKeyStatus
	Key    string
}

type CreateAPIKeyUseCase struct {
	apiKeyRepository *repository.APIKeyRepository
}

func NewCreateAPIKeyUseCase(apiKeyRepository *repository.APIKeyRepository) *CreateAPIKeyUseCase {
	return &CreateAPIKeyUseCase{apiKeyRepository: apiKeyRepository}
}

func (this *CreateAPIKeyUseCase) Execute(ctx context.Context, input CreateAPIKeyInput) (CreateAPIKeyOutput, error) {
	if input.Name == "" {
		return CreateAPIKeyOutput{Status: NameMissing}, nil
	}
	if utf8.RuneCountInString(input.Name) > maxNameCharacters {
		return CreateAPIKeyOutput{Status: NameTooLong}, nil
	}

	permission := model.Permission(input.Permission)
	if !permission.IsGrantable() {
		return CreateAPIKeyOutput{Status: PermissionInvalid}, nil
	}
	if !input.OwnerPermission.Includes(permission) {
		return CreateAPIKeyOutput{Status: PermissionAboveOwn}, nil
	}

	expiresAt, valid := expiresAtFrom(input.Expiration, time.Now())
	if !valid {
		return CreateAPIKeyOutput{Status: ExpirationInvalid}, nil
	}

	random, err := tokens.Random()
	if err != nil {
		return CreateAPIKeyOutput{}, err
	}

	key := keyPrefix + random
	if !this.apiKeyRepository.Create(ctx, &model.APIKey{
		UserID:        input.UserID,
		KeyHash:       tokens.Hash(key),
		DisplayPrefix: keyPrefix + random[:displayedRandomLength],
		Name:          input.Name,
		Permission:    permission,
		ExpiresAt:     expiresAt,
	}) {
		return CreateAPIKeyOutput{Status: OwnerRefused}, nil
	}

	return CreateAPIKeyOutput{Status: APIKeyCreated, Key: key}, nil
}

func expiresAtFrom(expiration string, now time.Time) (expiresAt null.Time, valid bool) {
	if expiration == neverExpires {
		return null.Time{}, true
	}

	lifetime, ok := lifetimes[expiration]
	if !ok {
		return null.Time{}, false
	}

	return null.TimeFrom(now.Add(lifetime)), true
}

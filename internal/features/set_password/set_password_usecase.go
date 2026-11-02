package set_password

import (
	"context"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type SetPasswordStatus int

const (
	PasswordSet SetPasswordStatus = iota
	PasswordInvalid
	LinkInvalid
)

type SetPasswordInput struct {
	Token    string
	Password string
}

type SetPasswordOutput struct {
	Status SetPasswordStatus
}

type SetPasswordUseCase struct {
	userRepository        *repository.UserRepository
	oneTimeLinkRepository *repository.OneTimeLinkRepository
}

func NewSetPasswordUseCase(
	userRepository *repository.UserRepository,
	oneTimeLinkRepository *repository.OneTimeLinkRepository,
) *SetPasswordUseCase {
	return &SetPasswordUseCase{userRepository: userRepository, oneTimeLinkRepository: oneTimeLinkRepository}
}

func (this *SetPasswordUseCase) IsLinkUsable(ctx context.Context, token string) bool {
	_, _, usable := this.oneTimeLinkRepository.FindUsable(ctx, tokens.Hash(token))
	return usable
}

func (this *SetPasswordUseCase) Execute(ctx context.Context, input SetPasswordInput) (SetPasswordOutput, error) {
	if validations.Password(input.Password) != nil {
		return SetPasswordOutput{Status: PasswordInvalid}, nil
	}

	if !this.IsLinkUsable(ctx, input.Token) {
		return SetPasswordOutput{Status: LinkInvalid}, nil
	}

	passwordHash, err := passwords.Hash(input.Password)
	if err != nil {
		return SetPasswordOutput{}, err
	}

	if !this.userRepository.SetPasswordWithLink(ctx, tokens.Hash(input.Token), passwordHash) {
		return SetPasswordOutput{Status: LinkInvalid}, nil
	}

	return SetPasswordOutput{Status: PasswordSet}, nil
}

package set_password

import (
	"context"

	"github.com/bidirekt/broker/internal/components"
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
	_, _, usable := this.oneTimeLinkRepository.FindUsable(ctx, components.HashToken(token))
	return usable
}

func (this *SetPasswordUseCase) Execute(ctx context.Context, input SetPasswordInput) (SetPasswordOutput, error) {
	if validations.Password(input.Password) != nil {
		return SetPasswordOutput{Status: PasswordInvalid}, nil
	}

	passwordHash, err := components.HashPassword(input.Password)
	if err != nil {
		return SetPasswordOutput{}, err
	}

	if !this.userRepository.SetPasswordWithLink(ctx, components.HashToken(input.Token), passwordHash) {
		return SetPasswordOutput{Status: LinkInvalid}, nil
	}

	return SetPasswordOutput{Status: PasswordSet}, nil
}

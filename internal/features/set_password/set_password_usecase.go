package set_password

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type SetPasswordStatus int

const (
	PasswordSet SetPasswordStatus = iota
	LinkFound
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

type FindLinkOutput struct {
	Status    SetPasswordStatus
	Email     string
	ExpiresAt time.Time
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

func (this *SetPasswordUseCase) FindLink(ctx context.Context, token string) FindLinkOutput {
	link, user, usable := this.oneTimeLinkRepository.FindUsable(ctx, tokens.Hash(token))
	if !usable {
		return FindLinkOutput{Status: LinkInvalid}
	}

	return FindLinkOutput{Status: LinkFound, Email: user.Email, ExpiresAt: link.ExpiresAt}
}

func (this *SetPasswordUseCase) Execute(ctx context.Context, input SetPasswordInput) (SetPasswordOutput, error) {
	if validations.Password(input.Password) != nil {
		return SetPasswordOutput{Status: PasswordInvalid}, nil
	}

	if this.FindLink(ctx, input.Token).Status != LinkFound {
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

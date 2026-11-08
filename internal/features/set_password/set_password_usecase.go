package set_password

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
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

type FindLinkOutput struct {
	Status    SetPasswordStatus
	Email     string
	Purpose   model.LinkPurpose
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

	return FindLinkOutput{Status: LinkFound, Email: user.Email, Purpose: link.Purpose, ExpiresAt: link.ExpiresAt}
}

func (this *SetPasswordUseCase) Execute(ctx context.Context, input SetPasswordInput) (FindLinkOutput, error) {
	link := this.FindLink(ctx, input.Token)
	if link.Status != LinkFound {
		return link, nil
	}

	if validations.Password(input.Password) != nil {
		link.Status = PasswordInvalid
		return link, nil
	}

	passwordHash, err := passwords.Hash(input.Password)
	if err != nil {
		return FindLinkOutput{}, err
	}

	if !this.userRepository.SetPasswordWithLink(ctx, tokens.Hash(input.Token), passwordHash) {
		return FindLinkOutput{Status: LinkInvalid}, nil
	}

	return FindLinkOutput{Status: PasswordSet}, nil
}

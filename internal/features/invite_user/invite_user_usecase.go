package invite_user

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/components/tokens"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

const inviteLinkLifetime = 24 * time.Hour

type InviteUserStatus int

const (
	UserInvited InviteUserStatus = iota
	EmailInvalid
	PermissionInvalid
	EmailAlreadyInUse
)

type InviteUserInput struct {
	Email      string
	Permission string
}

type InviteUserOutput struct {
	Status InviteUserStatus
	Link   string
}

type InviteUserUseCase struct {
	userRepository *repository.UserRepository
	publicURL      string
}

func NewInviteUserUseCase(userRepository *repository.UserRepository, publicURL string) *InviteUserUseCase {
	return &InviteUserUseCase{userRepository: userRepository, publicURL: publicURL}
}

func (this *InviteUserUseCase) Execute(ctx context.Context, input InviteUserInput) (InviteUserOutput, error) {
	if validations.Email(input.Email) != nil {
		return InviteUserOutput{Status: EmailInvalid}, nil
	}

	permission := model.Permission(input.Permission)
	if permission != model.PermissionRead && permission != model.PermissionWrite {
		return InviteUserOutput{Status: PermissionInvalid}, nil
	}

	token, err := tokens.Random()
	if err != nil {
		return InviteUserOutput{}, err
	}

	created := this.userRepository.CreateInvited(ctx,
		&model.User{Email: input.Email, Permission: permission},
		&model.OneTimeLink{
			TokenHash: tokens.Hash(token),
			Purpose:   model.LinkPurposeInvite,
			ExpiresAt: time.Now().Add(inviteLinkLifetime),
		},
	)
	if !created {
		return InviteUserOutput{Status: EmailAlreadyInUse}, nil
	}

	return InviteUserOutput{Status: UserInvited, Link: this.publicURL + setPasswordPath + token}, nil
}

package generate_link

import (
	"context"
	"time"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

const linkLifetime = 24 * time.Hour

type GenerateLinkStatus int

const (
	LinkGenerated GenerateLinkStatus = iota
	UserNotFound
	UserDeactivated
)

type GenerateLinkInput struct {
	UserID int64
}

type GenerateLinkOutput struct {
	Status  GenerateLinkStatus
	Email   string
	Purpose model.LinkPurpose
	Link    string
}

type GenerateLinkUseCase struct {
	userRepository *repository.UserRepository
	linkRepository *repository.OneTimeLinkRepository
	publicURL      string
}

func NewGenerateLinkUseCase(userRepository *repository.UserRepository, linkRepository *repository.OneTimeLinkRepository, publicURL string) *GenerateLinkUseCase {
	return &GenerateLinkUseCase{userRepository: userRepository, linkRepository: linkRepository, publicURL: publicURL}
}

func (this *GenerateLinkUseCase) Execute(ctx context.Context, input GenerateLinkInput) (GenerateLinkOutput, error) {
	user, found := this.userRepository.FindByID(ctx, input.UserID)
	if !found {
		return GenerateLinkOutput{Status: UserNotFound}, nil
	}

	purpose, ok := linkPurposeFor(user.State())
	if !ok {
		return GenerateLinkOutput{Status: UserDeactivated}, nil
	}

	token, err := components.RandomToken()
	if err != nil {
		return GenerateLinkOutput{}, err
	}

	this.linkRepository.ReplaceOpen(ctx, &model.OneTimeLink{
		TokenHash: components.HashToken(token),
		UserID:    user.ID,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(linkLifetime),
	})

	return GenerateLinkOutput{
		Status:  LinkGenerated,
		Email:   user.Email,
		Purpose: purpose,
		Link:    this.publicURL + setPasswordPath + token,
	}, nil
}

func linkPurposeFor(state model.UserState) (model.LinkPurpose, bool) {
	switch state {
	case model.UserInvited:
		return model.LinkPurposeInvite, true
	case model.UserActive:
		return model.LinkPurposeReset, true
	}

	return "", false
}

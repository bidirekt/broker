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
	linkRepository *repository.OneTimeLinkRepository
	publicURL      string
}

func NewGenerateLinkUseCase(linkRepository *repository.OneTimeLinkRepository, publicURL string) *GenerateLinkUseCase {
	return &GenerateLinkUseCase{linkRepository: linkRepository, publicURL: publicURL}
}

func (this *GenerateLinkUseCase) Execute(ctx context.Context, input GenerateLinkInput) (GenerateLinkOutput, error) {
	token, err := components.RandomToken()
	if err != nil {
		return GenerateLinkOutput{}, err
	}

	link := &model.OneTimeLink{
		TokenHash: components.HashToken(token),
		UserID:    input.UserID,
		ExpiresAt: time.Now().Add(linkLifetime),
	}
	email, found, deactivated := this.linkRepository.ReplaceOpen(ctx, link)
	if !found {
		return GenerateLinkOutput{Status: UserNotFound}, nil
	}

	if deactivated {
		return GenerateLinkOutput{Status: UserDeactivated}, nil
	}

	return GenerateLinkOutput{
		Status:  LinkGenerated,
		Email:   email,
		Purpose: link.Purpose,
		Link:    this.publicURL + setPasswordPath + token,
	}, nil
}

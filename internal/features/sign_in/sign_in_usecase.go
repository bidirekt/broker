package sign_in

import (
	"context"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
)

type SignInStatus int

const (
	SignedIn SignInStatus = iota
	InvalidCredentials
)

type SignInInput struct {
	Email    string
	Password string
}

type SignInOutput struct {
	Status               SignInStatus
	UserID               int64
	VerifiedPasswordHash string
}

type SignInUseCase struct {
	userRepository *repository.UserRepository
}

func NewSignInUseCase(userRepository *repository.UserRepository) *SignInUseCase {
	return &SignInUseCase{userRepository: userRepository}
}

func (this *SignInUseCase) Execute(ctx context.Context, input SignInInput) SignInOutput {
	user, found := this.userRepository.FindByEmail(ctx, input.Email)
	if !found || user.State() != model.UserActive {
		passwords.Verify(input.Password, passwords.DummyHash)
		return SignInOutput{Status: InvalidCredentials}
	}

	if !passwords.Verify(input.Password, user.PasswordHash.String) {
		return SignInOutput{Status: InvalidCredentials}
	}

	return SignInOutput{Status: SignedIn, UserID: user.ID, VerifiedPasswordHash: user.PasswordHash.String}
}

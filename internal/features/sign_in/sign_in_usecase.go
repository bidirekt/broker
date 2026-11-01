package sign_in

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/bidirekt/broker/internal/components"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/guregu/null"
)

type SignInStatus int

const (
	SignedIn SignInStatus = iota
	InvalidCredentials
)

type SignInInput struct {
	Identifier string
	Password   string
}

type SignInOutput struct {
	Status SignInStatus
	// null when the admin signed in
	UserID null.Int
}

type SignInUseCase struct {
	userRepository *repository.UserRepository
	adminUsername  string
	adminPassword  string
}

func NewSignInUseCase(userRepository *repository.UserRepository, adminUsername, adminPassword string) *SignInUseCase {
	return &SignInUseCase{
		userRepository: userRepository,
		adminUsername:  adminUsername,
		adminPassword:  adminPassword,
	}
}

func (this *SignInUseCase) Execute(ctx context.Context, input SignInInput) SignInOutput {
	if !strings.Contains(input.Identifier, "@") {
		return this.signInAdmin(input)
	}

	user, found := this.userRepository.FindByEmail(ctx, input.Identifier)
	if !found || user.State() != model.UserActive {
		components.VerifyPassword(input.Password, components.DummyPasswordHash)
		return SignInOutput{Status: InvalidCredentials}
	}

	if !components.VerifyPassword(input.Password, user.PasswordHash.String) {
		return SignInOutput{Status: InvalidCredentials}
	}

	return SignInOutput{Status: SignedIn, UserID: null.IntFrom(user.ID)}
}

func (this *SignInUseCase) signInAdmin(input SignInInput) SignInOutput {
	usernameMatches := equalInConstantTime(input.Identifier, this.adminUsername)
	passwordMatches := equalInConstantTime(input.Password, this.adminPassword)
	if usernameMatches&passwordMatches != 1 {
		return SignInOutput{Status: InvalidCredentials}
	}

	return SignInOutput{Status: SignedIn}
}

// both sides are hashed first because subtle.ConstantTimeCompare returns early on a length difference
func equalInConstantTime(given, expected string) int {
	givenHash := sha256.Sum256([]byte(given))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(givenHash[:], expectedHash[:])
}

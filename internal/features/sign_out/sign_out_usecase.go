package sign_out

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
)

type SignOutStatus int

const (
	SignedOut SignOutStatus = iota
)

type SignOutInput struct {
	SessionIDHash []byte
}

type SignOutOutput struct {
	Status SignOutStatus
}

type SignOutUseCase struct {
	sessionRepository *repository.SessionRepository
}

func NewSignOutUseCase(sessionRepository *repository.SessionRepository) *SignOutUseCase {
	return &SignOutUseCase{sessionRepository: sessionRepository}
}

func (this *SignOutUseCase) Execute(ctx context.Context, input SignOutInput) SignOutOutput {
	this.sessionRepository.Delete(ctx, input.SessionIDHash)

	return SignOutOutput{Status: SignedOut}
}

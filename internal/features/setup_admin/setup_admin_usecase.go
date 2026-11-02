package setup_admin

import (
	"context"

	"github.com/bidirekt/broker/internal/components/passwords"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type SetupAdminStatus int

const (
	AdminCreated SetupAdminStatus = iota
	EmailInvalid
	PasswordInvalid
	AlreadySetUp
	SetUpConcurrently
)

type SetupAdminInput struct {
	Email    string
	Password string
}

type SetupAdminOutput struct {
	Status       SetupAdminStatus
	UserID       int64
	PasswordHash string
}

type SetupAdminUseCase struct {
	userRepository *repository.UserRepository
}

func NewSetupAdminUseCase(userRepository *repository.UserRepository) *SetupAdminUseCase {
	return &SetupAdminUseCase{userRepository: userRepository}
}

func (this *SetupAdminUseCase) IsSetUp(ctx context.Context) bool {
	return this.userRepository.AdminExists(ctx)
}

func (this *SetupAdminUseCase) Execute(ctx context.Context, input SetupAdminInput) (SetupAdminOutput, error) {
	if this.IsSetUp(ctx) {
		return SetupAdminOutput{Status: AlreadySetUp}, nil
	}

	if validations.Email(input.Email) != nil {
		return SetupAdminOutput{Status: EmailInvalid}, nil
	}

	if validations.Password(input.Password) != nil {
		return SetupAdminOutput{Status: PasswordInvalid}, nil
	}

	passwordHash, err := passwords.Hash(input.Password)
	if err != nil {
		return SetupAdminOutput{}, err
	}

	admin := &model.User{Email: input.Email}
	if !this.userRepository.CreateAdmin(ctx, admin, passwordHash) {
		return SetupAdminOutput{Status: SetUpConcurrently}, nil
	}

	return SetupAdminOutput{Status: AdminCreated, UserID: admin.ID, PasswordHash: passwordHash}, nil
}

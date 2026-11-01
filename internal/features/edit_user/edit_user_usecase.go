package edit_user

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type EditUserStatus int

const (
	UserEdited EditUserStatus = iota
	EmailInvalid
	PermissionInvalid
	EmailAlreadyInUse
	UserNotFound
)

type EditUserInput struct {
	UserID     int64
	Email      string
	Permission string
}

type EditUserOutput struct {
	Status EditUserStatus
}

type EditUserUseCase struct {
	userRepository *repository.UserRepository
}

func NewEditUserUseCase(userRepository *repository.UserRepository) *EditUserUseCase {
	return &EditUserUseCase{userRepository: userRepository}
}

func (this *EditUserUseCase) FindUser(ctx context.Context, userID int64) (*model.User, bool) {
	return this.userRepository.FindByID(ctx, userID)
}

func (this *EditUserUseCase) Execute(ctx context.Context, input EditUserInput) EditUserOutput {
	if validations.Email(input.Email) != nil {
		return EditUserOutput{Status: EmailInvalid}
	}

	permission := model.Permission(input.Permission)
	if permission != model.PermissionRead && permission != model.PermissionWrite {
		return EditUserOutput{Status: PermissionInvalid}
	}

	updated, conflict := this.userRepository.UpdateEmailAndPermission(ctx, input.UserID, input.Email, permission)
	if conflict {
		return EditUserOutput{Status: EmailAlreadyInUse}
	}

	if !updated {
		return EditUserOutput{Status: UserNotFound}
	}

	return EditUserOutput{Status: UserEdited}
}

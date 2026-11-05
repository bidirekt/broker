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
	UserFound
	EmailInvalid
	PermissionInvalid
	EmailAlreadyInUse
	UserNotFound
	TargetIsTheAdmin
)

type EditUserInput struct {
	UserID     int64
	Email      string
	Permission string
}

type EditUserOutput struct {
	Status EditUserStatus
	User   *model.User
}

type FindUserOutput struct {
	Status EditUserStatus
	User   *model.User
}

type EditUserUseCase struct {
	userRepository *repository.UserRepository
}

func NewEditUserUseCase(userRepository *repository.UserRepository) *EditUserUseCase {
	return &EditUserUseCase{userRepository: userRepository}
}

func (this *EditUserUseCase) FindUser(ctx context.Context, userID int64) FindUserOutput {
	user, found := this.userRepository.FindByID(ctx, userID)
	if !found {
		return FindUserOutput{Status: UserNotFound}
	}

	if user.Permission == model.PermissionAdmin {
		return FindUserOutput{Status: TargetIsTheAdmin}
	}

	return FindUserOutput{Status: UserFound, User: user}
}

func (this *EditUserUseCase) Execute(ctx context.Context, input EditUserInput) EditUserOutput {
	target := this.FindUser(ctx, input.UserID)
	if target.Status != UserFound {
		return EditUserOutput{Status: target.Status}
	}

	if validations.Email(input.Email) != nil {
		return EditUserOutput{Status: EmailInvalid, User: target.User}
	}

	permission := model.Permission(input.Permission)
	if !permission.IsGrantable() {
		return EditUserOutput{Status: PermissionInvalid, User: target.User}
	}

	updated, conflict := this.userRepository.UpdateEmailAndPermission(ctx, input.UserID, input.Email, permission)
	if conflict {
		return EditUserOutput{Status: EmailAlreadyInUse, User: target.User}
	}

	if !updated {
		return EditUserOutput{Status: UserNotFound}
	}

	return EditUserOutput{Status: UserEdited, User: target.User}
}

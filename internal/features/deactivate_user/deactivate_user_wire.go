package deactivate_user

const (
	UserNotFoundMessage string = "user not found"

	userIDParam = "id"

	usersPath = "/users"

	deactivateUserTemplate = "deactivate_user.html"
)

type DeactivateUserPage struct {
	Message string
}

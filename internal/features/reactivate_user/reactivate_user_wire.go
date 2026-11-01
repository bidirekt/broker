package reactivate_user

const (
	UserNotFoundMessage string = "user not found"

	userIDParam = "id"

	usersPath = "/users"

	reactivateUserTemplate = "reactivate_user.html"
)

type ReactivateUserPage struct {
	Message string
}

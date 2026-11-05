package reactivate_user

import "strconv"

const (
	UserNotFoundMessage    string = "user not found"
	UserReactivatedMessage string = "user reactivated: "

	userIDParam = "id"
	pageParam   = "page"

	usersPath = "/users"
)

func usersPathFor(page int64) string {
	if page == 0 {
		return usersPath
	}

	return usersPath + "?" + pageParam + "=" + strconv.FormatInt(page, 10)
}

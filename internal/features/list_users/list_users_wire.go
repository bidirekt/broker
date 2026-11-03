package list_users

import "github.com/bidirekt/broker/internal/model"

const listUsersTemplate = "list_users.html"

type ListUsersPage struct {
	Users []*model.User
}

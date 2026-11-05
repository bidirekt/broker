package list_users

import (
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	pageParam = "page"

	listUsersTemplate = "list_users.html"
)

type ListUsersPage struct {
	Users []*model.User
	Pager pages.Pager
}

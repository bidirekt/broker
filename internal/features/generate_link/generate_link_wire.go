package generate_link

import "github.com/bidirekt/broker/internal/model"

const (
	UserNotFoundMessage    string = "user not found"
	UserDeactivatedMessage string = "deactivated users cannot get a link"

	userIDParam = "id"

	setPasswordPath = "/set-password?token="

	generateLinkTemplate = "generate_link.html"
)

type GenerateLinkPage struct {
	Email   string
	Purpose model.LinkPurpose
	Link    string
	Message string
}

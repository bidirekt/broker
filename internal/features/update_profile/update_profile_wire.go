package update_profile

import (
	"time"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

const (
	PasswordChangedMessage        string = "password changed"
	CurrentPasswordWrongMessage   string = "the current password is wrong"
	PasswordInvalidMessage        string = "the password must have 8 to 128 characters"
	TooManyProfileAttemptsMessage string = "too many attempts, try again later"

	currentPasswordFormField = "current_password"
	newPasswordFormField     = "new_password"

	profilePath = "/profile"
	pageParam   = "page"
	firstPage   = 1

	displayPrefixEllipsis = "…"

	updateProfileTemplate = "update_profile.html"
)

var permissionDescriptions = map[model.Permission]string{
	model.PermissionRead:  "Sees every data screen and their own profile; runs can-i-deploy and validate.",
	model.PermissionWrite: "Everything read does, plus publishing contracts, recording deployments, and creating participants and environments, and renaming environments.",
	model.PermissionAdmin: "Everything write can do, plus managing users. The only admin of this broker.",
}

type UpdateProfilePage struct {
	Email                 string
	Permission            model.Permission
	PermissionDescription string
	Field                 string
	Message               string
	APIKeys               []APIKeyRow
	Pager                 pages.Pager
}

type APIKeyRow struct {
	ID            int64
	Name          string
	DisplayPrefix string
	Permission    model.Permission
	ActsAs        model.Permission
	CreatedAt     time.Time
	LastUsedAt    null.Time
	ExpiresAt     null.Time
	Expired       bool
}

func toAPIKeyRows(keys []*model.APIKey, ownerPermission model.Permission, now time.Time) []APIKeyRow {
	rows := make([]APIKeyRow, 0, len(keys))
	for _, key := range keys {
		row := APIKeyRow{
			ID:            key.ID,
			Name:          key.Name,
			DisplayPrefix: key.DisplayPrefix + displayPrefixEllipsis,
			Permission:    key.Permission,
			CreatedAt:     key.CreatedAt,
			LastUsedAt:    key.LastUsedAt,
			ExpiresAt:     key.ExpiresAt,
			Expired:       key.IsExpiredAt(now),
		}
		if capped := key.Permission.CappedBy(ownerPermission); capped != key.Permission {
			row.ActsAs = capped
		}
		rows = append(rows, row)
	}

	return rows
}

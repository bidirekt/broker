package list_user_api_keys

import (
	"time"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

const (
	UserNotFoundMessage string = "user not found"

	userIDParam = "id"
	pageParam   = "page"

	displayPrefixEllipsis = "…"

	listUserAPIKeysTemplate = "list_user_api_keys.html"
)

type ListUserAPIKeysPage struct {
	UserID  int64
	Email   string
	APIKeys []APIKeyRow
	Pager   pages.Pager
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

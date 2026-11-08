package revoke_user_api_key

import "strconv"

const (
	UserNotFoundMessage        string = "user not found"
	APIKeyNotFoundMessage      string = "API key not found"
	APIKeyRevokedMessageFormat string = "API key %s revoked"

	userIDParam   = "id"
	apiKeyIDParam = "keyId"
)

func userAPIKeysPath(userID int64) string {
	return "/users/" + strconv.FormatInt(userID, 10) + "/api-keys"
}

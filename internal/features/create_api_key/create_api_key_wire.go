package create_api_key

const (
	NameMissingMessage        string = "name is required"
	NameTooLongMessage        string = "name has at most 100 characters"
	PermissionInvalidMessage  string = "choose read or write"
	PermissionAboveOwnMessage string = "you can't give more than your own permission"
	ExpirationInvalidMessage  string = "choose 30, 90, 365 days or never"

	nameFormField       = "name"
	permissionFormField = "permission"
	expirationFormField = "expiration"

	defaultExpiration = "90"

	newAPIKeyPath = "/profile/api-keys/new"

	createAPIKeyTemplate = "create_api_key.html"
)

type CreateAPIKeyPage struct {
	Name       string
	Permission string
	Expiration string
	Field      string
	Message    string
	Key        string
}

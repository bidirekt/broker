package list_environments

const EnvironmentsListed string = "environments listed successfully"

type ListEnvironmentsResponseBody struct {
	Message      string   `json:"message"`
	Environments []string `json:"environments"`
}

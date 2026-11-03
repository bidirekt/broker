package health

const HealthOK string = "ok"

// bumped only when the request or response format changes
const apiVersion = 2

// var, not const: the release pipeline sets it with -ldflags -X
var brokerVersion = "dev"

type HealthResponseBody struct {
	Status        string `json:"status"`
	BrokerVersion string `json:"brokerVersion"`
	APIVersion    int    `json:"apiVersion"`
}

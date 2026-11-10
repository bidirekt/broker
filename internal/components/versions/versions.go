package versions

const MinimumCli = "0.1.0"

// var, not const: the release pipeline sets it with -ldflags -X
var brokerVersion = "dev"

func Broker() string {
	return brokerVersion
}

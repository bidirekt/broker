package graphql

// DeployedContract is the contract version a participant has deployed in an environment.
type DeployedContract struct {
	ParticipantID int64
	ContractID    int64
	Version       string
}

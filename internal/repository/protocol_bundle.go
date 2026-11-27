package repository

// Protocols bundles what each registered protocol hands the contract repository: its table and its optional ports.
type Protocols struct {
	Tables     []ProtocolTable
	Extensions []ContractExtensionStore
}

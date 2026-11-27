package graphql

type Identity struct {
	Operation   string
	Field       string
	Interaction string
}

func (this Identity) HashParts() []string {
	return []string{this.Operation, this.Field, this.Interaction}
}

func (this Identity) Describe() string {
	return this.Operation + " " + this.Field
}

func (this Identity) Location() map[string]string {
	return map[string]string{
		OperationKey:   this.Operation,
		FieldKey:       this.Field,
		InteractionKey: this.Interaction,
	}
}

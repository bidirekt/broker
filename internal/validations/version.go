package validations

func Version(value string) error {
	return UnpaddedValue(value)
}

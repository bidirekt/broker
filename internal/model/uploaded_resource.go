package model

import "strings"

type UploadedResource struct {
	Resource
}

// Describe names the resource the way publish errors quote it: the same parts that
// make up its hash, in reading order.
func (this *UploadedResource) Describe() string {
	parts := []string{this.Direction.String()}

	if this.IsConsumer() && this.ConsumedProvider.String != "" {
		parts = append(parts, this.ConsumedProvider.String)
	}

	parts = append(parts, this.Identity.Describe())

	return strings.Join(parts, " ")
}

func (this *UploadedResource) ProviderHash() string {
	providerName := this.ParticipantName
	if this.IsConsumer() {
		providerName = this.ConsumedProvider.String
	}

	return Hash(append([]string{providerName}, this.Identity.HashParts()...)...)
}

func (this *UploadedResource) ConsumerHash() string {
	if !this.IsConsumer() {
		return ""
	}

	return Hash(append([]string{this.ParticipantName, this.ConsumedProvider.String}, this.Identity.HashParts()...)...)
}

func (this *UploadedResource) PrimaryHash() string {
	if this.IsProvider() {
		return this.ProviderHash()
	}

	return this.ConsumerHash()
}

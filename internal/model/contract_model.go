package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type UploadedContract struct {
	ID              int64
	Version         string
	ContractContent string
	Resources       map[string]UploadedResource
	ParticipantID   int64
	ParticipantName string
}

func NewUploadedContract(
	participantID int64,
	participantName string,
	version string,
	contractContent string,
) *UploadedContract {
	return &UploadedContract{
		ParticipantID:   participantID,
		ParticipantName: participantName,
		Version:         version,
		ContractContent: contractContent,
	}
}

// AddResource keys the resource by its hash and rejects a hash already taken.
func (this *UploadedContract) AddResource(resource *UploadedResource) error {
	if this.Resources == nil {
		this.Resources = make(map[string]UploadedResource)
	}

	resource.ParticipantName = this.ParticipantName
	hash := resource.PrimaryHash()

	if _, taken := this.Resources[hash]; taken {
		return fmt.Errorf("resource already added: %s", resource.Describe())
	}

	this.Resources[hash] = *resource

	return nil
}

func (this *UploadedContract) Checksum() string {
	payload, _ := json.Marshal(this.Resources)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

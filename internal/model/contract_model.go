package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/guregu/null"
)

type UploadedContract struct {
	ID              int64
	Version         string
	ContractContent string
	Resources       map[string]UploadedResource
	Schemas         Schemas
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

// The sources are left out, like the files themselves: the same content split into other files is the
// same snapshot. Roots are sorted so the slice order never changes it.
func (this *UploadedContract) Checksum() string {
	resources := make(map[string]UploadedResource, len(this.Resources))
	roots := make(map[string][]null.String, len(this.Resources))
	for hash, resource := range this.Resources {
		roots[hash] = sortedRoots(resource.Bodies)
		resource.Bodies = nil
		resources[hash] = resource
	}

	payload, _ := json.Marshal(struct {
		Resources map[string]UploadedResource
		Roots     map[string][]null.String
		Schemas   Schemas
	}{resources, roots, this.Schemas})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func sortedRoots(bodies []ResourceBody) []null.String {
	roots := make([]null.String, len(bodies))
	for index, body := range bodies {
		roots[index] = body.SchemaName
	}

	slices.SortFunc(roots, func(first, second null.String) int {
		if first.Valid != second.Valid {
			if first.Valid {
				return 1
			}
			return -1
		}

		return strings.Compare(first.String, second.String)
	})

	return roots
}

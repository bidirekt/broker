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
	Files           []ContractFile
	Resources       map[string]UploadedResource
	Schemas         map[string]Schemas
	Extensions      map[string]any
	ParticipantID   int64
	ParticipantName string
}

func NewUploadedContract(
	participantID int64,
	participantName string,
	version string,
	files []ContractFile,
) *UploadedContract {
	return &UploadedContract{
		ParticipantID:   participantID,
		ParticipantName: participantName,
		Version:         version,
		Files:           files,
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
// same snapshot; an extension keeps its sources out of its JSON for the same reason. Roots are sorted so the
// slice order never changes it.
func (this *UploadedContract) Checksum() string {
	resources := make(map[string]UploadedResource, len(this.Resources))
	roots := make(map[string][]null.String, len(this.Resources))
	for hash, resource := range this.Resources {
		roots[hash] = sortedRoots(resource.Bodies)
		resource.Bodies = nil
		resources[hash] = resource
	}

	yamlSchemas := Schemas{}
	namespacedSchemas := map[string]Schemas{}
	for namespace, schemas := range this.Schemas {
		if namespace == YAMLNamespace {
			yamlSchemas = schemas
		} else {
			namespacedSchemas[namespace] = schemas
		}
	}

	payload, _ := json.Marshal(struct {
		Resources         map[string]UploadedResource
		Roots             map[string][]null.String
		Schemas           Schemas
		NamespacedSchemas map[string]Schemas `json:",omitempty"`
		Extensions        map[string]any     `json:",omitempty"`
	}{resources, roots, yamlSchemas, namespacedSchemas, this.Extensions})
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

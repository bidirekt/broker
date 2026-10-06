package repository

import (
	"database/sql"
	"time"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
)

// This struct is used to build the entire contract avoiding N+1 problem
type tableRow struct {
	// Participant
	ParticipantID   int64
	ParticipantName string

	// Contract
	ContractID        int64
	ContractVersion   string
	ContractContent   string
	ContractCreatedAt time.Time

	// Resource
	ResourceID                 int64
	ResourceDirection          string
	ResourceInteraction        string
	ResourceConsumedProvider   sql.NullString
	ResourceEndpoint           string
	ResourceMethod             string
	ResourceResponseStatusCode sql.NullString
	ResourceProviderHash       string
	ResourceConsumerHash       sql.NullString
	ResourceCreatedAt          time.Time
	ResourceVersion            string
	ResourceVersionChangeType  string

	// Property
	PropertyID                int64
	PropertyPath              string
	PropertyVersionType       sql.NullString
	PropertyVersionOptional   sql.NullBool
	PropertyVersionChangeType string

	// Deploy & Environment
	DeploymentEnvironment sql.NullString
	DeploymentVersion     sql.NullString
}

func (this *tableRow) toPersistedContractModel() *model.PersistedContract {
	return &model.PersistedContract{
		ID:              this.ContractID,
		ParticipantID:   this.ParticipantID,
		ParticipantName: this.ParticipantName,
		Version:         this.ContractVersion,
		ContractContent: this.ContractContent,
		Resources:       make(map[string]model.PersistedResource),
	}
}

func (this *tableRow) toResourceModel() model.PersistedResource {
	resource := model.PersistedResource{
		Direction:        model.Direction(this.ResourceDirection),
		Interaction:      model.Interaction(this.ResourceInteraction),
		Endpoint:         this.ResourceEndpoint,
		Method:           this.ResourceMethod,
		Properties:       make(map[string]model.Property),
		DeployedVersions: make(map[string]string),
		ParticipantName:  this.ParticipantName,
		ParticipantID:    this.ParticipantID,
		ContractID:       this.ContractID,
		ProviderHash:     this.ResourceProviderHash,
		Removed:          this.ResourceVersionChangeType == string(model.ChangeRemoved),
	}

	if this.ResourceVersion != "" {
		resource.ParticipantVersion = null.StringFrom(this.ResourceVersion)
	}

	if this.ResourceConsumedProvider.String != "" {
		resource.ConsumedProvider = null.StringFrom(this.ResourceConsumedProvider.String)
	}

	if this.ResourceResponseStatusCode.String != "" {
		resource.ResponseStatusCode = null.StringFrom(this.ResourceResponseStatusCode.String)
	}

	if this.ResourceConsumerHash.String != "" {
		resource.ConsumerHash = null.StringFrom(this.ResourceConsumerHash.String)
	}

	return resource
}

func (this *tableRow) toPropertyModel() model.Property {
	return model.Property{
		ID:       this.PropertyID,
		Path:     this.PropertyPath,
		Type:     this.PropertyVersionType.String,
		Optional: this.PropertyVersionOptional.Bool,
	}
}

type insertPropertyVersionRow struct {
	PropertyID int64
	ContractID int64
	Type       sql.NullString
	Optional   sql.NullBool
	ChangeType string
}

func newInsertPropertyVersionRowAdded(contractID, propertyID int64, p model.Property) *insertPropertyVersionRow {
	return newInsertPropertyVersionRow(contractID, propertyID, p, model.ChangeAdded)
}

func newInsertPropertyVersionRowModified(contractID, propertyID int64, p model.Property) *insertPropertyVersionRow {
	return newInsertPropertyVersionRow(contractID, propertyID, p, model.ChangeModified)
}

func newInsertPropertyVersionRowRemoved(contractID, propertyID int64, p model.Property) *insertPropertyVersionRow {
	return newInsertPropertyVersionRow(contractID, propertyID, p, model.ChangeRemoved)
}

func newInsertPropertyVersionRow(contractID, propertyID int64, p model.Property, change model.ChangeKind) *insertPropertyVersionRow {
	return &insertPropertyVersionRow{
		PropertyID: propertyID,
		ContractID: contractID,
		Type:       sql.NullString{String: p.Type, Valid: p.Type != ""},
		Optional:   sql.NullBool{Bool: p.Optional, Valid: true},
		ChangeType: string(change),
	}
}

type insertResourceVersionRow struct {
	ResourceID int64
	ContractID int64
	ChangeType string
}

func newInsertResourceVersionRowAdded(contractID, resourceID int64) *insertResourceVersionRow {
	return &insertResourceVersionRow{
		ResourceID: resourceID,
		ContractID: contractID,
		ChangeType: string(model.ChangeAdded),
	}
}

func newInsertResourceVersionRowRemoved(contractID, resourceID int64) *insertResourceVersionRow {
	return &insertResourceVersionRow{
		ResourceID: resourceID,
		ContractID: contractID,
		ChangeType: string(model.ChangeRemoved),
	}
}

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

	// Bodies, aligned by index
	BodySources     []string
	BodySchemaNames []null.String

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
		Bodies:           this.toResourceBodies(),
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

func (this *tableRow) toResourceBodies() []model.ResourceBody {
	bodies := make([]model.ResourceBody, len(this.BodySources))
	for index, source := range this.BodySources {
		bodies[index] = model.ResourceBody{Source: source, SchemaName: this.BodySchemaNames[index]}
	}

	return bodies
}

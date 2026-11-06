package repository

import (
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
	ResourceConsumedProvider   null.String
	ResourceEndpoint           string
	ResourceMethod             string
	ResourceResponseStatusCode null.String
	ResourceProviderHash       string
	ResourceConsumerHash       null.String
	ResourceCreatedAt          time.Time
	ResourceVersion            string
	ResourceVersionChangeType  string

	// Bodies, aligned by index
	BodySources     []string
	BodySchemaNames []null.String
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
		Direction:          model.Direction(this.ResourceDirection),
		Interaction:        model.Interaction(this.ResourceInteraction),
		Endpoint:           this.ResourceEndpoint,
		Method:             this.ResourceMethod,
		Bodies:             this.toResourceBodies(),
		ParticipantName:    this.ParticipantName,
		ParticipantID:      this.ParticipantID,
		ContractID:         this.ContractID,
		ProviderHash:       this.ResourceProviderHash,
		ConsumedProvider:   this.ResourceConsumedProvider,
		ResponseStatusCode: this.ResourceResponseStatusCode,
		ConsumerHash:       this.ResourceConsumerHash,
		Removed:            this.ResourceVersionChangeType == string(model.ChangeRemoved),
	}

	if this.ResourceVersion != "" {
		resource.ParticipantVersion = null.StringFrom(this.ResourceVersion)
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

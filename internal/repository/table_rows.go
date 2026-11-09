package repository

import (
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
)

const resourceColumns = `
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.interaction,
			r.consumed_provider,
			r.endpoint,
			r.method,
			r.response_status_code,
			r.provider_hash,
			r.consumer_hash,
			rv.change_type,
			bodies.sources,
			bodies.schema_names`

// This struct is used to build the entire contract avoiding N+1 problem
type tableRow struct {
	// Participant
	ParticipantID   int64
	ParticipantName string

	// Contract
	ContractID      int64
	ContractVersion string

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
		Resources:       make(map[string]model.PersistedResource),
	}
}

func (this *tableRow) toResourceModel() model.PersistedResource {
	return model.PersistedResource{
		Resource: model.Resource{
			Direction:          model.Direction(this.ResourceDirection),
			Interaction:        model.Interaction(this.ResourceInteraction),
			Endpoint:           this.ResourceEndpoint,
			Method:             this.ResourceMethod,
			Bodies:             this.toResourceBodies(),
			ParticipantName:    this.ParticipantName,
			ConsumedProvider:   this.ResourceConsumedProvider,
			ResponseStatusCode: this.ResourceResponseStatusCode,
		},
		ParticipantID: this.ParticipantID,
		ContractID:    this.ContractID,
		ProviderHash:  this.ResourceProviderHash,
		ConsumerHash:  this.ResourceConsumerHash,
		Removed:       this.ResourceVersionChangeType == string(model.ChangeRemoved),
	}
}

func (this *tableRow) toResourceBodies() []model.ResourceBody {
	bodies := make([]model.ResourceBody, len(this.BodySources))
	for index, source := range this.BodySources {
		bodies[index] = model.ResourceBody{Source: source, SchemaName: this.BodySchemaNames[index]}
	}

	return bodies
}

func scanTableRows(rows pgx.Rows) []tableRow {
	defer rows.Close()

	var tableRows []tableRow

	for rows.Next() {
		var row tableRow

		if err := rows.Scan(
			&row.ContractID,
			&row.ContractVersion,
			&row.ParticipantID,
			&row.ParticipantName,
			&row.ResourceID,
			&row.ResourceDirection,
			&row.ResourceInteraction,
			&row.ResourceConsumedProvider,
			&row.ResourceEndpoint,
			&row.ResourceMethod,
			&row.ResourceResponseStatusCode,
			&row.ResourceProviderHash,
			&row.ResourceConsumerHash,
			&row.ResourceVersionChangeType,
			&row.BodySources,
			&row.BodySchemaNames,
		); err != nil {
			panic(fmt.Errorf("error scanning resource row: %w", err))
		}

		tableRows = append(tableRows, row)
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading resource rows: %w", err))
	}

	return tableRows
}

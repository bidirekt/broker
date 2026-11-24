package integration_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/bidirekt/broker/internal/repository"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
	"github.com/guregu/null"
)

const withoutResourcesSchemasOnlyContract = `schemas:
  Pet:
    type: object
    properties:
      name:
        type: string
`

func (this *IntegrationSuite) mustPostForWithoutResources(path, body string) string {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)

	return response
}

func (this *IntegrationSuite) withoutResourcesParticipantID() int64 {
	this.mustPostForWithoutResources("/api/participants", `{"participant":"pets_api"}`)

	var participantID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = 'pets_api'`).Scan(&participantID))

	return participantID
}

func withoutResourcesSchemas(names ...string) map[string]model.Schemas {
	schemas := model.Schemas{}
	for _, name := range names {
		schemas[name] = model.Schema{"": {Type: "object"}, "name": {Type: "string"}}
	}

	return map[string]model.Schemas{model.YAMLNamespace: schemas}
}

func withoutResourcesContract(participantID int64, version string, schemas map[string]model.Schemas) *model.UploadedContract {
	contract := model.NewUploadedContract(participantID, "pets_api", version,
		[]model.ContractFile{{Source: "api.yaml", Content: "version " + version}})
	contract.Schemas = schemas

	return contract
}

func (this *IntegrationSuite) withPetsResource(contract *model.UploadedContract) *model.UploadedContract {
	resource := model.UploadedResource{Resource: model.Resource{
		Direction: model.Provides,
		Protocol:  restmodel.Protocol,
		Body:      model.Output,
		Identity:  restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"},
		Bodies:    []model.ResourceBody{{Source: "api.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("Pet")}},
	}}
	this.Require().NoError(contract.AddResource(&resource))

	return contract
}

func (this *IntegrationSuite) publishWithoutResources(repo *repository.ContractRepository, contract *model.UploadedContract) {
	conflict := repo.Publish(context.Background(), contract,
		func(current *model.PersistedContract) model.ContractDiff {
			return contract_differ.DiffContracts(current, contract)
		},
		func([]model.ContractFile) bool { return true })
	this.Require().False(conflict)
}

func (this *IntegrationSuite) TestContractRepository_ContractWithoutResources_LoadsWithItsSchemas() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, repository.Protocols{Tables: []repository.ProtocolTable{restrepository.New()}})
	participantID := this.withoutResourcesParticipantID()
	this.publishWithoutResources(repo, withoutResourcesContract(participantID, "1", withoutResourcesSchemas("Pet", "Owner")))

	byName, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "1")
	this.Require().True(found)
	this.Empty(byName.Resources)
	this.Equal(withoutResourcesSchemas("Pet", "Owner"), byName.Schemas)

	byID := repo.GetContractByIDAndVersion(ctx, byName.ID, "1")
	this.Empty(byID.Resources)
	this.Equal(withoutResourcesSchemas("Pet", "Owner"), byID.Schemas)

	latest, found := repo.GetLatestSnapshotByParticipantID(ctx, participantID)
	this.Require().True(found)
	this.Equal(byName.ID, latest.ID)
	this.Empty(latest.Resources)
	this.Equal(withoutResourcesSchemas("Pet", "Owner"), latest.Schemas)
}

func (this *IntegrationSuite) TestContractRepository_NextSnapshotOverAContractWithoutResources_RemovesTheDroppedSchema() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, repository.Protocols{Tables: []repository.ProtocolTable{restrepository.New()}})
	participantID := this.withoutResourcesParticipantID()
	this.publishWithoutResources(repo, withoutResourcesContract(participantID, "1", withoutResourcesSchemas("Pet", "Owner")))
	this.publishWithoutResources(repo, withoutResourcesContract(participantID, "2", withoutResourcesSchemas("Pet")))

	this.Equal([]string{"1 Owner added", "1 Pet added", "2 Owner removed"}, this.withoutResourcesSchemaEvents())

	second, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "2")
	this.Require().True(found)
	this.Equal(withoutResourcesSchemas("Pet"), second.Schemas)

	first, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "1")
	this.Require().True(found)
	this.Equal(withoutResourcesSchemas("Pet", "Owner"), first.Schemas)
}

func (this *IntegrationSuite) TestContractRepository_EveryResourceRemoved_StaysOutOfTheVersionThatRemovedIt() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, repository.Protocols{Tables: []repository.ProtocolTable{restrepository.New()}})
	participantID := this.withoutResourcesParticipantID()
	this.publishWithoutResources(repo, this.withPetsResource(withoutResourcesContract(participantID, "1", withoutResourcesSchemas("Pet"))))
	this.publishWithoutResources(repo, withoutResourcesContract(participantID, "2", withoutResourcesSchemas("Pet")))

	latest, found := repo.GetLatestSnapshotByParticipantID(ctx, participantID)
	this.Require().True(found)
	this.Equal("2", latest.Version)
	this.Empty(latest.Resources)

	second, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "2")
	this.Require().True(found)
	this.Require().Len(second.Resources, 1)
	for _, resource := range second.Resources {
		this.True(resource.Removed)
		this.Empty(resource.Bodies)
	}

	first, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "1")
	this.Require().True(found)
	this.Require().Len(first.Resources, 1)
	for _, resource := range first.Resources {
		this.False(resource.Removed)
	}

	this.publishWithoutResources(repo, withoutResourcesContract(participantID, "3", withoutResourcesSchemas()))
	this.Equal([]string{"1 Pet added", "3 Pet removed"}, this.withoutResourcesSchemaEvents())
	this.Equal([]string{"1 added", "2 removed"}, this.withoutResourcesResourceEvents())
}

func (this *IntegrationSuite) TestCanIDeployAndValidate_ContractWithoutResources_AreDeployable() {
	this.mustPostForWithoutResources("/api/environments", `{"environment":"production"}`)
	this.mustPostForWithoutResources("/api/participants", `{"participant":"pets_api"}`)
	this.mustPostForWithoutResources("/api/contracts",
		this.publishBody("pets_api", "1", contractFragment{"api.yaml", withoutResourcesSchemasOnlyContract}))
	this.mustPostForWithoutResources("/api/deployments", `{"participant":"pets_api","version":"1","environment":"production"}`)

	canIDeploy := this.mustPostForWithoutResources("/api/can-i-deploy",
		`{"participant":"pets_api","version":"1","environment":"production"}`)

	validateBody, err := json.Marshal(struct {
		Participant string              `json:"participant"`
		Environment string              `json:"environment"`
		Contracts   []map[string]string `json:"contracts"`
	}{"pets_api", "production", []map[string]string{{"source": "api.yaml", "content": withoutResourcesSchemasOnlyContract}}})
	this.Require().NoError(err)
	validate := this.mustPostForWithoutResources("/api/contracts/validate", string(validateBody))

	this.JSONEq(`{
		"message": "contract checked successfully",
		"participant": "pets_api",
		"version": "1",
		"environment": "production",
		"deployable": true,
		"results": {}
	}`, canIDeploy)
	this.JSONEq(`{
		"message": "contract validated successfully",
		"participant": "pets_api",
		"environment": "production",
		"deployable": true,
		"results": {}
	}`, validate)
}

func (this *IntegrationSuite) withoutResourcesSchemaEvents() []string {
	return this.withoutResourcesStrings(`
		SELECT sv.contract_id || ' ' || s.name || ' ' || sv.change_type
		FROM schema_versions sv
		JOIN schemas s ON s.id = sv.schema_id
		ORDER BY sv.contract_id, s.name COLLATE "C"`)
}

func (this *IntegrationSuite) withoutResourcesResourceEvents() []string {
	return this.withoutResourcesStrings(`
		SELECT contract_id || ' ' || change_type
		FROM resource_versions
		ORDER BY contract_id`)
}

func (this *IntegrationSuite) withoutResourcesStrings(query string) []string {
	rows, err := this.Pool.Query(context.Background(), query)
	this.Require().NoError(err)
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		this.Require().NoError(rows.Scan(&value))
		values = append(values, value)
	}
	this.Require().NoError(rows.Err())

	return values
}

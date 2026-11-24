package integration_test

import (
	"context"
	"net/http"

	"github.com/bidirekt/broker/internal/contracts/contract_differ"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/bidirekt/broker/internal/repository"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
	"github.com/guregu/null"
)

func (this *IntegrationSuite) scalarArgumentsParticipantID() int64 {
	status, response := this.post("/api/participants", `{"participant":"pets_api"}`)
	this.Require().Equalf(http.StatusOK, status, "POST /api/participants: %s", response)

	var participantID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM participants WHERE name = 'pets_api'`).Scan(&participantID))

	return participantID
}

func scalarArgumentsPetSchemas(scalar, arguments string) map[string]model.Schemas {
	return map[string]model.Schemas{model.YAMLNamespace: {
		"Pet": {
			"":          {Type: "object"},
			"bornAt":    {Type: "scalar", Scalar: scalar},
			"reviews":   {Type: "array", Arguments: arguments},
			"reviews[]": {Type: "string"},
		},
		"ReviewsArguments": {"": {Type: "object"}, "first": {Type: "integer", Optional: true}},
		"ReviewsPage":      {"": {Type: "object"}, "after": {Type: "string", Optional: true}},
	}}
}

func (this *IntegrationSuite) scalarArgumentsContract(participantID int64, version string, schemas map[string]model.Schemas) *model.UploadedContract {
	contract := model.NewUploadedContract(participantID, "pets_api", version,
		[]model.ContractFile{{Source: "api.yaml", Content: "version " + version}})
	contract.Schemas = schemas

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

func (this *IntegrationSuite) publishScalarArgumentsContract(repo *repository.ContractRepository, contract *model.UploadedContract) {
	conflict := repo.Publish(context.Background(), contract,
		func(current *model.PersistedContract) model.ContractDiff {
			return contract_differ.DiffContracts(current, contract)
		},
		func([]model.ContractFile) bool { return true })
	this.Require().False(conflict)
}

func (this *IntegrationSuite) TestContractRepository_ScalarAndArguments_ReadBackAsWritten() {
	repo := repository.NewContractRepository(this.Pool, repository.Protocols{Tables: []repository.ProtocolTable{restrepository.New()}})
	written := scalarArgumentsPetSchemas("DateTime", "ReviewsArguments")
	this.publishScalarArgumentsContract(repo, this.scalarArgumentsContract(this.scalarArgumentsParticipantID(), "1", written))

	loaded, found := repo.GetContractByNameAndVersion(context.Background(), "pets_api", "1")
	this.Require().True(found)

	this.Equal(written, loaded.Schemas)
}

func (this *IntegrationSuite) TestContractRepository_ScalarOrArgumentsChanged_WritesAModifiedFieldVersion() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, repository.Protocols{Tables: []repository.ProtocolTable{restrepository.New()}})
	participantID := this.scalarArgumentsParticipantID()

	this.publishScalarArgumentsContract(repo, this.scalarArgumentsContract(participantID, "1", scalarArgumentsPetSchemas("DateTime", "ReviewsArguments")))
	this.publishScalarArgumentsContract(repo, this.scalarArgumentsContract(participantID, "2", scalarArgumentsPetSchemas("Date", "ReviewsArguments")))
	this.publishScalarArgumentsContract(repo, this.scalarArgumentsContract(participantID, "3", scalarArgumentsPetSchemas("Date", "ReviewsPage")))

	this.Equal([]string{
		"1 Pet.bornAt added DateTime ",
		"1 Pet.reviews added  ReviewsArguments",
		"2 Pet.bornAt modified Date ",
		"3 Pet.reviews modified  ReviewsPage",
	}, this.scalarArgumentsFieldVersions())

	second, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "2")
	this.Require().True(found)
	this.Equal(scalarArgumentsPetSchemas("Date", "ReviewsArguments"), second.Schemas)

	third, found := repo.GetContractByNameAndVersion(ctx, "pets_api", "3")
	this.Require().True(found)
	this.Equal(scalarArgumentsPetSchemas("Date", "ReviewsPage"), third.Schemas)
}

func (this *IntegrationSuite) scalarArgumentsFieldVersions() []string {
	rows, err := this.Pool.Query(context.Background(), `
		SELECT fv.contract_id || ' ' || s.name || '.' || f.path || ' ' || fv.change_type || ' ' ||
			COALESCE(fv.scalar, '') || ' ' || COALESCE(fv.arguments, '')
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		JOIN schemas s ON s.id = f.schema_id
		WHERE f.path IN ('bornAt', 'reviews')
		ORDER BY fv.contract_id, f.path`)
	this.Require().NoError(err)
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		this.Require().NoError(rows.Scan(&version))
		versions = append(versions, version)
	}
	this.Require().NoError(rows.Err())

	return versions
}

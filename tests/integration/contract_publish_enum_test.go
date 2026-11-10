package integration_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
)

const enumPetsProviderContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      status:
        type: enum
        members: [%s]
      name:
        type: string
`

const invalidEnumsContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      status:
        type: enum
      kind:
        type: enum
        members: []
      size:
        type: enum
        members: [small, large, small]
      color:
        type: enum
        members: [black, ""]
      name:
        type: string
        members: [rex]
`

func (this *IntegrationSuite) mustPostForEnum(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishEnumPetsProvider(version, members string) {
	this.mustPostForEnum("/api/contracts", this.publishBody("petstore_api", version,
		contractFragment{"api.yaml", fmt.Sprintf(enumPetsProviderContract, members)}))
}

func (this *IntegrationSuite) TestPublishContract_Enum_ReadsBackItsValuesSorted() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, []repository.ProtocolTable{restrepository.New()})

	this.mustPostForEnum("/api/participants", `{"participant":"petstore_api"}`)
	this.publishEnumPetsProvider("1", "sold, available, pending")

	contract, found := repo.GetContractByNameAndVersion(ctx, "petstore_api", "1")
	this.Require().True(found)
	this.Equal(model.Schema{
		"":       {Type: "object"},
		"status": {Type: "enum", Members: []string{"available", "pending", "sold"}},
		"name":   {Type: "string"},
	}, contract.Schemas["Pet"])
}

func (this *IntegrationSuite) TestPublishContract_OnlyEnumValuesChanged_WritesAModifiedFieldVersion() {
	ctx := context.Background()

	this.mustPostForEnum("/api/participants", `{"participant":"petstore_api"}`)
	this.publishEnumPetsProvider("1", "available, sold")
	this.publishEnumPetsProvider("2", "sold, available")
	this.publishEnumPetsProvider("3", "available, pending")

	rows, err := this.Pool.Query(ctx, `
		SELECT fv.change_type, array_to_string(fv.members, ', ')
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		WHERE f.path = 'status'
		ORDER BY fv.contract_id`)
	this.Require().NoError(err)
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var changeType, members string
		this.Require().NoError(rows.Scan(&changeType, &members))
		versions = append(versions, changeType+": "+members)
	}
	this.Require().NoError(rows.Err())

	this.Equal([]string{"added: available, sold", "modified: available, pending"}, versions)

	var nameMembers []string
	this.Require().NoError(this.Pool.QueryRow(ctx, `
		SELECT fv.members
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		WHERE f.path = 'name'`).Scan(&nameMembers))
	this.Nil(nameMembers)
}

func (this *IntegrationSuite) TestPublishContract_InvalidEnums_RejectedWithEachViolation() {
	this.mustPostForEnum("/api/participants", `{"participant":"petstore_api"}`)

	status, body := this.post("/api/contracts", this.publishBody("petstore_api", "1",
		contractFragment{"api.yaml", invalidEnumsContract},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"members.not_text","path":"schemas;Pet;properties;color;members;1","source":"api.yaml","details":null},`+
		`{"code":"members.empty","path":"schemas;Pet;properties;kind;members","source":"api.yaml","details":{"type":"enum"}},`+
		`{"code":"schema.key_not_allowed","path":"schemas;Pet;properties;name","source":"api.yaml","details":{"key":"members","type":"string"}},`+
		`{"code":"members.duplicate","path":"schemas;Pet;properties;size;members;2","source":"api.yaml","details":{"value":"small"}},`+
		`{"code":"members.required","path":"schemas;Pet;properties;status","source":"api.yaml","details":{"type":"enum"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

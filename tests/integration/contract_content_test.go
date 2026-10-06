package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

const contentParticipantBody = `{"participant":"content_service"}`

const singleFileYAML = `# the whole service in one file
provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet   # the only status we serve
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

const endpointsYAML = `# endpoints only
provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
`

const schemasYAML = `# schemas only
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
`

func (this *IntegrationSuite) contractContentForVersion(version string) []contractFragment {
	var content string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		"SELECT contract_content FROM contract_versions WHERE version = $1", version,
	).Scan(&content))

	var fragments []contractFragment
	this.Require().NoError(json.Unmarshal([]byte(content), &fragments))

	return fragments
}

func (this *IntegrationSuite) TestPublish_EachVersionStampsItsOwnFiles() {
	status, _ := this.post("/api/participants", contentParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("content_service", "v42",
		contractFragment{"api.yaml", singleFileYAML},
	))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("content_service", "v43",
		contractFragment{".contracts/api/pets.yaml", endpointsYAML},
		contractFragment{".contracts/api/schemas.yaml", schemasYAML},
	))
	this.Require().Equal(http.StatusOK, status)

	// same hydrated resources: v43 aliases the snapshot of v42
	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))

	this.Equal(
		[]contractFragment{{"api.yaml", singleFileYAML}},
		this.contractContentForVersion("v42"),
	)
	this.Equal(
		[]contractFragment{
			{".contracts/api/pets.yaml", endpointsYAML},
			{".contracts/api/schemas.yaml", schemasYAML},
		},
		this.contractContentForVersion("v43"),
	)
}

func (this *IntegrationSuite) TestPublish_RepublishingTheSameVersionKeepsTheFirstFiles() {
	status, _ := this.post("/api/participants", contentParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("content_service", "v42",
		contractFragment{"api.yaml", singleFileYAML},
	))
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("content_service", "v42",
		contractFragment{".contracts/api/pets.yaml", endpointsYAML},
		contractFragment{".contracts/api/schemas.yaml", schemasYAML},
	))
	this.Require().Equal(http.StatusOK, status)

	this.Equal(1, this.countRows("contract_versions"))
	this.Equal(
		[]contractFragment{{"api.yaml", singleFileYAML}},
		this.contractContentForVersion("v42"),
	)
}

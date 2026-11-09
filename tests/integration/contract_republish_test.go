package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
)

const (
	republishParticipant = "republish_service"
	republishVersion     = "1.2.3"
)

const republishPetsYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
`

const republishPetsRestyledYAML = `# the pets the store serves
schemas:
    Pet: {properties: {name: {type: "string"}, id: {type: string}}, type: 'object'}
provides:
    rest:
        /pets:
            get:
                responses: {200: Pet}   # the only status we serve
`

const republishPetsRestYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
`

const republishPetsSchemasYAML = `schemas:
  Pet:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
`

const republishPetsWithAnIntegerIDYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id:
        type: integer
      name:
        type: string
`

const republishDottedSchemaNameYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet.v2
schemas:
  Pet.v2:
    type: object
    properties:
      id:
        type: string
`

const republishUndottedSchemaNameYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: PetV2
schemas:
  PetV2:
    type: object
    properties:
      id:
        type: string
`

func (this *IntegrationSuite) createParticipantForRepublish() {
	status, body := this.post("/api/participants", `{"participant":"`+republishParticipant+`"}`)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) publishForRepublish(files ...contractFragment) (status int, body string) {
	return this.post("/api/contracts", this.publishBody(republishParticipant, republishVersion, files...))
}

func (this *IntegrationSuite) firstPublishForRepublish(files ...contractFragment) {
	this.createParticipantForRepublish()

	status, body := this.publishForRepublish(files...)
	this.Require().Equal(http.StatusOK, status, body)
}

func (this *IntegrationSuite) storeVersionAcceptedByAnOlderGrammar(files ...contractFragment) {
	ctx := context.Background()

	content, err := json.Marshal(files)
	this.Require().NoError(err)

	var participantID, contractID int64
	this.Require().NoError(this.Pool.QueryRow(ctx,
		`SELECT id FROM participants WHERE name = $1`, republishParticipant,
	).Scan(&participantID))
	this.Require().NoError(this.Pool.QueryRow(ctx,
		`INSERT INTO contracts (participant_id, checksum) VALUES ($1, 'checksum of an older grammar') RETURNING id`, participantID,
	).Scan(&contractID))

	_, err = this.Pool.Exec(ctx,
		`INSERT INTO contract_versions (participant_id, version, contract_id, contract_content) VALUES ($1, $2, $3, $4)`,
		participantID, republishVersion, contractID, string(content),
	)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) storedFilesForRepublish() []contractFragment {
	var content string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT contract_content FROM contract_versions WHERE version = $1`, republishVersion,
	).Scan(&content))

	var files []contractFragment
	this.Require().NoError(json.Unmarshal([]byte(content), &files))

	return files
}

func (this *IntegrationSuite) TestRepublish_SameFiles_Succeeds() {
	this.firstPublishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	status, body := this.publishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("contract_versions"))
}

func (this *IntegrationSuite) TestRepublish_SameFilesAfterTheBrokerChangedTheirChecksum_Succeeds() {
	this.firstPublishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})
	_, err := this.Pool.Exec(context.Background(), `UPDATE contracts SET checksum = 'checksum of an older grammar'`)
	this.Require().NoError(err)

	status, body := this.publishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("contract_versions"))
}

func (this *IntegrationSuite) TestRepublish_OnlyCommentsFormattingOrKeyOrderChanged_Succeeds() {
	this.firstPublishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	status, body := this.publishForRepublish(contractFragment{"contracts/api.yaml", republishPetsRestyledYAML})

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("contract_versions"))
}

func (this *IntegrationSuite) TestRepublish_SameContentSplitIntoOtherFiles_SucceedsKeepingTheFirstFiles() {
	this.firstPublishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	status, body := this.publishForRepublish(
		contractFragment{"contracts/pets.yaml", republishPetsRestYAML},
		contractFragment{"contracts/schemas.yaml", republishPetsSchemasYAML},
	)

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal([]contractFragment{{"contracts/api.yaml", republishPetsYAML}}, this.storedFilesForRepublish())
}

func (this *IntegrationSuite) TestRepublish_FromAnotherFolder_SucceedsKeepingTheFirstSources() {
	this.firstPublishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	status, body := this.publishForRepublish(contractFragment{"./contracts/api.yaml", republishPetsYAML})

	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)
	this.Equal([]contractFragment{{"contracts/api.yaml", republishPetsYAML}}, this.storedFilesForRepublish())
}

func (this *IntegrationSuite) TestRepublish_AFieldTypeChanged_Returns409() {
	this.firstPublishForRepublish(contractFragment{"contracts/api.yaml", republishPetsYAML})

	status, body := this.publishForRepublish(contractFragment{"contracts/api.yaml", republishPetsWithAnIntegerIDYAML})

	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"contract version already exists with different content"}`, body)
	this.Equal(1, this.countRows("contracts"))
	this.Equal([]contractFragment{{"contracts/api.yaml", republishPetsYAML}}, this.storedFilesForRepublish())
}

func (this *IntegrationSuite) TestRepublish_FilesTheCurrentGrammarRefuses_Returns400WithTheViolation() {
	this.createParticipantForRepublish()
	this.storeVersionAcceptedByAnOlderGrammar(contractFragment{"contracts/api.yaml", republishDottedSchemaNameYAML})

	status, body := this.publishForRepublish(contractFragment{"contracts/api.yaml", republishDottedSchemaNameYAML})

	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.name_syntax","path":"schemas;Pet.v2","source":"contracts/api.yaml","details":{"key":"Pet.v2","error":"must not contain \".\", \"[\", \"]\", \"{\", \"}\" or \";\""}}`+
		`]}`, body)
}

func (this *IntegrationSuite) TestRepublish_OverStoredFilesTheCurrentGrammarRefuses_Returns409() {
	this.createParticipantForRepublish()
	this.storeVersionAcceptedByAnOlderGrammar(contractFragment{"contracts/api.yaml", republishDottedSchemaNameYAML})

	status, body := this.publishForRepublish(contractFragment{"contracts/api.yaml", republishUndottedSchemaNameYAML})

	this.Equal(http.StatusConflict, status)
	this.JSONEq(`{"message":"contract version already exists with different content"}`, body)
	this.Equal([]contractFragment{{"contracts/api.yaml", republishDottedSchemaNameYAML}}, this.storedFilesForRepublish())
}

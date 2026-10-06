package integration_test

import (
	"context"
	"fmt"
	"net/http"
)

const frontParticipantBody = `{"participant":"front_app"}`

const invoiceListModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceList
schemas:
  InvoiceList:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
`

const invoiceDetailModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceDetail
schemas:
  InvoiceDetail:
    type: object
    properties:
      id:
        type: string
      created_at:
        type: string
`

const invoiceCreateModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        post:
          request: InvoiceCreate
schemas:
  InvoiceCreate:
    type: object
    properties:
      id:
        type: string
      name:
        type: string
`

const invoiceImportModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        post:
          request: InvoiceImport
schemas:
  InvoiceImport:
    type: object
    properties:
      id:
        type: string
      email:
        type: string
`

const invoiceBothSpellingsYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceList
      /invoices/:
        get:
          responses:
            200: InvoiceDetail
`

const invoiceBothSpellingsSchemasYAML = `schemas:
  InvoiceList:
    type: object
    properties:
      id:
        type: string
  InvoiceDetail:
    type: object
    properties:
      created_at:
        type: string
`

const invoiceStringIDModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceString
schemas:
  InvoiceString:
    type: object
    properties:
      id:
        type: string
`

const invoiceIntegerIDModuleYAML = `consumes:
  payments:
    rest:
      /invoices:
        get:
          responses:
            200: InvoiceInteger
schemas:
  InvoiceInteger:
    type: object
    properties:
      id:
        type: integer
`

func (this *IntegrationSuite) mergedProperties() []string {
	rows, err := this.Pool.Query(context.Background(),
		`SELECT properties.path, property_versions.type, property_versions.optional
		   FROM properties
		   JOIN property_versions ON property_versions.property_id = properties.id
		  ORDER BY properties.path`,
	)
	this.Require().NoError(err)
	defer rows.Close()

	var stored []string
	for rows.Next() {
		var path, propertyType string
		var optional bool
		this.Require().NoError(rows.Scan(&path, &propertyType, &optional))

		presence := "required"
		if optional {
			presence = "optional"
		}

		stored = append(stored, fmt.Sprintf("%s %s %s", path, propertyType, presence))
	}

	return stored
}

func (this *IntegrationSuite) TestPublishContract_ConsumerModulesReadTheSameResponse_MergesByUnion() {
	status, _ := this.post("/api/participants", frontParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("front_app", "1",
		contractFragment{"list.yaml", invoiceListModuleYAML},
		contractFragment{"detail.yaml", invoiceDetailModuleYAML},
	))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	this.Equal(1, this.countRows("resources"))
	this.Equal([]string{
		"$ object required",
		"$.created_at string required",
		"$.id string required",
		"$.name string required",
	}, this.mergedProperties())
}

func (this *IntegrationSuite) TestPublishContract_ConsumerModulesSendTheSameRequest_PartialFieldsTurnOptional() {
	status, _ := this.post("/api/participants", frontParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("front_app", "1",
		contractFragment{"create.yaml", invoiceCreateModuleYAML},
		contractFragment{"import.yaml", invoiceImportModuleYAML},
	))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	this.Equal(1, this.countRows("resources"))
	this.Equal([]string{
		"$ object required",
		"$.email string optional",
		"$.id string required",
		"$.name string optional",
	}, this.mergedProperties())
}

func (this *IntegrationSuite) TestPublishContract_BothConsumedSpellingsInOneFile_MergesIntoOneResource() {
	status, _ := this.post("/api/participants", frontParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("front_app", "1",
		contractFragment{"invoices.yaml", invoiceBothSpellingsYAML},
		contractFragment{"schemas.yaml", invoiceBothSpellingsSchemasYAML},
	))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	this.Equal(1, this.countRows("resources"))
	this.Equal([]string{
		"$ object required",
		"$.created_at string required",
		"$.id string required",
	}, this.mergedProperties())

	var endpoint string
	err := this.Pool.QueryRow(context.Background(), "SELECT endpoint FROM resources").Scan(&endpoint)
	this.Require().NoError(err)
	this.Equal("/invoices", endpoint)
}

func (this *IntegrationSuite) TestPublishContract_ConsumerModulesDisagreeOnPropertyType_Rejected() {
	status, _ := this.post("/api/participants", frontParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("front_app", "1",
		contractFragment{"a.yaml", invoiceStringIDModuleYAML},
		contractFragment{"b.yaml", invoiceIntegerIDModuleYAML},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"resource.type_conflict","path":"consumes;payments;rest;/invoices;get;responses;200","source":"b.yaml","details":{"resource":"consumes payments GET /invoices 200","property":"$.id","type":"integer","declaredIn":"a.yaml","declaredType":"string"}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
	this.Equal(0, this.countRows("resources"))
}

func (this *IntegrationSuite) TestPublishContract_MergedFragmentsInAnyOrder_AliasTheSameSnapshot() {
	status, _ := this.post("/api/participants", frontParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	status, _ = this.post("/api/contracts", this.publishBody("front_app", "1",
		contractFragment{"list.yaml", invoiceListModuleYAML},
		contractFragment{"detail.yaml", invoiceDetailModuleYAML},
	))
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", this.publishBody("front_app", "2",
		contractFragment{"detail.yaml", invoiceDetailModuleYAML},
		contractFragment{"list.yaml", invoiceListModuleYAML},
	))
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	this.Equal(1, this.countRows("contracts"))
	this.Equal(2, this.countRows("contract_versions"))
	this.Equal(1, this.countRows("resources"))
}

func (this *IntegrationSuite) TestPublishContract_MergedContractRepublished_StaysOneVersion() {
	status, _ := this.post("/api/participants", frontParticipantBody)
	this.Require().Equal(http.StatusOK, status)

	published := this.publishBody("front_app", "1",
		contractFragment{"list.yaml", invoiceListModuleYAML},
		contractFragment{"detail.yaml", invoiceDetailModuleYAML},
	)

	status, _ = this.post("/api/contracts", published)
	this.Require().Equal(http.StatusOK, status)

	status, body := this.post("/api/contracts", published)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract publish successful"}`, body)

	this.Equal(1, this.countRows("contracts"))
	this.Equal(1, this.countRows("contract_versions"))
	this.Equal(1, this.countRows("resources"))
}

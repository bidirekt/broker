package integration_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/internal/repository"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
)

const unionOrdersProviderContract = `provides:
  rest:
    /orders:
      get:
        responses:
          200: Order
    /payments:
      get:
        responses:
          200: Payment
schemas:
  Order:
    type: object
    properties:
      payment:
        type: union
        members:
          card: { ref: %s }
          pix:  { ref: PixPayment }
  Payment:
    type: union
    members:
      card: { ref: CardPayment }
  CardPayment:
    type: object
    properties:
      number: { type: string }
  CreditCardPayment:
    type: object
    properties:
      number: { type: string }
  PixPayment:
    type: object
    properties:
      key: { type: string }
`

const invalidUnionsContract = `provides:
  rest:
    /orders:
      get:
        responses:
          200: Order
schemas:
  Order:
    type: object
    properties:
      bare:
        type: union
      empty:
        type: union
        members: {}
      refless:
        type: union
        members:
          card: { type: object }
      named:
        type: union
        members:
          "card(x)": { ref: CardPayment }
      "total(brl)":
        type: string
  CardPayment:
    type: object
    properties:
      number: { type: string }
`

func (this *IntegrationSuite) mustPostForUnion(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) publishUnionOrdersProvider(version, cardRef string) {
	this.mustPostForUnion("/api/contracts", this.publishBody("shop_api", version,
		contractFragment{"api.yaml", fmt.Sprintf(unionOrdersProviderContract, cardRef)}))
}

func (this *IntegrationSuite) TestPublishContract_Union_ReadsBackEachMemberAsARefField() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, []repository.ProtocolTable{restrepository.New()})

	this.mustPostForUnion("/api/participants", `{"participant":"shop_api"}`)
	this.publishUnionOrdersProvider("1", "CardPayment")

	contract, found := repo.GetContractByNameAndVersion(ctx, "shop_api", "1")
	this.Require().True(found)
	this.Equal(model.Schema{
		"":              {Type: "object"},
		"payment":       {Type: "union"},
		"payment(card)": {Ref: "CardPayment"},
		"payment(pix)":  {Ref: "PixPayment"},
	}, contract.Schemas["Order"])
	this.Equal(model.Schema{
		"":       {Type: "union"},
		"(card)": {Ref: "CardPayment"},
	}, contract.Schemas["Payment"])
}

func (this *IntegrationSuite) TestPublishContract_UnionMemberChanged_WritesAModifiedFieldVersion() {
	ctx := context.Background()

	this.mustPostForUnion("/api/participants", `{"participant":"shop_api"}`)
	this.publishUnionOrdersProvider("1", "CardPayment")
	this.publishUnionOrdersProvider("2", "CreditCardPayment")

	rows, err := this.Pool.Query(ctx, `
		SELECT fv.change_type, fv.ref
		FROM field_versions fv
		JOIN fields f ON f.id = fv.field_id
		WHERE f.path = 'payment(card)'
		ORDER BY fv.contract_id`)
	this.Require().NoError(err)
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var changeType, ref string
		this.Require().NoError(rows.Scan(&changeType, &ref))
		versions = append(versions, changeType+": "+ref)
	}
	this.Require().NoError(rows.Err())

	this.Equal([]string{"added: CardPayment", "modified: CreditCardPayment"}, versions)
}

func (this *IntegrationSuite) TestPublishContract_InvalidUnions_RejectedWithEachViolation() {
	this.mustPostForUnion("/api/participants", `{"participant":"shop_api"}`)

	status, body := this.post("/api/contracts", this.publishBody("shop_api", "1",
		contractFragment{"api.yaml", invalidUnionsContract},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"members.required","path":"schemas;Order;properties;bare","source":"api.yaml","details":{"type":"union"}},`+
		`{"code":"members.empty","path":"schemas;Order;properties;empty;members","source":"api.yaml","details":{"type":"union"}},`+
		`{"code":"member.name_syntax","path":"schemas;Order;properties;named;members;card(x)","source":"api.yaml","details":{"key":"card(x)","error":"must not contain \".\", \"[\", \"]\", \"{\", \"}\", \";\", \"(\" or \")\""}},`+
		`{"code":"key.unknown","path":"schemas;Order;properties;refless;members;card;type","source":"api.yaml","details":{"key":"type"}},`+
		`{"code":"member.ref_required","path":"schemas;Order;properties;refless;members;card","source":"api.yaml","details":{"member":"card"}},`+
		`{"code":"property.name_syntax","path":"schemas;Order;properties;total(brl)","source":"api.yaml","details":{"key":"total(brl)","error":"must not contain \".\", \"[\", \"]\", \"{\", \"}\", \";\", \"(\" or \")\""}}`+
		`]}`, body)

	this.Equal(0, this.countRows("contracts"))
}

func (this *IntegrationSuite) TestPublishContract_UnionMemberRefToAnUndeclaredSchema_RejectedAtTheMember() {
	this.mustPostForUnion("/api/participants", `{"participant":"shop_api"}`)

	status, body := this.post("/api/contracts", this.publishBody("shop_api", "1",
		contractFragment{"api.yaml", fmt.Sprintf(unionOrdersProviderContract, "BoletoPayment")},
	))
	this.Equal(http.StatusBadRequest, status)
	this.JSONEq(`{"message":"contract validation failed","violations":[`+
		`{"code":"schema.unresolved_ref","path":"schemas;Order;properties;payment;members;card","source":"api.yaml","details":{"schema":"BoletoPayment","property":"Order.payment(card)"}}`+
		`]}`, body)
}

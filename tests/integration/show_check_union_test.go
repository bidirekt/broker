package integration_test

import (
	"context"
	"net/http"
	"strconv"
)

const unionCheckAPIContract = `provides:
  rest:
    /orders/*:
      get:
        responses:
          200: OrderRecord
schemas:
  OrderRecord:
    type: object
    properties:
      payment:
        type: union
        members:
          card: { ref: CardRecord }
          boleto: { ref: BoletoRecord }
  CardRecord:
    type: object
    properties:
      digits: { type: string }
  BoletoRecord:
    type: object
    properties:
      barcode: { type: string }
`

const unionCheckWebContract = `consumes:
  petstore_api:
    rest:
      /orders/*:
        get:
          responses:
            200: Order
schemas:
  Order:
    type: object
    properties:
      payment:
        type: union
        members:
          card: { ref: CardPayment }
          pix: { ref: PixPayment }
  CardPayment:
    type: object
    properties:
      number: { type: string }
  PixPayment:
    type: object
    properties:
      key: { type: string }
`

func (this *IntegrationSuite) mustPostForUnionCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) TestShowCheck_MissingUnionMemberIsAFieldBlockNamingTheMember() {
	this.mustPostForUnionCheck("/api/environments", `{"environment":"production"}`)
	this.mustPostForUnionCheck("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForUnionCheck("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForUnionCheck("/api/contracts", this.publishBody("petstore_api", "1.4.0", contractFragment{"api.yaml", unionCheckAPIContract}))
	this.mustPostForUnionCheck("/api/deployments", `{"participant":"petstore_api","version":"1.4.0","environment":"production"}`)
	this.mustPostForUnionCheck("/api/contracts", this.publishBody("petstore_web", "2.3.0", contractFragment{"web.yaml", unionCheckWebContract}))
	this.mustPostForUnionCheck("/api/can-i-deploy", `{"participant":"petstore_web","version":"2.3.0","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	status, _, body := this.getPage("/checks/"+strconv.FormatInt(checkID, 10), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="prop">FIELD</span><span>Order.payment</span><span class="badge">1 place</span>`)
	this.Contains(body, `<b class="name">petstore_web</b> reads &#34;pix&#34;, but <b class="name">petstore_api</b> doesn&#39;t provide it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>stop reading &#34;pix&#34;</span></div>`)
	this.Contains(body, `<div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>provide &#34;pix&#34; there</span></div>`)
	this.Contains(body, `<li><span class="meth">GET</span><span>/orders/*</span><span class="at">response 200</span><span>at &#34;$.payment&#34;</span><span class="cf">(petstore_api: OrderRecord.payment)</span></li>`)

	this.Contains(body, `<span class="prop">FIELD</span><span>CardPayment.number</span>`)
	this.Contains(body, `<span>at &#34;$.payment(card).number&#34;</span>`)
}

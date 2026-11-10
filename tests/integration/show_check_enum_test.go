package integration_test

import (
	"context"
	"net/http"
	"strconv"
)

const enumCheckAPIContract = `provides:
  rest:
    /pets:
      get:
        responses:
          200: PetRecords
      post:
        request: NewPetRecord
        responses:
          201: ~
    /pets/*:
      get:
        responses:
          200: PetRecord
schemas:
  PetRecords:
    type: array
    items: { ref: PetRecord }
  PetRecord:
    type: object
    properties:
      status: { type: enum, members: [available, pending] }
  NewPetRecord:
    type: object
    properties:
      status: { type: enum, members: [available, pending] }
`

const enumCheckWebContract = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pets
        post:
          request: NewPet
          responses:
            201: ~
      /pets/*:
        get:
          responses:
            200: Pet
schemas:
  Pets:
    type: array
    items: { ref: Pet }
  Pet:
    type: object
    properties:
      status: { type: enum, members: [adopted, available] }
  NewPet:
    type: object
    properties:
      status: { type: enum, members: [available, sold] }
`

func (this *IntegrationSuite) mustPostForEnumCheck(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) TestShowCheck_MissingEnumValuesAreFieldBlocksNamingTheValue() {
	this.mustPostForEnumCheck("/api/environments", `{"environment":"production"}`)
	this.mustPostForEnumCheck("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForEnumCheck("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForEnumCheck("/api/contracts", this.publishBody("petstore_api", "1.4.0", contractFragment{"api.yaml", enumCheckAPIContract}))
	this.mustPostForEnumCheck("/api/deployments", `{"participant":"petstore_api","version":"1.4.0","environment":"production"}`)
	this.mustPostForEnumCheck("/api/contracts", this.publishBody("petstore_web", "2.3.0", contractFragment{"web.yaml", enumCheckWebContract}))
	this.mustPostForEnumCheck("/api/can-i-deploy", `{"participant":"petstore_web","version":"2.3.0","environment":"production"}`)

	var checkID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT max(id) FROM compatibility_checks`).Scan(&checkID))

	status, _, body := this.getPage("/checks/"+strconv.FormatInt(checkID, 10), this.signInAsAdmin())

	this.Require().Equal(http.StatusOK, status, body)
	this.Contains(body, `<span class="prop">FIELD</span><span>Pet.status</span><span class="badge">2 places</span>`)
	this.Contains(body, `<b class="name">petstore_web</b> reads &#34;adopted&#34;, but <b class="name">petstore_api</b> doesn&#39;t provide it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>stop reading &#34;adopted&#34;</span></div>`)
	this.Contains(body, `<div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>provide &#34;adopted&#34; there</span></div>`)
	this.Contains(body, `<li><span class="meth">GET</span><span>/pets</span><span class="at">response 200</span><span>at &#34;$[].status&#34;</span><span class="cf">(petstore_api: PetRecord.status)</span></li>`)
	this.Contains(body, `<li><span class="meth">GET</span><span>/pets/*</span><span class="at">response 200</span><span>at &#34;$.status&#34;</span><span class="cf">(petstore_api: PetRecord.status)</span></li>`)

	this.Contains(body, `<span class="prop">FIELD</span><span>NewPet.status</span><span class="badge">1 place</span>`)
	this.Contains(body, `<b class="name">petstore_web</b> sends &#34;sold&#34;, but <b class="name">petstore_api</b> doesn&#39;t accept it`)
	this.Contains(body, `<div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>stop sending &#34;sold&#34;</span></div>`)
	this.Contains(body, `<div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>accept &#34;sold&#34; there</span></div>`)
	this.Contains(body, `<li><span class="meth">POST</span><span>/pets</span><span class="at">request</span><span>at &#34;$.status&#34;</span><span class="cf">(petstore_api: NewPetRecord.status)</span></li>`)
}

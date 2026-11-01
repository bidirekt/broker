package integration_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

const (
	inputDoorInvalidInput = "invalid input"
	inputDoorParticipant  = "pets"
	inputDoorEnvironment  = "production"
	inputDoorPassword     = "input-door-correct-horse"
	inputDoorContractYAML = "provides:\n  rest:\n    /pets:\n      get:\n        responses:\n          200: ~\n"
)

func (this *IntegrationSuite) TestInputDoor_RefusesTextThatIsNotUTF8OrCarriesNUL() {
	status, response := this.post("/api/participants", `{"participant":"`+inputDoorParticipant+`"}`)
	this.Require().Equal(http.StatusOK, status, response)
	status, response = this.post("/api/environments", `{"environment":"`+inputDoorEnvironment+`"}`)
	this.Require().Equal(http.StatusOK, status, response)
	status, response = this.post("/api/contracts", this.publishBody(inputDoorParticipant, "1", contractFragment{"api.yaml", inputDoorContractYAML}))
	this.Require().Equal(http.StatusOK, status, response)

	adminCookie := this.signInAsAdmin()
	_, _, usersPage := this.getPage("/users", adminCookie)
	csrfToken := this.csrfTokenFrom(usersPage)

	var writerID int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE email = $1`, suiteWriterEmail,
	).Scan(&writerID))

	pages := []struct {
		name  string
		send  func() (int, http.Header, string)
		table string
	}{
		{"sign-in identifier with an invalid UTF-8 byte", func() (int, http.Header, string) {
			return this.postForm("/sign-in", "", "", url.Values{"identifier": {"a\xff@x.io"}, "password": {inputDoorPassword}})
		}, "sessions"},
		{"sign-in identifier with NUL", func() (int, http.Header, string) {
			return this.postForm("/sign-in", "", "", url.Values{"identifier": {"a\x00@x.io"}, "password": {inputDoorPassword}})
		}, "sessions"},
		{"invite email with an invalid UTF-8 byte", func() (int, http.Header, string) {
			return this.postForm("/users/invite", adminCookie, csrfToken, url.Values{"email": {"a\xff@x.io"}, "permission": {"read"}})
		}, "users"},
		{"edit email with an invalid UTF-8 byte", func() (int, http.Header, string) {
			return this.postForm("/users/"+strconv.FormatInt(writerID, 10)+"/edit", adminCookie, csrfToken, url.Values{"email": {"a\xff@x.io"}, "permission": {"read"}})
		}, "users"},
		{"set-password token with an invalid UTF-8 byte and NUL", func() (int, http.Header, string) {
			return this.getPage("/set-password?token=%FF%00", "")
		}, "one_time_links"},
	}

	for _, page := range pages {
		this.Run(page.name, func() {
			rowsBefore := this.countRows(page.table)

			status, header, body := page.send()

			this.Equal(http.StatusBadRequest, status)
			this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
			this.Contains(body, "<h1>"+inputDoorInvalidInput+"</h1>")
			this.Equal(rowsBefore, this.countRows(page.table))
		})
	}

	var writerEmail string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT email FROM users WHERE id = $1`, writerID,
	).Scan(&writerEmail))
	this.Equal(suiteWriterEmail, writerEmail)

	apis := []struct {
		name  string
		path  string
		token string
		body  string
		table string
	}{
		{"sign-in email with NUL", "/api/sign-in", "",
			`{"email":"a\u0000@x.io","password":"` + inputDoorPassword + `"}`, "sessions"},
		{"environment with an invalid UTF-8 byte", "/api/environments", this.writerToken,
			"{\"environment\":\"a\xff\"}", "environments"},
		{"environment with NUL", "/api/environments", this.writerToken,
			`{"environment":"a\u0000"}`, "environments"},
		{"environment with NUL after an escaped backslash", "/api/environments", this.writerToken,
			`{"environment":"a\\\u0000"}`, "environments"},
		{"rename oldName with NUL", "/api/participants/rename", this.writerToken,
			`{"oldName":"a\u0000","newName":"b"}`, "participants"},
		{"deployment version with NUL", "/api/deployments", this.writerToken,
			`{"participant":"pets","version":"a\u0000","environment":"production"}`, "deployments"},
		{"can-i-deploy environment with NUL", "/api/can-i-deploy", this.writerToken,
			`{"participant":"pets","version":"1","environment":"a\u0000"}`, "compatibility_checks"},
		{"validate participant with NUL", "/api/contracts/validate", this.writerToken,
			`{"participant":"a\u0000","environment":"production","contracts":[{"source":"api.yaml","content":` + strconv.Quote(inputDoorContractYAML) + `}]}`, "compatibility_checks"},
		{"publish version with NUL", "/api/contracts", this.writerToken,
			this.publishBody(inputDoorParticipant, "a\x00", contractFragment{"api.yaml", inputDoorContractYAML}), "contract_versions"},
		{"publish source with NUL", "/api/contracts", this.writerToken,
			this.publishBody(inputDoorParticipant, "2", contractFragment{"a\x00.yaml", inputDoorContractYAML}), "contract_versions"},
	}

	for _, api := range apis {
		this.Run(api.name, func() {
			rowsBefore := this.countRows(api.table)

			status, _, body := this.postWithToken(api.path, api.token, api.body)

			this.Equal(http.StatusBadRequest, status)
			this.JSONEq(`{"message":"`+inputDoorInvalidInput+`"}`, body)
			this.Equal(rowsBefore, this.countRows(api.table))
		})
	}
}

func (this *IntegrationSuite) TestInputDoor_AcceptsAnEscapedBackslashBeforeU0000() {
	status, body := this.post("/api/environments", `{"environment":"a\\u0000"}`)

	this.Equal(http.StatusOK, status, body)

	var name string
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT name FROM environments`).Scan(&name))
	this.Equal(`a\u0000`, name)
}

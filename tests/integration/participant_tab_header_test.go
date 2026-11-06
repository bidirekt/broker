package integration_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

const participantTabHeaderContract = `provides:
  rest:
    /orders:
      get:
        responses:
          200: ~
`

var participantTabHeaderTabs = []struct {
	name    string
	path    string
	current string
}{
	{"versions", "", "Versions"},
	{"deployments", "/deployments", "Deployments"},
	{"checks", "/checks", "Checks"},
	{"relations", "/relations", "Relations"},
}

func (this *IntegrationSuite) mustPostForParticipantTabHeader(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) seedForParticipantTabHeader() {
	for _, environment := range []string{"staging", "production"} {
		this.mustPostForParticipantTabHeader("/api/environments", `{"environment":"`+environment+`"}`)
	}
	for _, participant := range []string{"orders_web", "orders_api", "orders_admin"} {
		this.mustPostForParticipantTabHeader("/api/participants", `{"participant":"`+participant+`"}`)
	}
	for _, published := range []struct{ participant, version string }{
		{"orders_web", "1.0.0"},
		{"orders_web", "1.0.0+build"},
		{"orders_api", "2.0.0"},
	} {
		this.mustPostForParticipantTabHeader("/api/contracts", this.publishBody(published.participant, published.version, contractFragment{"contract.yaml", participantTabHeaderContract}))
	}
	for _, deployed := range []struct{ version, environment string }{
		{"1.0.0", "production"},
		{"1.0.0+build", "staging"},
	} {
		this.mustPostForParticipantTabHeader("/api/deployments", `{"participant":"orders_web","version":"`+deployed.version+`","environment":"`+deployed.environment+`"}`)
	}
	this.mustPostForParticipantTabHeader("/api/can-i-deploy", `{"participant":"orders_web","version":"1.0.0+build","environment":"production"}`)
}

func (this *IntegrationSuite) environmentPathForParticipantTabHeader(name string) string {
	var id int64
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT id FROM environments WHERE name = $1`, name).Scan(&id))

	return "/environments/" + strconv.FormatInt(id, 10)
}

func participantTabHeaderNav(participant, current string, versions, deployments, checks, relations int) string {
	link := func(path, label string, count int) string {
		ariaCurrent := ""
		if label == current {
			ariaCurrent = ` aria-current="page"`
		}

		return `  <a href="/participants/` + participant + path + `"` + ariaCurrent + `>` + label + ` <span class="count">` + strconv.Itoa(count) + `</span></a>` + "\n"
	}

	return `<nav class="tabs" aria-label="Participant">` + "\n" +
		link("", "Versions", versions) +
		link("/deployments", "Deployments", deployments) +
		link("/checks", "Checks", checks) +
		link("/relations", "Relations", relations) +
		`</nav>`
}

func (this *IntegrationSuite) TestParticipantTabHeader_EveryTabRendersTheSameHeader() {
	this.seedForParticipantTabHeader()
	production := this.environmentPathForParticipantTabHeader("production")
	staging := this.environmentPathForParticipantTabHeader("staging")
	sessionCookie := this.signInAsAdmin()

	participants := []struct {
		name   string
		head   string
		counts [4]int
	}{
		{
			name: "orders_web",
			head: strings.Join([]string{
				`<div class="head">`,
				`  <div>`,
				`    <h1 class="mono">orders_web</h1>`,
				`    `,
				`    <div class="sub badges">`,
				`      <span>Latest <a class="ver" href="/participants/orders_web/versions/1.0.0&#43;build">1.0.0&#43;build</a></span>`,
				`      <span aria-hidden="true">·</span>`,
				`      <a class="badge env" href="` + production + `">production <span class="v">1.0.0</span></a><a class="badge env" href="` + staging + `">staging <span class="v">1.0.0&#43;build</span></a>`,
				`    </div>`,
				`    `,
				`  </div>`,
				`</div>`,
			}, "\n"),
			counts: [4]int{2, 2, 1, 0},
		},
		{
			name: "orders_api",
			head: strings.Join([]string{
				`<div class="head">`,
				`  <div>`,
				`    <h1 class="mono">orders_api</h1>`,
				`    `,
				`    <div class="sub badges">`,
				`      <span>Latest <a class="ver" href="/participants/orders_api/versions/2.0.0">2.0.0</a></span>`,
				`      <span aria-hidden="true">·</span>`,
				`      <span class="badge quiet">not deployed</span>`,
				`    </div>`,
				`    `,
				`  </div>`,
				`</div>`,
			}, "\n"),
			counts: [4]int{1, 0, 0, 0},
		},
		{
			name: "orders_admin",
			head: strings.Join([]string{
				`<div class="head">`,
				`  <div>`,
				`    <h1 class="mono">orders_admin</h1>`,
				`    `,
				`    <p class="sub">never published</p>`,
				`    `,
				`  </div>`,
				`</div>`,
			}, "\n"),
			counts: [4]int{0, 0, 0, 0},
		},
	}

	for _, participant := range participants {
		for _, tab := range participantTabHeaderTabs {
			this.Run(participant.name+"/"+tab.name, func() {
				status, _, body := this.getPage("/participants/"+participant.name+tab.path, sessionCookie)

				this.Require().Equal(http.StatusOK, status, body)
				this.Equal(1, strings.Count(body, participant.head+"\n"+participantTabHeaderNav(participant.name, tab.current, participant.counts[0], participant.counts[1], participant.counts[2], participant.counts[3])), body)
			})
		}
	}
}

package integration_test

import (
	"net/http"
	"regexp"
	"strings"
)

var (
	layoutInlineStyle  = regexp.MustCompile(`<style|\sstyle=`)
	layoutInlineScript = regexp.MustCompile(`<script[^>]*>[^<]`)
	layoutExternalLink = regexp.MustCompile(`(?:src|href)="(?:https?:)?//`)
)

const (
	layoutBrand   = `<a class="brand" href="/"><span class="mark" aria-hidden="true">🤝</span>Bidirekt</a>`
	layoutBigMark = `<span class="mark big" aria-hidden="true">🤝</span>`
)

func (this *IntegrationSuite) TestLayout_SignedInPageIsDarkWithTheTopBar() {
	cookie := this.signInAsAdmin()

	status, _, body := this.getPage("/users", cookie)

	this.Equal(http.StatusOK, status)
	this.Contains(body, `<html lang="en" data-theme="dark">`)
	this.Contains(body, `<body class="bk" hx-boost="true" hx-headers='{"X-CSRF-Token": "`)
	this.Contains(body, `<link rel="stylesheet" href="/assets/bundle.css">`)
	this.Contains(body, layoutBrand)
	this.NotContains(body, layoutBigMark)
	this.Contains(body, `<span class="email">`+suiteAdminEmail+`</span>`)
	this.Contains(body, `<button type="submit" class="linkbtn">Sign out</button>`)
	this.Contains(body, `<a href="/participants">Participants</a>`)
	this.Contains(body, `<a href="/environments">Environments</a>`)
	this.Contains(body, `<a href="/compatibility">Compatibility</a>`)
	this.assertLayoutIsSelfContained(body)
}

func (this *IntegrationSuite) TestLayout_SignedOutPageIsLightWithOnlyTheBrand() {
	status, _, body := this.getPage("/sign-in", "")

	this.Equal(http.StatusOK, status)
	this.Contains(body, `<html lang="en" data-theme="light">`)
	this.Contains(body, `<body class="bk" hx-boost="true">`)
	this.Contains(body, `<link rel="stylesheet" href="/assets/bundle.css">`)
	this.Contains(body, layoutBrand)
	this.Contains(body, `<div class="auth-brand">
  `+layoutBigMark+`
  <h1>Sign in to Bidirekt</h1>`)
	this.NotContains(body, `class="nav"`)
	this.NotContains(body, `class="email"`)
	this.assertLayoutIsSelfContained(body)
}

func (this *IntegrationSuite) assertLayoutIsSelfContained(body string) {
	this.NotRegexp(layoutInlineStyle, body)
	this.NotRegexp(layoutInlineScript, body)
	this.Equal(strings.Count(body, "<script"), strings.Count(body, `<script src="/assets/`))
	this.NotRegexp(layoutExternalLink, body)
}

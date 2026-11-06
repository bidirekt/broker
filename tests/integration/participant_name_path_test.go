package integration_test

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

const participantNamePathNotFound = "<h1>Not Found</h1>"

func (this *IntegrationSuite) sendRawParticipantNamePathRequest(request string) (status int, header http.Header, body string) {
	client, server := net.Pipe()
	served := make(chan error, 1)
	go func() { served <- this.Components.Server.Server().ServeConn(server) }()
	defer func() {
		_ = client.Close()
		<-served
	}()
	go func() { _, _ = client.Write([]byte(request)) }()

	response, err := http.ReadResponse(bufio.NewReader(client), nil)
	this.Require().NoError(err)
	content, err := io.ReadAll(response.Body)
	this.Require().NoError(err)

	return response.StatusCode, response.Header, string(content)
}

func (this *IntegrationSuite) TestParticipantNamePath_AnInvalidUTF8ByteInTheNameIsNotFoundOnEveryRoute() {
	sessionCookie := this.signInAsUser(suiteWriterEmail, suiteWriterPassword)
	_, _, participantsPage := this.getPage("/participants", sessionCookie)
	csrfToken := this.csrfTokenFrom(participantsPage)

	signedIn := "Host: localhost:8080\r\n" +
		"Accept: text/html\r\n" +
		"Cookie: " + suiteSessionCookieName + "=" + sessionCookie + "\r\n"
	form := url.Values{"name": {"spirit"}}.Encode()

	routes := []struct {
		name    string
		request string
	}{
		{"versions", "GET /participants/pets\xff HTTP/1.1\r\n" + signedIn + "\r\n"},
		{"deployments", "GET /participants/pets\xff/deployments HTTP/1.1\r\n" + signedIn + "\r\n"},
		{"checks", "GET /participants/pets\xff/checks HTTP/1.1\r\n" + signedIn + "\r\n"},
		{"relations", "GET /participants/pets\xff/relations HTTP/1.1\r\n" + signedIn + "\r\n"},
		{"version", "GET /participants/pets\xff/versions/1.0.0 HTTP/1.1\r\n" + signedIn + "\r\n"},
		{"rename form", "GET /participants/pets\xff/rename HTTP/1.1\r\n" + signedIn + "\r\n"},
		{"rename", "POST /participants/pets\xff/rename HTTP/1.1\r\n" + signedIn +
			"Origin: " + suitePublicURL + "\r\n" +
			"X-CSRF-Token: " + csrfToken + "\r\n" +
			"Content-Type: application/x-www-form-urlencoded\r\n" +
			"Content-Length: " + strconv.Itoa(len(form)) + "\r\n" +
			"\r\n" + form},
	}

	for _, route := range routes {
		this.Run(route.name, func() {
			status, header, body := this.sendRawParticipantNamePathRequest(route.request)

			this.Equal(http.StatusNotFound, status)
			this.Equal("text/html; charset=utf-8", header.Get("Content-Type"))
			this.Contains(body, participantNamePathNotFound)
		})
	}
}

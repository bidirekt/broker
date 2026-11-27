package graphql_test

import (
	"html/template"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/components/breakreport"
	graphqlbreakreport "github.com/bidirekt/broker/internal/components/breakreport/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
)

func TestPlugin_IsTheGraphQLOperationsSection(t *testing.T) {
	section := graphqlbreakreport.New()

	assert.Equal(t, model.Protocol("graphql"), section.Protocol())
	assert.Equal(t, "operations", section.ReportKey())
}

func TestFiles_HoldTheStylesheetAndTheTemplate(t *testing.T) {
	section := graphqlbreakreport.New()

	_, err := fs.Stat(section.Files(), "operations.css")
	require.NoError(t, err)

	templates, err := template.ParseFS(section.Files(), "*.html")
	require.NoError(t, err)
	assert.NotNil(t, templates.Lookup(section.Template()))
}

func TestToSection_DrawsNoBreak(t *testing.T) {
	section, err := graphqlbreakreport.New().ToSection(breakreport.CheckedCounterpart{}, []byte(`{}`))

	require.NoError(t, err)
	assert.Zero(t, section.Breaks)
	assert.Zero(t, section.Places)
}

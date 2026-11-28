package graphql_test

import (
	"html/template"
	"io/fs"
	"strings"
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

func TestToSection_DrawsNoBreakWithoutATree(t *testing.T) {
	section, err := graphqlbreakreport.New().ToSection(breakreport.CheckedCounterpart{}, nil)

	require.NoError(t, err)
	assert.Zero(t, section.Breaks)
	assert.Zero(t, section.Places)
}

func TestToSection_CountsOperationBreaksAndFieldBlocks(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_web", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_api"}}
	tree := []byte(`{"query": {
		"owner": {"returns": [{"reason": "provider_resource_not_found", "role": "consumer"}]},
		"pet": {
			"arguments": [{"reason": "property_missing_in_provider", "role": "consumer", "details": {"property": "$", "argument": "owner", "consumerField": "Query.pet(owner:)"}}],
			"returns": [{"reason": "property_missing_in_provider", "role": "consumer", "details": {"property": "$.reviews", "argument": "first", "consumerField": "ReviewsArgs.first"}}]
		}
	}}`)

	section, err := graphqlbreakreport.New().ToSection(checked, tree)

	require.NoError(t, err)
	assert.Equal(t, 3, section.Breaks)
	assert.Equal(t, 2, section.Places)

	display := section.Display.(graphqlbreakreport.Operations)
	require.Len(t, display.Operations, 1)
	assert.Equal(t, "petstore_web calls query owner, but petstore_api doesn't provide it", display.Operations[0].Breaks[0].Sentence.String())
	require.Len(t, display.Fields, 2)
	assert.Equal(t, "Query.pet(owner:)", display.Fields[0].Field)
	assert.Equal(t, []graphqlbreakreport.Place{{Operation: "query", Field: "pet", Interaction: "arguments", Argument: `"owner"`}}, display.Fields[0].Places)
	assert.Equal(t, "ReviewsArgs.first", display.Fields[1].Field)
	assert.Equal(t, "petstore_web sends it, but petstore_api doesn't accept it", display.Fields[1].Sentence.String())
	assert.Equal(t, []graphqlbreakreport.Place{{Operation: "query", Field: "pet", Interaction: "returns", Property: `"$.reviews"`, Argument: `"first"`}}, display.Fields[1].Places)
}

func TestToSection_ANestedArgumentBreakOfTheCheckedProviderReadsAsAcceptedArguments(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_api", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_web"}}
	tree := []byte(`{"query": {"pet": {"returns": [
		{"reason": "property_missing_in_provider", "role": "provider", "details": {"property": "$.reviews", "argument": "first", "consumerField": "ReviewsArgs.first"}}
	]}}}`)

	section, err := graphqlbreakreport.New().ToSection(checked, tree)

	require.NoError(t, err)
	display := section.Display.(graphqlbreakreport.Operations)
	require.Len(t, display.Fields, 1)
	assert.Equal(t, "petstore_api doesn't accept it, but petstore_web sends it", display.Fields[0].Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_api", Action: "accept it", Checked: true},
		{Participant: "petstore_web", Action: "stop sending it there"},
	}, display.Fields[0].Fixes)
}

func TestToSection_ResourceBreaksReadAsTheOperation(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_web", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_api"}}

	notDeployed, err := graphqlbreakreport.New().ToSection(checked, []byte(`{"query": {"pet": {"returns": [{"reason": "provider_resource_not_deployed_in_environment", "role": "consumer"}]}}}`))
	require.NoError(t, err)
	operation := notDeployed.Display.(graphqlbreakreport.Operations).Operations[0]
	assert.Equal(t, "petstore_web calls query pet, but petstore_api is not deployed in production", operation.Breaks[0].Sentence.String())
	assert.Equal(t, []breakreport.Fix{{Participant: "petstore_api", Action: "deploy it to production first"}}, operation.Breaks[0].Fixes)

	removed, err := graphqlbreakreport.New().ToSection(checked, []byte(`{"query": {"pet": {"returns": [{"reason": "provider_resource_removed_but_still_consumed", "role": "provider"}]}}}`))
	require.NoError(t, err)
	operation = removed.Display.(graphqlbreakreport.Operations).Operations[0]
	assert.Equal(t, "petstore_web removed query pet, but petstore_api still calls it", operation.Breaks[0].Sentence.String())
	assert.Equal(t, []breakreport.Fix{
		{Participant: "petstore_web", Action: "keep it until petstore_api stops calling it", Checked: true},
		{Participant: "petstore_api", Action: "stop calling it"},
	}, operation.Breaks[0].Fixes)
}

func TestToSection_AGraphThatDoesNotCompose_RendersEveryCauseAndLinksTheDeployedGraph(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_web", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_graph"}}
	tree := []byte(`{"query": {"pet": {"returns": [{"reason": "provider_graph_not_composable", "role": "consumer", "details": {
		"graph": "petstore_graph",
		"cause": "petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable",
		"causes": "petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable\npetstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.currency without @shareable"
	}}]}}}`)

	section, err := graphqlbreakreport.New().ToSection(checked, tree)
	require.NoError(t, err)

	page := renderOperations(t, section)
	assert.Contains(t, page, `<ul class="op-causes"><li>petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable</li><li>petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.currency without @shareable</li></ul>`)
	assert.Contains(t, page, `<p class="op-see">Deployed graph: <a href="/graphs/petstore_graph">/graphs/petstore_graph</a></p>`)
	assert.NotContains(t, page, "See ")
}

func TestToSection_AGraphWithMoreThanFiveCauses_RendersTheFirstFiveAndCountsTheRest(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_web", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_graph"}}
	tree := []byte(`{"query": {"pet": {"returns": [{"reason": "provider_graph_not_composable", "role": "consumer", "details": {
		"graph": "petstore_graph",
		"cause": "failure 1",
		"causes": "failure 1\nfailure 2\nfailure 3\nfailure 4\nfailure 5\nfailure 6\nfailure 7"
	}}]}}}`)

	section, err := graphqlbreakreport.New().ToSection(checked, tree)
	require.NoError(t, err)

	page := renderOperations(t, section)
	assert.Contains(t, page, `<ul class="op-causes"><li>failure 1</li><li>failure 2</li><li>failure 3</li><li>failure 4</li><li>failure 5</li><li>and 2 more</li></ul>`)
	assert.NotContains(t, page, "failure 6")
}

func TestToSection_AGraphWithExactlyFiveCauses_RendersThemAllWithoutAMoreLine(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_web", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_graph"}}
	tree := []byte(`{"query": {"pet": {"returns": [{"reason": "provider_graph_not_composable", "role": "consumer", "details": {
		"graph": "petstore_graph",
		"cause": "failure 1",
		"causes": "failure 1\nfailure 2\nfailure 3\nfailure 4\nfailure 5"
	}}]}}}`)

	section, err := graphqlbreakreport.New().ToSection(checked, tree)
	require.NoError(t, err)

	page := renderOperations(t, section)
	assert.Contains(t, page, `<ul class="op-causes"><li>failure 1</li><li>failure 2</li><li>failure 3</li><li>failure 4</li><li>failure 5</li></ul>`)
	assert.NotContains(t, page, " more</li>")
}

func TestToSection_AGraphBreakWithoutCauses_ShowsItsOnlyCause(t *testing.T) {
	checked := breakreport.CheckedCounterpart{Participant: "petstore_web", Environment: "production", Counterpart: breakreport.Counterpart{Name: "petstore_graph"}}
	tree := []byte(`{"query": {"pet": {"returns": [{"reason": "provider_graph_not_composable", "role": "consumer", "details": {
		"graph": "petstore_graph",
		"cause": "petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable"
	}}]}}}`)

	section, err := graphqlbreakreport.New().ToSection(checked, tree)
	require.NoError(t, err)

	broken := section.Display.(graphqlbreakreport.Operations).Operations[0].Breaks[0]
	assert.Equal(t, "petstore_web calls query pet, but petstore_graph can't be built in production", broken.Sentence.String())
	assert.Equal(t, []string{"petstore_api 1.4.0 and petstore_orders 1.0.0 both resolve Money.amount without @shareable"}, broken.Causes)
}

func renderOperations(t *testing.T, section breakreport.Section) string {
	t.Helper()

	plugin := graphqlbreakreport.New()
	templates := template.Must(template.New("partials").Parse(
		`{{define "icon_x"}}{{end}}{{define "fixes"}}{{end}}{{define "phrase"}}{{range .}}{{.Text}}{{end}}{{end}}`,
	))
	templates = template.Must(templates.ParseFS(plugin.Files(), "*.html"))

	var page strings.Builder
	require.NoError(t, templates.ExecuteTemplate(&page, plugin.Template(), section.Display))

	return page.String()
}

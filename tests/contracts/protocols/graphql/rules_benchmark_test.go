package graphql_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contractfiles"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
)

func restContractOfElevenThousandSchemaNodes() string {
	var source strings.Builder

	source.WriteString("provides:\n  rest:\n")
	for thing := range 100 {
		fmt.Fprintf(&source, "    /things%d:\n      get:\n        responses:\n          200: Thing%d\n", thing, thing)
	}

	source.WriteString("schemas:\n")
	for thing := range 100 {
		fmt.Fprintf(&source, "  Thing%d:\n    type: object\n    properties:\n", thing)

		for part := range 10 {
			fmt.Fprintf(&source, "      part%d:\n        type: object\n        properties:\n", part)

			for tags := range 5 {
				fmt.Fprintf(&source, "          tags%d:\n            type: array\n            items:\n              type: string\n", tags)
			}
		}
	}

	return source.String()
}

func BenchmarkToViolations_RESTContractOfElevenThousandSchemaNodes(b *testing.B) {
	files := []model.ContractFile{{Source: "things.yaml", Content: restContractOfElevenThousandSchemaNodes()}}

	registrations := []struct {
		name        string
		interpreter *contractfiles.Interpreter
	}{
		{"REST only", contractfiles.NewInterpreter(protocol.NewRegistry([]protocol.Protocol{restcontracts.New()}), nil)},
		{"REST and GraphQL", contractfiles.NewInterpreter(
			protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), graphqlcontracts.New()}),
			[]contractfiles.FileFormat{graphqlcontracts.NewSDLFormat()},
		)},
	}

	for _, registration := range registrations {
		b.Run(registration.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				violations, readErr := registration.interpreter.ToViolations(files)
				if readErr != nil || len(violations) > 0 {
					b.Fatal(readErr, violations)
				}
			}
		})
	}
}

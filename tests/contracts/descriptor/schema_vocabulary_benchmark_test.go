package descriptor_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
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

func BenchmarkValidate_RESTContractOfElevenThousandSchemaNodes(b *testing.B) {
	grammar := protocol.NewRegistry([]protocol.Protocol{restcontracts.New()}).Grammar()

	fragment, err := contract.DecodeFragment("things.yaml", restContractOfElevenThousandSchemaNodes())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if violations := descriptor.Validate(grammar, fragment.Document, fragment.Source); len(violations) > 0 {
			b.Fatal(violations)
		}
	}
}

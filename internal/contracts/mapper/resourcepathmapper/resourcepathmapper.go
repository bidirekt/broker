package resourcepathmapper

import (
	"fmt"
	"regexp"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/model"
)

var (
	consumerRestRequestRegex = regexp.MustCompile(
		`^consumes;(?P<provider>[^;]*);rest;(?P<endpoint>[^;]+);(?P<method>[^;]+);request$`,
	)
	consumerRestResponseRegex = regexp.MustCompile(
		`^consumes;(?P<provider>[^;]*);rest;(?P<endpoint>[^;]+);(?P<method>[^;]+);responses;(?P<status>\d+)$`,
	)
	providerRestRequestRegex = regexp.MustCompile(
		`^provides;rest;(?P<endpoint>[^;]+);(?P<method>[^;]+);request$`,
	)
	providerRestResponseRegex = regexp.MustCompile(
		`^provides;rest;(?P<endpoint>[^;]+);(?P<method>[^;]+);responses;(?P<status>\d+)$`,
	)
)

func ToResourceModel(path contract.ResourcePath, properties map[string]model.Property) model.UploadedResource {
	if args, ok := path.ExtractNamedArgs(consumerRestRequestRegex); ok {
		return *model.NewRestRequestConsumer(
			args["provider"],
			args["endpoint"],
			args["method"],
			properties,
		)
	}

	if args, ok := path.ExtractNamedArgs(consumerRestResponseRegex); ok {
		return *model.NewRestResponseConsumer(
			args["provider"],
			args["endpoint"],
			args["method"],
			args["status"],
			properties,
		)
	}

	if args, ok := path.ExtractNamedArgs(providerRestRequestRegex); ok {
		return *model.NewRestRequestProvider(
			args["endpoint"],
			args["method"],
			properties,
		)
	}

	if args, ok := path.ExtractNamedArgs(providerRestResponseRegex); ok {
		return *model.NewRestResponseProvider(
			args["endpoint"],
			args["method"],
			args["status"],
			properties,
		)
	}

	panic(fmt.Errorf("unrecognized resource path: %q", path.String()))
}

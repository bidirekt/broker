package versions

import (
	"fmt"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/components/respond"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/mod/semver"
)

const cliVersionHeader = "Bidirekt-Cli-Version"

func Door(ctx fiber.Ctx) error {
	if !pages.IsAPIPath(ctx.Path()) {
		return ctx.Next()
	}

	cliVersion := ctx.Get(cliVersionHeader)
	if !isBelowMinimumCli(cliVersion) {
		return ctx.Next()
	}

	message := fmt.Sprintf("bidirekt %s doesn't work with this broker (%s); upgrade bidirekt to %s or later", cliVersion, Broker(), MinimumCli)
	return respond.Problem(ctx, fiber.StatusUpgradeRequired, message)
}

func isBelowMinimumCli(cliVersion string) bool {
	semanticVersion := "v" + cliVersion
	return semver.IsValid(semanticVersion) && semver.Compare(semanticVersion, "v"+MinimumCli) < 0
}

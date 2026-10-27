# broker

[![CI](https://img.shields.io/github/actions/workflow/status/bidirekt/broker/ci.yml?branch=main&label=CI)](https://github.com/bidirekt/broker/actions/workflows/ci.yml) [![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/bidirekt/broker/badges/coverage.json)](https://github.com/bidirekt/broker/actions/workflows/ci.yml) [![Release](https://img.shields.io/github/v/release/bidirekt/broker)](https://github.com/bidirekt/broker/releases) [![License](https://img.shields.io/badge/license-BSL_1.1-blue)](LICENSE) [![Go](https://img.shields.io/github/go-mod/go-version/bidirekt/broker)](go.mod)

The broker is the server of Bidirekt, a declarative, static, bidirectional contract testing tool. Each participant declares the resources it provides and consumes in a contract and publishes it with the `bidirekt` CLI; the broker stores every contract in its Postgres database. Before a deployment, `bidirekt can-i-deploy` asks the broker whether a version can go to an environment: the broker compares that version's contract with the contracts of its consumers and providers deployed there, so no service needs to be running. After the deployment, the pipeline records it, so the broker knows which version runs in each environment.

## Run it

The broker ships as the Docker image `ghcr.io/bidirekt/broker` and needs one Postgres database:

```sh
docker pull ghcr.io/bidirekt/broker
```

To try it on your machine, the `docker-compose.yaml` in this repository starts Postgres and the broker on `localhost:8080`. In an empty folder:

```sh
curl -O https://raw.githubusercontent.com/bidirekt/broker/main/docker-compose.yaml
docker compose up -d
curl -s http://localhost:8080/health
```

For production, configuration and the health check: https://bidirekt.com/docs/start/run-the-broker/

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for issues, pull requests, the CLA and security reports. Point Git at the repository's hooks once:

```sh
git config core.hooksPath .githooks
```

The `pre-push` hook runs the same gates as CI (`gofmt`, `go vet`, `golangci-lint`, `go-arch-lint`, `go test`, the CLA tests, `govulncheck`) before every push.

The tests need Docker: the integration and migrator tests start Postgres with testcontainers.

## License

Source-available under the [Business Source License 1.1](LICENSE) — the same
license HashiCorp and MariaDB use.

In plain terms: read, copy, modify and self-host it for any purpose, commercial
work included. The one thing you may not do is offer it — or something
substantially like it — to others as a competing commercial product or service.
Every version turns into plain Apache License 2.0 four years after its release.

Need a competing use? A commercial license is available:
alefaraujocastelo@gmail.com

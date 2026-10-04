FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG VERSION=dev
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -ldflags "-X github.com/bidirekt/broker/internal/features/health.brokerVersion=${VERSION}" \
      -o /out/broker ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/broker /broker
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD ["/broker", "healthcheck"]
ENTRYPOINT ["/broker"]

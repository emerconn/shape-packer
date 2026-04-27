# BUILD STAGE
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG MODE=slim

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN if [ "$MODE" = "slim" ]; then \
	CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -trimpath -o polygon-packer .; \
	else \
	CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o polygon-packer .; \
	fi

# SLIM RUNTIME
FROM gcr.io/distroless/static AS runtime-slim
COPY --from=build /app/polygon-packer /polygon-packer
ENTRYPOINT ["/polygon-packer"]

# DEBUG RUNTIME
FROM alpine:3.19 AS runtime-debug
RUN apk add --no-cache bash
COPY --from=build /app/polygon-packer /polygon-packer
ENTRYPOINT ["/polygon-packer"]

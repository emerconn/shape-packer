# BUILD STAGE
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETARCH
ARG TARGETOS
ARG MODE

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN if [ "$MODE" = "slim" ]; then \
    CGO_ENABLED=0 GOAMD64=v3 GOARM64=v8.0,lse,crypto GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -trimpath -o polygon-packer . ; \
    else \
    CGO_ENABLED=0 GOAMD64=v3 GOARM64=v8.0,lse,crypto GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o polygon-packer . ; \
    fi

# SLIM RUNTIME
FROM scratch AS runtime-slim
COPY --from=build /app/polygon-packer /polygon-packer
ENTRYPOINT ["/polygon-packer"]

# DEBUG RUNTIME
FROM alpine AS runtime-debug
COPY --from=build /app/polygon-packer /polygon-packer
ENTRYPOINT ["/polygon-packer"]

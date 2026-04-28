# BUILD STAGE
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w -X main.version=${VERSION}" -trimpath -o polygon-packer-slim .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-X main.version=${VERSION}" -o polygon-packer-debug .


# SLIM RUNTIME
FROM gcr.io/distroless/static AS runtime-slim
COPY --from=build /app/polygon-packer-slim /polygon-packer
ENTRYPOINT ["/polygon-packer"]

# DEBUG RUNTIME
FROM gcr.io/distroless/static AS runtime-debug
COPY --from=build /app/polygon-packer-debug /polygon-packer
ENTRYPOINT ["/polygon-packer"]

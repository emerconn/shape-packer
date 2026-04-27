# BUILD STAGE
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
	-ldflags="-s -w" \
	-trimpath \
	-o polygon-packer .

# FINAL STAGE
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=build /app/polygon-packer /polygon-packer

ENTRYPOINT ["/polygon-packer"]

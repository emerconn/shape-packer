FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod ./
COPY *.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
	-ldflags="-s -w" \
	-trimpath \
	-o /out/polygon-packer \
	.

FROM alpine:3.20

RUN adduser -D -u 10001 appuser \
	&& mkdir -p /work \
	&& chown appuser:appuser /work

WORKDIR /work
COPY --from=build /out/polygon-packer /usr/local/bin/polygon-packer

USER appuser
ENTRYPOINT ["polygon-packer"]

# BUILD STAGE
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETARCH
ARG TARGETOS
ARG MODE

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN ./scripts/build.sh ${TARGETOS:-linux} ${TARGETARCH:-amd64} ${MODE:-slim}

# SLIM RUNTIME
FROM scratch AS runtime-slim
COPY --from=build /app/polygon-packer /polygon-packer
ENTRYPOINT ["/polygon-packer"]

# DEBUG RUNTIME
FROM alpine AS runtime-debug
COPY --from=build /app/polygon-packer /polygon-packer
ENTRYPOINT ["/polygon-packer"]

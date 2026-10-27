# fleetwatch: static Go binary on alpine. The test suite runs in the build
# stage, so an image cannot be built from code whose tests fail.
# Base images come from the public registry, pinned by digest. A build that
# needs a mirror overrides them: --build-arg GO_IMAGE=... --build-arg ALPINE_IMAGE=...
ARG GO_IMAGE=golang:1.27.3-alpine3.24@sha256:d530cf60723abf1ffae0cd16bb546b379ec0550d2f3815e5b16ed69dbc00fba4
ARG ALPINE_IMAGE=alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY . .
# BUILD_TAGS selects optional test sets (comma separated); empty runs the
# default suite.
ARG BUILD_TAGS=
RUN CGO_ENABLED=0 go vet -tags "$BUILD_TAGS" ./... && CGO_ENABLED=0 go test -tags "$BUILD_TAGS" ./... \
 && CGO_ENABLED=0 go build -trimpath -tags "$BUILD_TAGS" -ldflags="-s -w" -o /out/fleetwatch ./cmd/fleetwatch

FROM ${ALPINE_IMAGE}
RUN apk upgrade --no-cache
ARG APP_COMMIT=unknown
# The version is the embedded VERSION file; only the commit is a build argument.
ENV APP_COMMIT=${APP_COMMIT}
COPY --from=build /out/fleetwatch /usr/local/bin/fleetwatch
# Runs as the host user (compose sets user: 1000:1000): the transcripts, the
# pool slots' .claude.json and the hook files are all owned by that uid.
USER 1000:1000
EXPOSE 8909
ENTRYPOINT ["/usr/local/bin/fleetwatch"]

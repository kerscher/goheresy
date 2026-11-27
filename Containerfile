FROM golang:1.25-bookworm AS source
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY format.go format_test.go ./
COPY cmd/ cmd/
COPY testdata/ testdata/

FROM source AS build
RUN go build -o /usr/local/bin/goheresy ./cmd/goheresy

FROM build AS verify
ENV GOHERESY_TEST_UPSTREAM=1 GOHERESY_TEST_CORPUS=1 GOTELEMETRY=off
RUN go test -v ./... && go vet ./...
RUN go test -run '^$' -fuzz '^FuzzFormat$' -fuzztime 10s -parallel 2

FROM source AS build-binary
ARG GOOS
ARG GOARCH
RUN CGO_ENABLED=0 go build -o /out/goheresy ./cmd/goheresy

FROM scratch AS binary
COPY --from=build-binary /out/goheresy /goheresy

FROM golang:1.25-bookworm
COPY --from=build /usr/local/bin/goheresy /usr/local/bin/goheresy
WORKDIR /work
ENTRYPOINT ["goheresy"]

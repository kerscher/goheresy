.PHONY: build build-container install fmt fmt-check test vet fuzz image verify

OUTPUT_DIR ?= .

build:
	go build ./cmd/goheresy

build-container:
	docker build -f Containerfile --target binary \
		--build-arg GOOS="$$(uname -s | tr '[:upper:]' '[:lower:]')" \
		--build-arg GOARCH="$$(uname -m | sed -e 's/^x86_64$$/amd64/' -e 's/^aarch64$$/arm64/')" \
		--output "type=local,dest=$(OUTPUT_DIR)" .

install:
	go install ./cmd/goheresy

fmt:
	go run ./cmd/goheresy -w .

fmt-check:
	@changed=$$(go run ./cmd/goheresy -l .) || exit $$?; \
	if [ -n "$$changed" ]; then printf '%s\n' "$$changed"; exit 1; fi

test:
	go test ./...

vet:
	go vet ./...

fuzz:
	go test -run '^$$' -fuzz '^FuzzFormat$$' -fuzztime 10s -parallel 2

image:
	docker build -f Containerfile -t goheresy .

verify:
	docker build -f Containerfile --target verify .

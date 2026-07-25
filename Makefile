.PHONY: fmt test vet build verify

fmt:
	gofmt -w .

test:
	go test -race ./...

vet:
	go vet ./...

build:
	go build -o gh-tidy-branches ./cmd/gh-tidy-branches

verify: fmt test vet build

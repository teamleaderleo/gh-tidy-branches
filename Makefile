BINARY := gh-tidy-branches
PACKAGE := ./cmd/gh-tidy-branches
VERSION ?= 0.1.0-dev

.PHONY: fmt test vet build smoke verify install-dev uninstall-dev clean

fmt:
	gofmt -w .

test:
	go test -race ./...

vet:
	go vet ./...

build:
	go build -trimpath -ldflags='-X=main.version=$(VERSION)' -o $(BINARY) $(PACKAGE)

smoke: build
	@test "$$(./$(BINARY) --version)" = "$(VERSION)"
	@help="$$(./$(BINARY) --help)"; \
	printf '%s\n' "$$help" | grep -F -- '--preview' >/dev/null; \
	printf '%s\n' "$$help" | grep -F -- 'gh tidy-branches undo' >/dev/null; \
	printf '%s\n' "$$help" | grep -F -- 'gh tidy-branches doctor [owner/repo ...]' >/dev/null

verify: fmt test vet smoke
	git diff --exit-code

install-dev: build
	@gh extension remove tidy-branches >/dev/null 2>&1 || true
	gh extension install .
	@test "$$(gh tidy-branches --version)" = "$(VERSION)"
	@help="$$(gh tidy-branches --help)"; \
	printf '%s\n' "$$help" | grep -F -- '--preview' >/dev/null; \
	printf '%s\n' "$$help" | grep -F -- 'gh tidy-branches undo' >/dev/null
	@echo "Installed local extension: gh tidy-branches ($(VERSION))"

uninstall-dev:
	gh extension remove tidy-branches

clean:
	rm -f $(BINARY)
	rm -rf dist

.PHONY: all build test check clean release snapshot

all: build

# Local build only. Releases are produced by GoReleaser in CI; see RELEASING.md.
build:
	go build -o bin/tada ./cmd/tada

test:
	go test ./...

# Mirrors the gates the CI workflow runs on every pull request.
check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	go vet ./...
	go test -race ./...

clean:
	rm -rf bin dist

# Dry run of the release pipeline: builds every target into dist/ without publishing.
snapshot:
	goreleaser release --snapshot --clean

# Offline fallback for cross-compiled builds. Does not publish anything.
release:
	@mkdir -p bin
	GOOS=darwin GOARCH=arm64 go build -o bin/tada-darwin-arm64 ./cmd/tada
	GOOS=darwin GOARCH=amd64 go build -o bin/tada-darwin-amd64 ./cmd/tada
	GOOS=linux GOARCH=amd64 go build -o bin/tada-linux-amd64 ./cmd/tada
	GOOS=linux GOARCH=arm64 go build -o bin/tada-linux-arm64 ./cmd/tada
	GOOS=windows GOARCH=amd64 go build -o bin/tada-windows-amd64.exe ./cmd/tada

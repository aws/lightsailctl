.PHONY: test snapshot tools

test:
	go test ./...

snapshot:
	goreleaser release --snapshot --clean --skip=publish

tools:
	go install github.com/goreleaser/goreleaser/v2@latest


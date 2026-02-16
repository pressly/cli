.PHONY: build test lint format ci-test

build:
	go build -v .

test:
	go test $$(go list ./... | grep -v 'examples') -count=1 -v

lint:
	golangci-lint run ./...

format:
	goimports -w $$(find . -name '*.go' -not -path './examples/*')

ci-test:
	go test $$(go list ./... | grep -v 'examples') -count=1 -v -json -cover \
		| tparse -all -follow -sort=elapsed -trimpath=auto

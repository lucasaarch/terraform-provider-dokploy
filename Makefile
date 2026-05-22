default: build

build:
	go build -o terraform-provider-dokploy

test:
	go test ./internal/client/... -v

testacc:
	TF_ACC=1 go test ./internal/provider/... -v -timeout 30m

lint:
	golangci-lint run

fmt:
	gofmt -w .

docs:
	go generate ./...

.PHONY: default build test testacc lint fmt docs

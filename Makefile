deps:
	go install github.com/golang/mock/mockgen@latest

gen:
	@go generate ./...

clean-mocks:
	find . -name "mock_*" -type f -exec rm -rf {} +

mocks: clean-mocks gen

build: deps mocks
	go build -o bin/local-storage-sync cmd/local-storage-sync/main.go

multiarch-docker:
	 docker buildx build --platform linux/arm64/v8,linux/amd64 --tag  ghcr.io/paulosotu/storage-sync:latest --push .

run:
	./bin/local-storage-sync

update-go-deps:
	@echo ">> updating ALL DIRECT Go dependencies"
	@for m in $$(go list -mod=readonly -m -f '{{ if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all); do \
		go get $$m; \
	done
	go mod tidy
ifneq (,$(wildcard vendor))
	go mod vendor
endif
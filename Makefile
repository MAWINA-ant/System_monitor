BIN := "./bin/server"

build-server:
	go build -o bin/server ./cmd/server

build-client:
	go build -o bin/client ./cmd/client

build: build-server build-client

version: build
	$(BIN) version

# needed install protobuf `apt install protobuf-compiler protoc-gen-go protoc-gen-go-grpc`
install-proto-deps:
	@command -v protoc >/dev/null 2>&1 || sudo apt install -y protobuf-compiler
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

generate: install-proto-deps
	rm -rf api/statspb
	mkdir -p api/statspb
	protoc --proto_path=api \
		--go_out=api/statspb --go_opt=paths=source_relative \
		--go-grpc_out=api/statspb --go-grpc_opt=paths=source_relative \
		api/system_monitor.proto

test:
	go test -race -count 100 ./...

install-lint-deps:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

lint: install-lint-deps
	golangci-lint run ./...

.PHONY: build version test lint
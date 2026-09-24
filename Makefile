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
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

generate: install-proto-deps
	rm -rf internal/server/internalgrpc/eventpb
	mkdir -p internal/server/internalgrpc/eventpb

	protoc --proto_path=api/ --go_out=internal/server/internalgrpc/eventpb --go-grpc_out=internal/server/internalgrpc/eventpb api/*.proto

test:
	go test -race -count 100 ./...

install-lint-deps:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

lint: install-lint-deps
	golangci-lint run ./...

.PHONY: build version test lint
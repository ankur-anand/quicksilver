protofiles = v1/*/*.proto

gen: gen-dir gen-grpc

gen-dir:
	@mkdir -p gen

gen-grpc:
	@for proto in $(protofiles) ; do \
		protoc $$proto -I=. \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=gen --grpc-gateway_opt=paths=source_relative; \
	  done

deps:
	go get -u google.golang.org/protobuf/cmd/protoc-gen-go
	go get -u google.golang.org/grpc/cmd/protoc-gen-go-grpc
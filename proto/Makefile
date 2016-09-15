all: protoc_internal_rpc

protoc_internal_rpc: gateway.proto
	protoc --go_out=plugins=grpc:. gateway.proto
all: rpc_all

GOFILES = \
	gateway.pb.go \
	job/job.pb.go \
	smc/smc.pb.go

%.pb.go: %.proto
	protoc --go_out=plugins=grpc:. -I .:$(GOPATH)/src $*.proto

rpc_all: $(GOFILES)

clean:
	rm ${GOFILES}
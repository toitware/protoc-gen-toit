PROTO_FLAGS ?=

protobuf:
	protoc hello.proto --toit_out=. --toit_opt=constructor_initializers=1 $(PROTO_FLAGS)

clean:
	rm *_pb.toit

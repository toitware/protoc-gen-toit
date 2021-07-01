PROTO_FLAGS ?=

protobuf:
	protoc time.proto --toit_out=. --toit_opt=constructor_initializers=1 $(PROTO_FLAGS)

clean:
	rm -f *_pb.toit

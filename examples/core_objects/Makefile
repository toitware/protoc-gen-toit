PROTO_FLAGS ?=

.PHONY: protoc-gen-toit
protoc-gen-toit:
	make -C ../../ protoc-gen-toit

protobuf: protoc-gen-toit
	protoc --plugin=protoc-gen-toit=../../protoc-gen-toit --plugin=protoc-gen-toit=../../protoc-gen-toit time.proto --toit_out=. --toit_opt=constructor_initializers=1 $(PROTO_FLAGS)

clean:
	rm -f *_pb.toit

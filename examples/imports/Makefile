PROTO_DIR := ./pkg
PROTO_SOURCES := $(shell find . -name '*.proto')
PROTO_TARGETS := $(PROTO_SOURCES:$(PROTO_DIR)/%.proto=toit/%_pb.toit)

targets:
	# $(PROTO_TARGETS)

toit:
	mkdir toit

toit/%_pb.toit: $(PROTO_DIR)/%.proto toit
	protoc $< --toit_out=toit --toit_opt=import_library=pkg=.

protobuf: $(PROTO_TARGETS)

clean:
	rm -rf toit

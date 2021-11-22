GO_SOURCES := $(shell find . -name '*.go')
protoc-gen-toit: $(GO_SOURCES)
	go build  -o protoc-gen-toit .

build: protoc-gen-toit

clean:
	rm -rf protoc-gen-toit
	$(MAKE) -C ./examples/core_objects clean
	$(MAKE) -C ./examples/helloworld clean
	$(MAKE) -C ./examples/imports clean
	$(MAKE) -C ./examples/nesting clean
	$(MAKE) -C ./examples/oneofs clean

test:
	go test ./... -bench=. -benchmem -cover -count=1 -v

gen_examples: protoc-gen-toit
	$(MAKE) -C ./examples/core_objects protobuf
	$(MAKE) -C ./examples/helloworld protobuf
	$(MAKE) -C ./examples/imports protobuf
	$(MAKE) -C ./examples/nesting protobuf
	$(MAKE) -C ./examples/oneofs protobuf

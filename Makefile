GO_SOURCES := $(shell find . -name '*.go')
protoc-gen-toit: $(GO_SOURCES)
	go build  -o protoc-gen-toit .

build: protoc-gen-toit

clean:
	rm -rf protoc-gen-toit
	$(MAKE) -C ./examples/core_objects clean
	$(MAKE) -C ./examples/helloworld clean
	$(MAKE) -C ./examples/imports clean

test:
	go test ./... -bench=. -benchmem -cover -count=1 -v

gen_examples:
	$(MAKE) -C ./examples/core_objects protobuf
	$(MAKE) -C ./examples/helloworld protobuf
	$(MAKE) -C ./examples/imports protobuf

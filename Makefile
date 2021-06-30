GO_SOURCES := $(shell find . -name '*.go')
protoc-gen-toit: $(GO_SOURCES)
	go build  -o protoc-gen-toit .

build: protoc-gen-toit

clean:
	rm -rf protoc-gen-toit

test:
	go test ./... -bench=. -benchmem -cover -count=1 -v

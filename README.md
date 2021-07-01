# protoc-gen-toit
The `protoc-gen-toit` is a compiler plugin to protoc. It augments the protoc compiler so it knows how to generate Toit specific code for a given .proto file.

## Install

Download the protocol compiler plugin for Toit:

```
$ go install github.com/toitware/protoc-gen-toit
```

Update your `PATH` so that the `protoc` compiler can find the plugin:

```
$ export PATH="$PATH:$(go env GOPATH)/bin"
```

## Generate

In order to generate toit files use:

```
$ protoc <proto-file> --toit_out=.
```

## Options

The compiler plugin has some options that can be enabled using the `--toit_opt` flag to `protoc`:

### `constructor_initializers` (default 0)

if set to `1` each generated class constructor will have flags to initialize the object fields.

see `examples/helloworld`.

### `import_library`

Can be used to change the import paths. The setting is a set and can be set multiple times. Using `import_library=<from>=<to>` will take all proto imports prefixed with `<from>` and replace that prefix with `<to>` in the toit code.

see `examples/imports`.

### `core_objects` (default 1)

If set to `1` the built-in protobuf objects such as Timestamp, Duration etc. will be mapped directly to their counterparts in toit.

see `examples/core_objects`.

## Development
To have automatic checks for copyright and MIT notices, run

```
$ git config core.hooksPath .githooks
```

If a file doesn't need a copyright/MIT notice, use the following to skip
the check:
```
git commit --no-verify
```

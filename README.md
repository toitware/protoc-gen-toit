# protoc-gen-toit
The `protoc-gen-toit` is a compiler plugin to protoc. It augments the protoc compiler so it knows how to generate Toit specific code for a given .proto file.

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

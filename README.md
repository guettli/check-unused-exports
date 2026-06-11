# check-unused-exports

Tiny Go tool that reports exported identifiers (constants, variables, types, functions, methods)
that are never referenced outside test files.

## Usage

```console
go run github.com/guettli/check-unused-exports@latest [root-dir]
```

Exits `0` when nothing is found, `1` when unused exports are detected.

## Limitation

Uses name-based matching. Two identifiers with the same name (e.g. a method and a function both
called `New`) are treated as one; a reference to either marks both as used.

## Example

```console
$ go run github.com/guettli/check-unused-exports@latest ./myproject
myproject/foo.go:12:1: exported function Foo is not used
myproject/bar.go:8:1: exported type Bar is not used (used in tests)
```

## Install

```console
go install github.com/guettli/check-unused-exports@latest
```

## Related

- [guettli/check-conditions](https://github.com/guettli/check-conditions) — check all Kubernetes
  conditions

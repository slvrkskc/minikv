# minikv

![tests](https://github.com/slvrkskc/minikv/actions/workflows/test.yml/badge.svg)

A small, Redis-like key-value database written from scratch in Go — built to learn how real databases stay fast and survive crashes.

## Roadmap

- [x] Thread-safe in-memory store
- [x] Command-line interface (SET / GET / DEL)
- [x] Write-ahead log — length-prefixed binary records, CRC32, fsync policies
- [ ] Crash recovery — replay the log into the store on startup
- [ ] Log compaction
- [ ] TCP server
- [ ] Benchmarks vs. SQLite

## Run

```bash
go run ./cmd/minikv
```

```
> SET name alara
OK
> GET name
alara
```

## Test

```bash
go test -race ./...
```
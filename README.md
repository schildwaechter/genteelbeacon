# Genteel Beacon

## Building

Provide build timestamp

```shell
go build -ldflags "-X main.buildEpoch=$(date '+%s')" .
```

To send traces to an OTEL endpoint, specify its address

```shell
export OTEL_TRACES_ENDPOINT="localhost:4318"
```

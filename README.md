# Genteel Beacon

## Building

Provide build timestamp

```shell
go build -ldflags "-X main.buildEpoch=$(date '+%s')" .
```

## Using

When the binary is running, simply call

```shell
curl http://localhost:1333
curl http://localhost:1333/telegram
```

## Configuration

There are options to send traces to an OpenTelemetry Endpoint, log in JSON and more, based on these environment variables.

* `USENAME` -- The name the application identifies as
* `OTEL_TRACES_ENDPOINT` -- OTLP/HTTP-Endpoint to send traces to (no `http://`-prefix!)
* `JSONLOGGING` -- If set, will cause the logs to be emitted in JSON to `stdout`
* `BACKEND` -- The URL of another Genteel Beacon to query
* `RUNPORT` -- The port to serve on, defaults to `1333` if unset

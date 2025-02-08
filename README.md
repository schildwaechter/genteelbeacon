# Genteel Beacon

## Building

The repository is set up for [Air](https://github.com/air-verse/air) and [Docker](https://hub.docker.com/r/schildwaechter/genteelbeacon) build.

When building manually, provide the build timestamp

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
* `OTLPHTTP_ENDPOINT` -- OTLP/HTTP-Endpoint to send metrics, traces & logs to (no `http://`-prefix!)
* `JSONLOGGING` -- If set, will cause the logs to be emitted in JSON to `stdout`
* `BACKEND` -- The URL of another Genteel Beacon to query
* `APP_PORT` -- The port to serve on, defaults to `1333` if unset
* `APP_ADDR` -- The address to listen on, defaults to `0.0.0.0` if unset

# Genteel Beacon

![image](assets/banner.png)

The Genteel Beacon is an application specifically designed for playing with observability in steam-powered microservices.

## Building

The repository is set up for [Air](https://github.com/air-verse/air), `go install tool`.

When building manually, provide the build timestamp

```shell
go build -ldflags "-X main.buildEpoch=$(date '+%s')" .
```

Note that the Gearsmith will not work when not running inside a Kubernetes pod via the [Docker](https://hub.docker.com/r/schildwaechter/genteelbeacon) image.

## Using

When the binary is running, simply call (depending on role)

```shell
curl http://localhost:1333
curl http://localhost:1333/timestamp
curl http://localhost:1333/telegram
```

The following endpoints are for more simple testing requirements, with a hello world, request echo and server error.

```shell
curl http://localhost:1333/nod
curl http://localhost:1333/curtsey
curl http://localhost:1333/calamity
```

## Configuration

There are options to send traces to an OpenTelemetry Endpoint, log in JSON and more, based on these environment variables.

* `APP_PORT` -- The port to serve on, defaults to `1333` if unset
* `APP_ADDR` -- The address to listen on, defaults to `0.0.0.0` if unset
* `GENTEEL_NAME` -- The name the application identifies as
* `GENTEEL_ROLE` -- The role to assume, possible values are `telegraphist`, `clock`, `gearsmith` and `debutante`
* `GENTEEL_CLOCK` -- The address of the clock instance
* `OTLPHTTP_ENDPOINT` -- OTLP/HTTP-Endpoint to send metrics, traces & logs to (no `http://`-prefix!)
* `JSONLOGGING` -- If set, will cause the logs to be emitted in JSON to `stdout`
* `BACKEND` -- The URL of another Genteel Beacon to query (in 40% of cases)

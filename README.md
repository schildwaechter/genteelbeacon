# Genteel Beacon

![image](assets/banner.png)

The Genteel Beacon is an application specifically designed for playing with observability in steam-powered microservices.

## Building

The repository is set up for [Air](https://github.com/air-verse/air), `go install tool`.

When building manually, provide the version or build timestamp

```shell
go build -ldflags "-X main.buildVersion=$(cat VERSION)" .
```

or

```shell
go build -ldflags "-X main.buildVersion=$(date '+%s')" .
```

## Using

When the binary is running, it offers different functionality depending on the set role.

There will always be an answer on

```shell
curl http://localhost:1333
```

### Telegraphist

To retrieve the telegram as `html`, `json` or plain text, call with Accept-header

```shell
curl http://localhost:1333/telegram
```

### Clock

To retrieve the timestamp in `json`, call

```shell
curl http://localhost:1333/timestamp
```

### Lightkeeper

This will provide a simple identification, unless Accept is `json`.
Then retrieve the echo of the request-headers, together with some execution environment data and all environment variables starting with `GENTEEL_`.

```shell
curl http://localhost:1333/emission
```

It's also possible to directly trigger and error.

```shell
curl http://localhost:1333/calamity
```

### Agitator

In this role the application's healthcheck will become unreliable and the app will crash if requested

```shell
curl http://localhost:1333/calamity
```

### Gearsmith

The Gearsmith provides custom metrics to Kubernetes.
This will **not** work, unless running inside a Kubernetes pod via the [Docker](https://hub.docker.com/r/schildwaechter/genteelbeacon) image.

## Configuration

There are options to send traces to an OpenTelemetry Endpoint, log in JSON and more, based on these environment variables.

* `APP_PORT` -- The port to serve on, defaults to `1333` if unset
* `APP_ADDR` -- The address to listen on, defaults to `0.0.0.0` if unset
* `INT_PORT` -- The port to serve metrics and healthchecks on, defaults to `1337` if unset
* `INT_ADDR` -- The address to listen on for metrics and healthchecks, defaults to `127.0.0.0` if unset
* `GENTEEL_NAME` -- The name the application identifies as
* `GENTEEL_ROLE` -- The role to assume, possible values are `telegraphist`, `clock`, `gearsmith`, `lightkeeper` and `agitator`
* `GENTEEL_CLOCK` -- The address of the clock instance
* `OTLPHTTP_ENDPOINT` -- OTLP/HTTP-Endpoint to send metrics, traces & logs to (no `http://`-prefix!)
* `OTLPHTTP_TRACES_ENDPOINT` -- OTLP/HTTP-Endpoint to send traces to (no `http://`-prefix!) -- overrides full sending!
* `JSONLOGGING` -- If set, will cause the logs to be emitted in JSON to `stdout`

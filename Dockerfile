# build the binaries
FROM --platform=$BUILDPLATFORM golang:latest AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/genteelbeacon
COPY *.go /go/src/genteelbeacon/
COPY go.* /go/src/genteelbeacon/
COPY assets /go/src/genteelbeacon/assets
COPY views /go/src/genteelbeacon/views
RUN go get
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-X main.buildEpoch=$(date '+%s')" -o genteelbeacon .

# package the binary into a container
FROM scratch
COPY --from=builder /go/src/genteelbeacon/genteelbeacon /genteelbeacon
COPY --from=builder /go/src/genteelbeacon/assets /assets
COPY --from=builder /go/src/genteelbeacon/views /views

ENTRYPOINT ["/genteelbeacon"]

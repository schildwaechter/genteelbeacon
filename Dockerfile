# build the binaries
FROM --platform=$BUILDPLATFORM golang:latest AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/genteelbeacon
COPY VERSION /go/src/genteelbeacon/
COPY *.go /go/src/genteelbeacon/
COPY *.templ /go/src/genteelbeacon/
COPY go.* /go/src/genteelbeacon/
COPY assets/background.png /go/src/genteelbeacon/assets/background.png
RUN go get
RUN go tool templ generate
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-X main.buildVersion=$(cat VERSION)" -o genteelbeacon .

# package the binary into a container
FROM scratch
COPY --from=builder /go/src/genteelbeacon/genteelbeacon /genteelbeacon
COPY --from=builder /go/src/genteelbeacon/assets /assets

ENTRYPOINT ["/genteelbeacon"]

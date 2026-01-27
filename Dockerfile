# build the binaries
FROM --platform=$BUILDPLATFORM golang:1.25.6 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/genteelbeacon
COPY VERSION /go/src/genteelbeacon/
COPY cmd /go/src/genteelbeacon/cmd
COPY internal /go/src/genteelbeacon/internal
COPY go.* /go/src/genteelbeacon/
RUN go mod download
RUN go tool templ generate ./internal/templates
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-X github.com/schildwaechter/genteelbeacon/internal/config.BuildVersion=$(cat VERSION)" -o genteelbeacon ./cmd/genteelbeacon

# package the binary into a container
FROM scratch
COPY --from=builder /go/src/genteelbeacon/genteelbeacon /genteelbeacon
COPY assets/background.png /assets/background.png

ENTRYPOINT ["/genteelbeacon"]

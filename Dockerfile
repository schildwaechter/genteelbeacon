# build the binary
FROM golang:latest AS builder
WORKDIR /go/src/genteelbeacon
COPY *.go /go/src/genteelbeacon/
COPY go.* /go/src/genteelbeacon/
RUN go get
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.buildEpoch=$(date '+%s')" -o genteelbeacon .

# package the binary into a container
FROM scratch
COPY --from=builder /go/src/genteelbeacon/genteelbeacon /genteelbeacon
CMD ["/genteelbeacon"]

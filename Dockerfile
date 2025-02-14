# build the binaries
FROM golang:latest AS builder
WORKDIR /go/src/genteelbeacon
COPY *.go /go/src/genteelbeacon/
COPY go.* /go/src/genteelbeacon/
COPY grumpygearsmith/*.go /go/src/genteelbeacon/grumpygearsmith/
RUN go get
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.buildEpoch=$(date '+%s')" -o genteelbeacon.bin .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -o grumpygearsmith.bin ./grumpygearsmith

# package the binary into a container
FROM scratch
COPY --from=builder /go/src/genteelbeacon/genteelbeacon.bin /genteelbeacon
COPY --from=builder /go/src/genteelbeacon/grumpygearsmith.bin /grumpygearsmith
ENTRYPOINT ["/genteelbeacon"]

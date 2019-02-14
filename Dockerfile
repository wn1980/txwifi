FROM arm32v6/golang:1.10.1-alpine3.7 AS builder

ENV GOPATH /go
WORKDIR /go/src

RUN mkdir -p /go/src/github.com/txn2/txwifi
COPY . /go/src/github.com/txn2/txwifi

RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o /go/bin/wifi /go/src/github.com/txn2/txwifi/main.go

FROM cjimti/iotwifi

WORKDIR /

COPY --from=builder /go/bin/wifi /wifi
ENTRYPOINT ["/wifi"]



FROM golang:1.9-alpine

ADD . /go/src/github.com/dcarbone/lruchal

RUN set -ex && \
apk --no-cache --no-progress update && \
apk --no-cache --no-progress upgrade && \
apk add --no-cache --no-progress bash bind-tools dumb-init git && \
go get golang.org/x/net/netutil && \
cd /go/src/github.com/dcarbone/lruchal/server && `go build` && \
cd /go/src/github.com/dcarbone/lruchal/client && `go build`
# Build collectd-vsphere in a separate container
FROM golang:1.11 AS builder

RUN go get github.com/FiloSottile/gvt

WORKDIR /go/src/github.com/travis-ci/collectd-vsphere

COPY . .
RUN make deps
ENV CGO_ENABLED 0
RUN make build



FROM ubuntu:xenial

RUN apt-get update -yqq && apt-get install -y ca-certificates && update-ca-certificates
RUN apt-get install -y debian-archive-keyring apt-transport-https curl

COPY librato-collectd-pin /etc/apt/preferences.d/librato-collectd
RUN echo "deb https://packagecloud.io/librato/librato-collectd/ubuntu/ xenial main" > /etc/apt/sources.list.d/librato_librato-collectd.list
RUN curl -L https://packagecloud.io/librato/librato-collectd/gpgkey 2>/dev/null | apt-key add -

RUN apt-get update -yqq && apt-get install -y collectd liboping0 snmp snmp-mibs-downloader

COPY --from=builder /go/bin/collectd-vsphere .
COPY docker-run.sh .
CMD ["./docker-run.sh"]

FROM golang:alpine3.21 AS builder

WORKDIR /xray

RUN apk update && \
    apk add --no-cache \
    git \
    make \
    gcc \
    g++ \
    libtool \
    shadow && \
    chmod -R 777 /xray

COPY . .

RUN /usr/local/go/bin/go build -o xray -trimpath -ldflags "-X github.com/xtls/xray-core/core.build= -s -w -buildid=" -v ./main

FROM alpine
WORKDIR /
RUN apk add --no-cache tzdata ca-certificates bash
COPY --from=builder /xray/xray /usr/local/bin/xray
COPY --from=builder /xray/server_config_example.json /etc/xray/config.json

CMD ["/bin/bash", "-c", "/usr/local/bin/xray -c /etc/xray/config.json"]

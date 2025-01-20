ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ENV CGO_ENABLED=0
RUN ./build.sh

FROM debian:bookworm

COPY --from=builder /usr/src/app/lcc.live /usr/local/bin/lcc.live

RUN apt-get update && apt-get install -y ca-certificates 
RUN update-ca-certificates

CMD ["lcc.live"]

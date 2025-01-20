ARG GO_VERSION=1
FROM golang:${GO_VERSION}-alpine as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# ENV CGO_ENABLED=0
# RUN ./build.sh
# RUN go build -ldflags "-s -w" -x -v -o lcc.live .
RUN go build  -v -o lcc.live .

FROM alpine:latest

COPY --from=builder /usr/src/app/lcc.live /usr/local/bin/lcc.live
RUN apk add ca-certificates
RUN update-ca-certificates
# RUN apt-get update && apt-get install -y ca-certificates 
# RUN update-ca-certificates

CMD ["lcc.live"]

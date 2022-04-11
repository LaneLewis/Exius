FROM golang:1.18-alpine as builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download
RUN apk update \
    && apk upgrade \
    && apk add build-base
COPY ./database ./database
COPY ./handles ./handles
COPY rcloneProxy.go ./rcloneProxy.go
RUN go build -o /rclone-proxy

FROM alpine:latest
RUN apk update \
    && apk upgrade \
    && apk add rclone
COPY initiate.sh /app/initiate.sh
COPY ["./rclone.conf","/root/.config/rclone/rclone.conf"]
COPY --from=builder /rclone-proxy /rclone-proxy
COPY /data /app/data
CMD /bin/sh /app/initiate.sh
EXPOSE 8080
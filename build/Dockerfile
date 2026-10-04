FROM golang:1.27.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

RUN adduser -D -H -u 10001 app

COPY --from=build /out/server /usr/local/bin/server
COPY configs/config.json /etc/system-monitor/config.json

USER app

EXPOSE 8080

ENTRYPOINT ["server"]
CMD ["-port", "8080", "-config", "/etc/system-monitor/config.json"]
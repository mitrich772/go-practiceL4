# syntax=docker/dockerfile:1
# Двух-стейджный билд: один образ собирает оба бинаря, далее формируется
# либо image для сервера (по умолчанию), либо для клиента (target=client).

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags='-s -w' -o /out/mygrep-server ./cmd/mygrep-server && \
    go build -trimpath -ldflags='-s -w' -o /out/mygrep        ./cmd/mygrep

FROM alpine:3.20 AS server
RUN adduser -D -u 10001 mygrep
COPY --from=build /out/mygrep-server /usr/local/bin/mygrep-server
USER mygrep
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mygrep-server"]
CMD ["-addr", ":8080"]

FROM alpine:3.20 AS client
COPY --from=build /out/mygrep /usr/local/bin/mygrep
ENTRYPOINT ["/usr/local/bin/mygrep"]

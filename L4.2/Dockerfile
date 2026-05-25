# syntax=docker/dockerfile:1
# Однострочный образ для mygrep-server.

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags='-s -w' -o /out/mygrep-server ./cmd/mygrep-server

FROM alpine:3.20
RUN adduser -D -u 10001 mygrep
COPY --from=build /out/mygrep-server /usr/local/bin/mygrep-server
USER mygrep
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mygrep-server"]
CMD ["-addr", ":8080"]

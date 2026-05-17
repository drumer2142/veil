# syntax=docker/dockerfile:1
FROM golang:1.22-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /veil .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /veil /usr/local/bin/veil

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/veil"]
CMD ["-addr", ":8080", "-data", "/data"]

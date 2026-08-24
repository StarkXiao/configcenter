FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/config-center ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
RUN mkdir /app/data && chown app:app /app/data
COPY --from=build /out/config-center /usr/local/bin/config-center
USER app
EXPOSE 8081
ENTRYPOINT ["config-center"]

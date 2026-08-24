# Official multi-architecture Go image with the complete toolchain.
FROM golang:1.24

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build ./...

CMD ["bash"]

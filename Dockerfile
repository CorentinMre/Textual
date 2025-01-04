FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o chat-server ./cmd/server

EXPOSE 8080

CMD ["./chat-server"]
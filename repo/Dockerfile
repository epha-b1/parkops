FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN go install github.com/swaggo/swag/cmd/swag@latest && /go/bin/swag init -g cmd/server/main.go
RUN CGO_ENABLED=0 go build -mod=mod -o /app/parkops ./cmd/server

EXPOSE 8080

ENTRYPOINT ["/app/parkops"]

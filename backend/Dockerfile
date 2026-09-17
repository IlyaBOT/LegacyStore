FROM golang:1.22-alpine

RUN apk add --no-cache ca-certificates postgresql-client p7zip

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/legacystore-backend ./cmd/legacystore-backend

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/legacystore-backend"]

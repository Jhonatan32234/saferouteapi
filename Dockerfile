# Dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copiar go.mod y go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/api

# Imagen final
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copiar el binario compilado
COPY --from=builder /app/main .
COPY --from=builder /app/.env .env

# Puerto que expone la aplicación
EXPOSE 8080

# Comando para ejecutar la aplicación
CMD ["./main"]
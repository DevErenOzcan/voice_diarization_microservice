# gateway/Dockerfile
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Bağımlılıkları kopyala ve indir
COPY go.mod go.sum ./
RUN go mod download

# Tüm kodları kopyala
COPY . .

# CGO'yu aktif ederek derle (SQLite için zorunlu)
RUN CGO_ENABLED=1 GOOS=linux go build -o gateway-app .

# Çalıştırma aşaması için daha hafif bir imaj
FROM debian:bookworm-slim

WORKDIR /app

# Veritabanı vb. dosyalar için gerekli olabilecek kütüphaneler
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/gateway-app .

# Gateway 8080 portunda çalışıyor
EXPOSE 8080

CMD ["./gateway-app"]
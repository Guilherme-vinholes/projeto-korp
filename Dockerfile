# ---- Stage 1: build ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copia apenas os arquivos de dependências primeiro (cache de layer)
COPY go.mod ./

# Baixa dependências e gera go.sum dentro do container
RUN go mod download

# Copia o restante do código e compila
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o server .

# ---- Stage 2: imagem final mínima ----
FROM alpine:3.20

WORKDIR /app

# Copia apenas o binário compilado (sem toolchain Go)
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]

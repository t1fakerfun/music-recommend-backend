FROM golang:1.21-alpine
RUN apt-get update && apt-get install -y python3 python3-pip && pip3 install mysql-connector-python --break-system-packages
RUN apk add --no-cache python3

WORKDIR /app

COPY . .

# 依存関係をダウンロードして整理
RUN go mod tidy

# Goのビルド
RUN go build -o main .

CMD ["./main"]
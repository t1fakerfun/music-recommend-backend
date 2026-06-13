FROM golang:1.21-alpine

# Alpineのパッケージマネージャーで python3 と pip の本体をインストール
RUN apk add --no-cache python3 py3-pip

# ⭕ 【最強の回避策】Python環境に「システム保護を完全無視しろ」という設定ファイルをあらかじめ叩き込む
RUN mkdir -p ~/.config/pip && echo -e "[global]\nbreak-system-packages = true" > ~/.config/pip/pip.conf

# ⭕ pip自体のアップデートはせず、ダイレクトにライブラリだけをインストールする
RUN python3 -m pip install mysql-connector-python google-generativeai

WORKDIR /app

COPY . .

# 依存関係をダウンロードして整理
RUN go mod tidy

# Goのビルド
RUN go build -o main .

CMD ["./main"]

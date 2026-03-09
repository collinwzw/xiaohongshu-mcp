# ---- build stage ----
FROM golang:1.24 AS builder

WORKDIR /src
# Go module proxy — use default (proxy.golang.org) for non-China builds
# Set GOPROXY=https://goproxy.cn,direct for builds inside China
ENV GOPROXY=https://proxy.golang.org,direct

COPY go.mod go.sum ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/app .

# ---- run stage ----
FROM ubuntu:22.04

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

WORKDIR /app

# 1. Install base tools (use default Ubuntu mirrors — aliyun mirrors have stale packages)
RUN apt-get update && apt-get install -y ca-certificates wget gnupg

# 2. 添加 Google Chrome APT 源并安装 Chrome（更稳定的无头浏览器）
RUN wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor -o /usr/share/keyrings/googlechrome-linux-keyring.gpg && \
    echo "deb [arch=amd64 signed-by=/usr/share/keyrings/googlechrome-linux-keyring.gpg] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list

# 3. 安装 Google Chrome + 依赖（无头模式运行 rod）
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    fonts-liberation \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libc6 \
    libcairo2 \
    libcups2 \
    libdbus-1-3 \
    libexpat1 \
    libfontconfig1 \
    libgbm1 \
    libgcc1 \
    libglib2.0-0 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libstdc++6 \
    libx11-6 \
    libx11-xcb1 \
    libxcb1 \
    libxcomposite1 \
    libxcursor1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxi6 \
    libxrandr2 \
    libxrender1 \
    libxss1 \
    libxtst6 \
    lsb-release \
    wget \
    xdg-utils \
    google-chrome-stable \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/app .

# 4. 创建共享目录并设置权限
RUN mkdir -p /app/images && \
    chmod 777 /app/images

# 5. 设置默认 Chrome 路径（rod 会用）
ENV ROD_BROWSER_BIN=/usr/bin/google-chrome

EXPOSE 18060

CMD ["./app"]


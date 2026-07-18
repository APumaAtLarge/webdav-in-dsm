
# 阶段 1: 编译阶段
FROM debian:bullseye-slim AS builder

# 安装构建依赖
RUN apt-get update && apt-get install -y \
    build-essential \
    libpcre3 \
    libpcre3-dev \
    zlib1g \
    zlib1g-dev \
    libssl-dev \
    wget \
    git \
    && rm -rf /var/lib/apt/lists/*

# 设置 Nginx 版本和目录
ARG NGINX_VERSION=1.30.3
WORKDIR /src

# 下载 Nginx 源码
RUN wget http://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz \
    && tar -zxvf nginx-${NGINX_VERSION}.tar.gz

# 下载 Fancyindex 模块
RUN git clone https://github.com/aperezdc/ngx-fancyindex.git

# 编译 Nginx
RUN cd nginx-${NGINX_VERSION} \
    && ./configure \
        --with-compat \
        --add-module=../ngx-fancyindex \
        --with-http_dav_module \
        --prefix=/etc/nginx \
        --sbin-path=/usr/sbin/nginx \
        --modules-path=/usr/lib/nginx/modules \
        --conf-path=/etc/nginx/nginx.conf \
        --error-log-path=/var/log/nginx/error.log \
        --http-log-path=/var/log/nginx/access.log \
        --pid-path=/var/run/nginx.pid \
        --lock-path=/var/run/nginx.lock \
    && make \
    && make install

# 阶段 2: 编译 Go 接口
FROM golang:1.22-bullseye AS go-builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/link-api ./cmd/link-api

# 阶段 3: 运行阶段
FROM debian:bullseye-slim

# 安装运行所需的库
RUN apt-get update && apt-get install -y \
    libpcre3 \
    zlib1g \
    libssl1.1 \
    && rm -rf /var/lib/apt/lists/*

# 创建 GID=1111 的组和 UID=1111 的用户
RUN useradd \
        --uid 1111 \
        --gid 100 \
        --home-dir /var/www \
        --shell /usr/sbin/nologin \
        --no-create-home \
        davuser && id davuser

# 从构建阶段复制编译好的 Nginx 二进制文件
COPY --from=builder /usr/sbin/nginx /usr/sbin/nginx
COPY --from=builder /etc/nginx /etc/nginx
COPY --from=go-builder /out/link-api /usr/local/bin/link-api
COPY nginx.conf /etc/nginx/nginx.conf
COPY .htpasswd /etc/nginx/.htpasswd
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh


# 创建必要的目录和权限
RUN  mkdir -p /var/www/data /var/log/nginx /var/run \
    && chown -R 1111:100 /var/www/data /var/log/nginx \
    && chown 1111:100 /etc/nginx/.htpasswd \
    && chmod 640 /etc/nginx/.htpasswd \
    && chmod +x /usr/local/bin/docker-entrypoint.sh

# 暴露端口
EXPOSE 80

# 启动 Go 接口和 Nginx
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/bin/sh", "-c", "ADDR=127.0.0.1:8080 /usr/local/bin/link-api & exec nginx -g 'daemon off;'"]

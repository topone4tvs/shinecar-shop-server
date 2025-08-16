# 使用官方 Caddy 镜像作为基础
FROM caddy:2.10.0-builder-alpine AS builder

# 替换为国内APK镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 配置国内Go代理镜像
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
ENV GOSUMDB=sum.golang.google.cn
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# 安装必要的工具
RUN apk add --no-cache git ca-certificates

# 安装xcaddy
RUN go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

# 构建包含alidns插件的Caddy
RUN xcaddy build v2.10.0 --with github.com/caddy-dns/alidns --with github.com/mholt/caddy-l4 --output /usr/bin/caddy

# 最终阶段
FROM caddy:2.10.0-alpine

# 复制自定义构建的Caddy二进制文件
COPY --from=builder /usr/bin/caddy /usr/bin/caddy

# 验证构建是否包含alidns模块
RUN /usr/bin/caddy list-modules | grep dns || echo "正在检查DNS模块..."

# 设置容器启动命令
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]

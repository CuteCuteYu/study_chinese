# 构建阶段
FROM golang:1.23-alpine AS builder

# 设置多个国内镜像源
ENV GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,https://goproxy.io,direct
# 设置国内校验服务器
ENV GOSUMDB=sum.golang.google.cn

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod tidy

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o study_chinese

# 运行阶段
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/study_chinese .
COPY --from=builder /app/templates ./templates

EXPOSE 8080
CMD ["./study_chinese"]
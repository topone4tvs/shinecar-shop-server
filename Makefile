# 构建配置
BINARY_NAME=shop_server
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# 默认目标
.PHONY: help
help:
	@echo "可用的构建目标:"
	@echo "  build-linux    - 构建 Linux 版本"
	@echo "  build-mac      - 构建 macOS 版本"
	@echo "  build-windows  - 构建 Windows 版本"
	@echo "  clean          - 清理构建文件"
	@echo "  test           - 运行测试"

# 构建 Linux 版本（用于生产环境）
.PHONY: build-linux
build-linux:
	@echo "构建 Linux 版本..."
	@mkdir -p ${BUILD_DIR}
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ./cmd
	@echo "构建完成: ${BUILD_DIR}/${BINARY_NAME}"

# 构建 macOS 版本
.PHONY: build-mac
build-mac:
	@echo "构建 macOS 版本..."
	@mkdir -p ${BUILD_DIR}
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-mac ./cmd
	@echo "构建完成: ${BUILD_DIR}/${BINARY_NAME}-mac"

# 构建 Windows 版本
.PHONY: build-windows
build-windows:
	@echo "构建 Windows 版本..."
	@mkdir -p ${BUILD_DIR}
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}.exe ./cmd
	@echo "构建完成: ${BUILD_DIR}/${BINARY_NAME}.exe"

# 清理构建文件
.PHONY: clean
clean:
	@echo "清理构建文件..."
	@rm -rf ${BUILD_DIR}
	@echo "清理完成"

# 运行测试
.PHONY: test
test:
	@echo "运行测试..."
	go test ./...

# 部署目标
.PHONY: deploy-staging
deploy-staging:
	@echo "部署到新预发环境..."
	@chmod +x script/deploy-staging.sh
	@./script/deploy-staging.sh

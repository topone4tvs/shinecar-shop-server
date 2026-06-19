#!/bin/bash

# 简化部署脚本 - ShineCar Backend
# 一键部署到指定环境

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置
SSH_ALIAS="scsp001"
REMOTE_BASE_DIR="/var/www/projects/shop_server"
REMOTE_CONFIG_DIR="${REMOTE_BASE_DIR}/config"
DOCKER_COMPOSE_DIR="/var/www"
DOCKER_SERVICE="shop_server"
BINARY_NAME="shop_server"

# 函数：打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 函数：显示帮助信息
show_help() {
    echo "简化部署脚本"
    echo ""
    echo "用法: $0 [环境] [选项]"
    echo ""
    echo "环境:"
    echo "  dev      - 开发环境"
    echo "  staging  - 预发环境"
    echo "  online   - 生产环境"
    echo ""
    echo "选项:"
    echo "  --help, -h    显示此帮助信息"
    echo "  --test        测试模式，只构建和检查，不上传"
    echo ""
    echo "示例:"
    echo "  $0 dev         # 部署到开发环境"
    echo "  $0 staging     # 部署到预发环境"
    echo "  $0 online      # 部署到生产环境"
    echo "  $0 dev --test  # 测试模式"
}

# 函数：获取配置文件
get_config_file() {
    local env=$1
    case $env in
        dev)
            echo "config/env.dev.yaml"
            ;;
        staging)
            echo "config/env.staging.yaml"
            ;;
        online)
            echo "config/env.online.yaml"
            ;;
        *)
            print_error "未知环境: $env"
            exit 1
            ;;
    esac
}

# 函数：构建项目
build_project() {
    print_info "构建项目..."
    make build-linux
    print_success "构建完成"
}

# 函数：上传文件
upload_files() {
    local env=$1
    local config_file=$2
    
    print_info "上传文件到服务器..."
    
    # 检查本地文件是否存在
    if [[ ! -f "bin/${BINARY_NAME}" ]]; then
        print_error "本地二进制文件不存在: bin/${BINARY_NAME}"
        exit 1
    fi
    
    if [[ ! -f "$config_file" ]]; then
        print_error "本地配置文件不存在: $config_file"
        exit 1
    fi
    
    # 测试 SSH 连接
    print_info "测试 SSH 连接..."
    if ! ssh $SSH_ALIAS "echo 'SSH 连接成功'" > /dev/null 2>&1; then
        print_error "SSH 连接失败，请检查 SSH 配置"
        exit 1
    fi
    
    # 创建远程目录
    print_info "创建远程目录..."
    ssh $SSH_ALIAS "mkdir -p ${REMOTE_BASE_DIR} ${REMOTE_CONFIG_DIR}"
    
    # 检查目录权限
    print_info "检查目录权限..."
    ssh $SSH_ALIAS "ls -la ${REMOTE_BASE_DIR}"
    
    # 备份现有二进制文件
    print_info "备份现有二进制文件..."
    ssh $SSH_ALIAS "if [ -f ${REMOTE_BASE_DIR}/${BINARY_NAME} ]; then cp ${REMOTE_BASE_DIR}/${BINARY_NAME} ${REMOTE_BASE_DIR}/${BINARY_NAME}.backup.$(date +%Y%m%d_%H%M%S); fi"
    
    # 删除现有二进制文件（避免 scp 覆盖问题）
    print_info "删除现有二进制文件..."
    ssh $SSH_ALIAS "rm -f ${REMOTE_BASE_DIR}/${BINARY_NAME}"
    
    # 上传二进制文件
    print_info "上传二进制文件..."
    if ! scp "bin/${BINARY_NAME}" "${SSH_ALIAS}:${REMOTE_BASE_DIR}/"; then
        print_error "二进制文件上传失败"
        print_info "诊断信息："
        ssh $SSH_ALIAS "df -h ${REMOTE_BASE_DIR}"
        ssh $SSH_ALIAS "ls -la ${REMOTE_BASE_DIR}"
        print_info "尝试恢复备份文件..."
        ssh $SSH_ALIAS "if [ -f ${REMOTE_BASE_DIR}/${BINARY_NAME}.backup.* ]; then cp ${REMOTE_BASE_DIR}/${BINARY_NAME}.backup.* ${REMOTE_BASE_DIR}/${BINARY_NAME}; fi"
        print_info "重新启动服务..."
        ssh $SSH_ALIAS "cd ${DOCKER_COMPOSE_DIR} && docker compose up -d --force-recreate $DOCKER_SERVICE" || true
        exit 1
    fi
    
    # 备份现有配置文件
    print_info "备份现有配置文件..."
    ssh $SSH_ALIAS "if [ -f ${REMOTE_CONFIG_DIR}/env.yaml ]; then cp ${REMOTE_CONFIG_DIR}/env.yaml ${REMOTE_CONFIG_DIR}/env.yaml.backup.$(date +%Y%m%d_%H%M%S); fi"
    
    # 上传配置文件
    print_info "上传配置文件..."
    if ! scp "$config_file" "${SSH_ALIAS}:${REMOTE_CONFIG_DIR}/env.yaml"; then
        print_error "配置文件上传失败"
        print_info "尝试恢复备份文件..."
        ssh $SSH_ALIAS "if [ -f ${REMOTE_CONFIG_DIR}/env.yaml.backup.* ]; then cp ${REMOTE_CONFIG_DIR}/env.yaml.backup.* ${REMOTE_CONFIG_DIR}/env.yaml; fi"
        exit 1
    fi
    
    # 设置权限
    print_info "设置文件权限..."
    ssh $SSH_ALIAS "chmod +x ${REMOTE_BASE_DIR}/${BINARY_NAME}"
    
    # 清理旧备份文件（保留最近3个）
    print_info "清理旧备份文件..."
    ssh $SSH_ALIAS "ls -t ${REMOTE_BASE_DIR}/${BINARY_NAME}.backup.* 2>/dev/null | tail -n +4 | xargs -r rm"
    ssh $SSH_ALIAS "ls -t ${REMOTE_CONFIG_DIR}/env.yaml.backup.* 2>/dev/null | tail -n +4 | xargs -r rm"
    
    print_success "文件上传完成"
}

# 函数：启动服务
start_service() {
    print_info "启动 Docker 服务..."
    ssh $SSH_ALIAS "cd ${DOCKER_COMPOSE_DIR} && docker compose up -d --force-recreate $DOCKER_SERVICE"
    
    # 等待服务启动
    print_info "等待服务启动..."
    sleep 10
    
    # 检查服务状态
    print_info "检查服务状态..."
    ssh $SSH_ALIAS "cd ${DOCKER_COMPOSE_DIR} && docker compose ps $DOCKER_SERVICE"
    
    print_success "服务启动完成"
}

# 主函数
main() {
    local env=""
    local test_mode=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_help
                exit 0
                ;;
            --test)
                test_mode=true
                shift
                ;;
            -*)
                print_error "未知选项: $1"
                show_help
                exit 1
                ;;
            *)
                if [[ -z "$env" ]]; then
                    env=$1
                else
                    print_error "环境参数重复: $1"
                    exit 1
                fi
                shift
                ;;
        esac
    done
    
    # 检查参数
    if [[ -z "$env" ]]; then
        print_error "请指定部署环境"
        show_help
        exit 1
    fi
    
    # 验证环境
    case $env in
        dev|staging|online)
            ;;
        *)
            print_error "无效的环境: $env"
            show_help
            exit 1
            ;;
    esac
    
    # 获取配置文件
    local config_file=$(get_config_file "$env")
    
    # 检查配置文件
    if [[ ! -f "$config_file" ]]; then
        print_error "配置文件不存在: $config_file"
        exit 1
    fi
    
    print_info "开始部署到环境: $env"
    print_info "配置文件: $config_file"
    
    if [[ "$test_mode" == true ]]; then
        print_info "测试模式：只构建和检查，不上传"
    fi
    
    # 构建项目
    build_project
    
    # 上传文件
    if [[ "$test_mode" != true ]]; then
        upload_files "$env" "$config_file"
        
        # 启动服务
        start_service
    else
        print_success "测试模式完成！"
        print_info "构建成功，配置文件检查通过"
    fi
    
    print_success "部署完成！"
}

# 执行主函数
main "$@" 
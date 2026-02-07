#!/bin/bash

# Anywhere Agent 编译脚本

# 默认配置
OUTPUT_DIR="./dist"
APP_NAME="backend"
VERSION="v1.0.0"
ARCH="$(uname -m)"  # 默认使用当前运行架构
GOOS="$(uname -s | tr '[:upper:]' '[:lower:]')"  # 默认使用当前运行操作系统
# 新的目录结构
BASE_DIR="$OUTPUT_DIR/aw_backend"
BIN_DIR="$BASE_DIR/bin"
CONF_DIR="$BASE_DIR/conf"

# 解析命令行参数
while getopts "o:v:a:s:h" opt; do
  case $opt in
    o) OUTPUT_DIR="$OPTARG" ;;    v) VERSION="$OPTARG" ;;    a) ARCH="$OPTARG" ;;    s) GOOS="$OPTARG" ;;    h) echo "Usage: $0 [-o output_dir] [-v version] [-a arch] [-s os] [-h]"; exit 0 ;;    *) echo "Invalid option: -$OPTARG"; exit 1 ;;  esac
done

# 更新目录结构
BASE_DIR="$OUTPUT_DIR/aw_backend"
BIN_DIR="$BASE_DIR/bin"
CONF_DIR="$BASE_DIR/conf"

echo "=== Anywhere Backend Build Script ==="    
echo "Version: $VERSION"
echo "Architecture: $ARCH"
echo "Operating System: $GOOS"
echo "Output Directory: $BASE_DIR"
echo


# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    exit 1
fi

# 输出当前运行架构和操作系统
echo "Current system: $(uname -s) $(uname -m)"
echo "✓ Go environment checked"

# 清理旧的构建产物
echo "Cleaning old build artifacts..."
rm -rf "$OUTPUT_DIR"
# 创建新的目录结构
mkdir -p "$BIN_DIR"
mkdir -p "$CONF_DIR"

echo "✓ Cleanup completed"

# 编译Go代码
echo "Building Backend binary for $GOOS-$ARCH..."
# 转换架构名称：aarch64 是 arm64 的别名
BUILD_ARCH="$ARCH"
if [ "$BUILD_ARCH" = "aarch64" ]; then
    BUILD_ARCH="arm64"
fi
if [ "$BUILD_ARCH" = "x86_64" ]; then
    BUILD_ARCH="amd64"
fi
# 设置交叉编译环境变量
export CGO_ENABLED=0
export GOOS="$GOOS"
export GOARCH="$BUILD_ARCH"
# 执行交叉编译
go build -o "$BIN_DIR/$APP_NAME" ./cmd/api/main.go

if [ $? -ne 0 ]; then
    echo "Error: Build failed"
    exit 1
fi

echo "✓ Binary built successfully: $BIN_DIR/$APP_NAME"

# 复制配置文件示例
echo "Copying configuration files..."
cp -f conf/conf.yaml.example "$CONF_DIR/conf.yaml.example"

if [ -f conf/config.yaml ]; then
    cp -f conf/config.yaml "$CONF_DIR/conf.yaml"
else
    # 如果没有config.yaml，创建一个空的示例文件
    cp -f conf/config.yaml.example "$CONF_DIR/conf.yaml"
fi

echo "✓ Configuration files copied"

# 复制systemd服务文件
echo "Copying systemd service file..."
cp -f scripts/aw_backend.service "$BASE_DIR/"
echo "✓ Systemd service file copied"

# 复制安装脚本
echo "Copying installation script..."
cp -f scripts/install.sh "$BASE_DIR/"
chmod +x "$BASE_DIR/install.sh"
echo "✓ Installation script copied"

# 设置执行权限
chmod +x "$BIN_DIR/$APP_NAME"

echo "✓ Execution permissions set"

echo


echo "=== Build Summary ==="
echo "✓ Build completed successfully!"
echo "Binary: $BIN_DIR/$APP_NAME"
echo "Version: $VERSION"
echo "Architecture: $ARCH"
echo "Configuration: $CONF_DIR/conf.yaml"
echo "Systemd Service: $BASE_DIR/aw_backend.service"
echo
echo "To install the backend:"
echo "  sudo $BASE_DIR/install.sh"
echo
echo "To run the backend manually:"
echo "  $BIN_DIR/$APP_NAME -c $CONF_DIR/conf.yaml"

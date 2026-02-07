#!/bin/bash

# Anywhere Backend 安装脚本

echo "=== Anywhere Backend Install Script ==="

# 检查是否以root权限运行
if [ "$EUID" -ne 0 ]; then
  echo "Error: Please run this script as root"
  exit 1
fi

# 获取当前目录
INSTALL_DIR="$(pwd)"
echo "Installing from directory: $INSTALL_DIR"

# 检查服务文件是否存在
if [ ! -f "$INSTALL_DIR/aw_backend.service" ]; then
  echo "Error: aw_backend.service file not found in $INSTALL_DIR"
  exit 1
fi

# 检查二进制文件是否存在
if [ ! -f "$INSTALL_DIR/bin/backend" ]; then
  echo "Error: backend binary not found in $INSTALL_DIR/bin"
  exit 1
fi

# 创建日志目录
mkdir -p /var/log/aw_backend

# 创建安装目录
INSTALL_TARGET="/opt/aw_backend"
echo "Creating installation directory: $INSTALL_TARGET"
mkdir -p "$INSTALL_TARGET/bin"
mkdir -p "$INSTALL_TARGET/conf"

# 复制文件
echo "Copying files..."
cp -r "$INSTALL_DIR/bin" "$INSTALL_TARGET/"
cp -r "$INSTALL_DIR/conf" "$INSTALL_TARGET/"
cp -f "$INSTALL_DIR/aw_backend.service" "$INSTALL_TARGET/"

# 复制systemd服务文件
echo "Installing systemd service..."
cp -f "$INSTALL_DIR/aw_backend.service" "/etc/systemd/system/"

# 重载systemd配置
echo "Reloading systemd configuration..."
systemctl daemon-reload

# 启动服务
echo "Starting aw_backend service..."
systemctl start aw_backend

# 启用开机自启
echo "Enabling aw_backend service on boot..."
systemctl enable aw_backend

# 检查服务状态
echo "Checking service status..."
systemctl status aw_backend --no-pager
echo

echo "=== Installation Complete ==="
echo "Backend installed at: $INSTALL_TARGET"
echo "Service installed as: aw_backend.service"
echo "To check service status: systemctl status aw_backend"
echo "To stop service: systemctl stop aw_backend" 
echo "To view logs: journalctl -u aw_backend -f"  

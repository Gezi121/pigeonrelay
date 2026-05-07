#!/bin/bash

echo "=========================================================="
echo "    PigeonRelay Speedtest Client 一键部署脚本"
echo "=========================================================="
echo ""

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo "[错误] 未检测到 Docker，请先安装 Docker: curl -fsSL https://get.docker.com | bash"
    exit 1
fi

# 获取用户输入
# 如果脚本通过 curl | bash 运行，重定向输入
if [ -t 0 ]; then
    INPUT_DEV="/dev/stdin"
else
    INPUT_DEV="/dev/tty"
fi

read -p "请输入 PigeonRelay 面板地址 (如 http://1.2.3.4:3214): " PIGEONRELAY_URL < $INPUT_DEV
if [ -z "$PIGEONRELAY_URL" ]; then
    echo "[错误] 地址不能为空"
    exit 1
fi

read -p "请输入 Latency Token: " LATENCY_TOKEN < $INPUT_DEV
if [ -z "$LATENCY_TOKEN" ]; then
    echo "[错误] Token 不能为空"
    exit 1
fi

read -p "请输入客户端所在运营商 (如 移动/电信/联通，可留空): " CLIENT_ISP < $INPUT_DEV
read -p "请输入客户端所在地区 (如 华南/华东/香港，可留空): " CLIENT_REGION < $INPUT_DEV

echo ""
echo "正在停止并删除旧的测速容器 (如果存在)..."
docker stop speedtest >/dev/null 2>&1
docker rm speedtest >/dev/null 2>&1

echo "正在拉取最新镜像并启动容器..."
docker run -d --name speedtest --network host --restart unless-stopped \
  -e PIGEONRELAY_URL="${PIGEONRELAY_URL}" \
  -e LATENCY_TOKEN="${LATENCY_TOKEN}" \
  -e CLIENT_ISP="${CLIENT_ISP}" \
  -e CLIENT_REGION="${CLIENT_REGION}" \
  ghcr.io/gezi121/pigeonrelay-speedtest:latest

echo ""
if [ $? -eq 0 ]; then
    echo "=========================================================="
    echo "部署成功！"
    echo "你可以使用以下命令查看运行日志："
    echo "docker logs -f speedtest"
    echo "=========================================================="
else
    echo "部署失败，请检查上方报错信息。"
fi

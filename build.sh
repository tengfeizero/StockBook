#!/usr/bin/env bash
# 一键构建发布包：前端 → 嵌入 → 单文件后端
# 产物: release/huoben.exe （含全部前端页面，部署仅需此一个文件 + data 目录运行时生成）
set -e
cd "$(dirname "$0")"

echo "[1/3] 构建前端..."
(cd web && npm run build)

echo "[2/3] 嵌入前端产物..."
rm -rf server/internal/webembed/webdist
mkdir -p server/internal/webembed/webdist
cp -r web/dist/. server/internal/webembed/webdist/
touch server/internal/webembed/webdist/.gitkeep   # 保持 git 中的占位文件，避免源码 clone 后无法编译

echo "[3/3] 编译单文件后端..."
(cd server && go build -ldflags "-s -w" -o ../release/huoben.exe ./cmd)

echo
echo "构建完成: release/huoben.exe"
echo "部署: 把 huoben.exe 放到任意空目录运行即可，浏览器访问 http://localhost:8080"
echo "开机自启(Windows 管理员): huoben.exe -install  查看状态: huoben.exe -status"
echo "提示: 客户机器码获取: huoben.exe -machine-code ；授权签发: go run ./cmd/genlicense"

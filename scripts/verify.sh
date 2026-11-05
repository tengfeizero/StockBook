#!/usr/bin/env bash
# 提交前自检：一个命令判定"这份代码能不能提交 / 新机器能不能跑起来"
#
#   bash scripts/verify.sh              # 只检查（快），缺依赖会自动装
#   bash scripts/verify.sh --build      # 检查 + 完整构建单文件产物
#   bash scripts/verify.sh --no-install # 缺依赖直接报错，不自动安装（CI 用）
#
# 依次执行：工具链 → 依赖 → gofmt → go vet → go test → vue-tsc →（可选）build.sh
set -uo pipefail
cd "$(dirname "$0")/.."

DO_BUILD=0
DO_INSTALL=1
for arg in "$@"; do
  case "$arg" in
    --build) DO_BUILD=1 ;;
    --no-install) DO_INSTALL=0 ;;
    -h|--help) sed -n '2,9p' "$0"; exit 0 ;;
    *) echo "未知参数: $arg"; exit 2 ;;
  esac
done

FAILED=()
step() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok()   { printf '   \033[32mOK\033[0m  %s\n' "$*"; }
bad()  { printf '   \033[31mFAIL\033[0m %s\n' "$*"; FAILED+=("$*"); }
warn() { printf '   \033[33mWARN\033[0m %s\n' "$*"; }
indent() { sed 's/^/        /' | head -25; }

# ---------- 1. 工具链 ----------
step "1/6 工具链"
REQ_GO="$(awk '/^go /{print $2; exit}' server/go.mod)"
if command -v go >/dev/null 2>&1; then
  GOV="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
  ok "go $GOV  ($(command -v go))"
  if [ -n "${REQ_GO:-}" ] && [ "$(printf '%s\n%s\n' "$REQ_GO" "$GOV" | sort -V | head -1)" != "$REQ_GO" ]; then
    bad "go 版本过低：server/go.mod 要求 >= $REQ_GO，当前 $GOV"
  fi
else
  bad "找不到 go（需要 >= ${REQ_GO:-1.27}）"
fi

if command -v node >/dev/null 2>&1; then
  ok "node $(node -v)"
  [ "$(node -p 'process.versions.node.split(".")[0]')" -lt 20 ] && bad "node 版本过低：需要 >= 20"
else
  bad "找不到 node（需要 >= 20）；只做后端改动可忽略"
fi
command -v npm >/dev/null 2>&1 && ok "npm $(npm -v)" || bad "找不到 npm"

# ---------- 2. 依赖 ----------
step "2/6 依赖"
if (cd server && go mod download >/dev/null 2>&1); then
  ok "Go 模块就绪"
else
  bad "go mod download 失败（网络/代理？）"
fi

if [ -d web/node_modules ]; then
  ok "web/node_modules 已存在"
elif [ "$DO_INSTALL" = "1" ]; then
  echo "   web/node_modules 不存在，执行 npm ci ..."
  if (cd web && npm ci --no-fund --no-audit >/dev/null 2>&1); then ok "npm ci 完成"; else bad "npm ci 失败"; fi
else
  bad "web/node_modules 不存在（去掉 --no-install 可自动安装）"
fi

# ---------- 3. gofmt ----------
step "3/6 gofmt"
UNFMT="$(cd server && gofmt -l . 2>/dev/null)"
if [ -z "$UNFMT" ]; then
  ok "格式统一"
else
  bad "以下文件未格式化（cd server && gofmt -w .）："; echo "$UNFMT" | indent
  # 全都报未格式化 = 换行被转成了 CRLF（Windows 上 core.autocrlf=true 的典型症状），不是真的没格式化
  if [ "$(echo "$UNFMT" | wc -l)" -gt 20 ]; then
    warn "报错文件数异常多：大概率是检出时换行变成 CRLF。检查 .gitattributes 是否有 '*.go text eol=lf'，"
    warn "可执行：git config core.autocrlf false && git rm --cached -r . >/dev/null && git reset --hard"
  fi
fi

# ---------- 4. go vet ----------
step "4/6 go vet"
if VET_OUT="$(cd server && go vet ./... 2>&1)"; then
  ok "vet 干净"
else
  bad "go vet 有告警："; echo "$VET_OUT" | indent
fi

# ---------- 5. go test ----------
step "5/6 go test"
if TEST_OUT="$(cd server && go test ./... 2>&1)"; then
  ok "测试通过（$(echo "$TEST_OUT" | grep -c '^ok' || true) 个包有测试）"
else
  bad "测试未通过："
  echo "$TEST_OUT" | grep -E '^(FAIL|--- FAIL|\s+\S+_test\.go:)' | indent
fi

# ---------- 6. 前端类型检查 ----------
step "6/6 前端类型检查 (vue-tsc -b)"
if [ -d web/node_modules ]; then
  if TSC_OUT="$(cd web && npx --no-install vue-tsc -b 2>&1)"; then
    ok "类型检查通过"
  else
    bad "类型错误："; echo "$TSC_OUT" | indent
  fi
else
  warn "跳过（缺 web/node_modules）"
fi

# ---------- 可选：完整构建 ----------
if [ "$DO_BUILD" = "1" ]; then
  step "附加：完整构建 build.sh"
  if BUILD_OUT="$(bash build.sh 2>&1)"; then
    ok "构建成功：release/huoben.exe $(ls -lh release/huoben.exe 2>/dev/null | awk '{print $5}')"
  else
    bad "构建失败："; echo "$BUILD_OUT" | indent
  fi
fi

# ---------- 汇总 ----------
echo
if [ "${#FAILED[@]}" -eq 0 ]; then
  printf '\033[42;30m 自检通过 \033[0m 可以提交。别忘了同步 docs/CHANGELOG.md 与 docs/项目现状.md。\n'
  exit 0
else
  printf '\033[41;37m 自检失败 %s 项 \033[0m\n' "${#FAILED[@]}"
  for f in "${FAILED[@]}"; do echo "  - $f"; done
  echo
  echo "改完再跑一次：bash scripts/verify.sh"
  exit 1
fi

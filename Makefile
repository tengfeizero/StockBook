# 常用命令：make verify / make dev-server / make build
# 注意：Windows 默认没有 make，直接跑 scripts/verify.sh 与下面列出的等价命令即可。
.PHONY: verify setup dev-server dev-web build test lint clean

verify:
	bash scripts/verify.sh

# 装依赖 + 自检，新机器第一步
setup:
	cd server && go mod download
	cd web && npm ci
	bash scripts/verify.sh

dev-server:
	cd server && go run ./cmd

dev-web:
	cd web && npm run dev

# 生产构建：前端 → 内嵌 → 单文件二进制
build:
	./build.sh

test:
	cd server && go test ./...

lint:
	cd server && test -z "$$(gofmt -l .)" && go vet ./...
	cd web && npx vue-tsc -b

clean:
	rm -rf release web/dist
	git checkout -- server/internal/webembed/webdist 2>/dev/null || true

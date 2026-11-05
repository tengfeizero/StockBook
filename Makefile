# 常用命令：make dev / make build / make test
.PHONY: dev-server dev-web build test lint clean

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

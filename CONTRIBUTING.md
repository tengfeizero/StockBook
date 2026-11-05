# 贡献指南

感谢你愿意改进 JXC。这是一个**单文件部署的小微商户进销存系统**，代码以「业务规则集中在 service 层」为组织原则，改动前建议先读 [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) 与 [`docs/DATA-MODEL.md`](docs/DATA-MODEL.md)。

## 环境要求

| 组件 | 版本 |
|---|---|
| Go | 与 `server/go.mod` 中 `go` 指令一致（1.22+） |
| Node.js | 20 LTS 或更高 |
| 数据库 | 无需安装，SQLite 文件运行时自动创建 |

## 本地开发

```bash
# 后端：:8080，首次启动自动建库并创建 admin/admin123
cd server && go run ./cmd

# 前端：:5173，/api 代理到 8080，改动热更新
cd web && npm install && npm run dev
```

打开 <http://localhost:5173>，用 `admin / admin123` 登录。登录页「选个行业一键体验」可载入四套演示数据，不必手工建档。

## 提交前自检

```bash
cd server && gofmt -l . && go vet ./... && go test ./...
cd web && npx vue-tsc -b && npm run build
```

CI 会重复执行以上命令，本地先跑一遍能省一次往返。

## 代码约定（摘要）

- **业务规则只写在 `server/internal/service`**；`api` 层只做参数绑定与响应，`repo` 只做数据访问
- 金额一律 `int`（分），数量一律 `int64`（实际值 ×1000），禁止浮点参与金额计算
- 所有写操作在 service 层开事务；库存变动必须写流水（只追加，不修改）
- 新增接口需同步 `docs/API.md`；新增表/字段需同步 `docs/DATA-MODEL.md`
- 前端严格 TypeScript，不用 `any` 绕过类型检查；接口类型集中在 `web/src/api/*.ts`
- 提交信息用中文或英文均可，建议 `type(scope): 说明`，如 `fix(doc): 盘点差异被抹平`

## Pull Request

1. 从 `main` 切分支：`feat/xxx`、`fix/xxx`
2. 一个 PR 只做一件事；涉及库存/成本计算的改动**必须**附单元测试
3. PR 描述里写清：背景、改动点、验证方式（截图或命令输出）、是否影响数据迁移
4. 破坏性变更（表结构、接口签名）需在描述中显式标注，并说明升级步骤

## 不要提交

`data/`、`*.db`、`license.key`、`.jwt_secret`、`.env`、`release/`、`node_modules/` 均已在 `.gitignore` 中。授权私钥**永远不要**写进代码或提交历史——签发工具从环境变量 `JXC_LICENSE_KEY` 读取。

## 报告问题

- 功能缺陷请用 Bug report 模板，附版本号（`docs/CHANGELOG.md` 最新版本）、操作系统、复现步骤、日志（`data/logs/`）
- 安全漏洞**不要**开公开 issue，见 [SECURITY.md](SECURITY.md)

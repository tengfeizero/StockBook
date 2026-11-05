# AGENTS.md — 给接手这个仓库的 AI 编码代理

> 任何 AI 编码工具（Claude Code / Cursor / Codex / WorkBuddy / Copilot Workspace / 其他）进入本仓库，**先读完这一页**。
> 给人类看的门面是 [README.md](README.md)；协作方法论见 [docs/AI协作开发流程.md](docs/AI协作开发流程.md)。

## 这是什么

单店/单仓小微商户的进销存系统。**Go 1.27 + Gin + GORM + SQLite** 后端 + **Vue 3 + TS + Element Plus** 前端，
构建时前端经 `go:embed` 打进二进制，产物是单个 `huoben.exe`，数据在 exe 同目录 `data/`。
当前 v0.27，功能完整、可交付；同时支持多租户 SaaS 模式（0 号租户 = 单机版）。

## 开工前 5 分钟（按顺序，别跳）

1. **[docs/项目现状.md](docs/项目现状.md)** —— 做到哪一步、有哪些不能碰的约定、踩过哪些坑、下一步做什么。**这是唯一的"当前状态"来源**，比任何代码注释都准。
2. **[docs/TODO.md](docs/TODO.md)** 顶部 —— 最近几个版本的交付内容与遗留项（含"没验证到的部分"，那些是真实风险）。
3. **[docs/CONVENTIONS.md](docs/CONVENTIONS.md)** —— 分层职责、金额/数量单位、错误码段、以及各业务模块的硬约定（库位、盘点、秤码、容量）。
4. 改接口查 [docs/API.md](docs/API.md)，改表查 [docs/DATA-MODEL.md](docs/DATA-MODEL.md)。
5. 只改**一个**关注点，跑完 `scripts/verify.sh` 再动下一个。

## 环境与命令

工具链：Go ≥ 1.27、Node ≥ 20（本项目在 Go 1.27.3 / Node 22 上验证）。

| 目的 | 命令 |
|---|---|
| 装依赖（首次） | `cd web && npm ci`（后端 `go mod download`） |
| 起后端 :8080 | `cd server && go run ./cmd` |
| 起前端 :5173（热更新，/api 代理到 8080） | `cd web && npm run dev` |
| 提交前自检（**必跑**） | `bash scripts/verify.sh` |
| 构建单文件产物 | `bash build.sh` → `release/huoben.exe` |
| 只跑后端测试 | `cd server && go test ./...` |
| 重置为全新库 | 停服务 → 删 `data/huoben.db*` → 重启（自动建表 + `admin/admin123`） |
| 装行业演示数据 | `POST /api/v1/demo/init`，body `{"confirm":"RESET","template":"hardware\|convenience\|fresh\|wholesale"}` |

`data/` 一律不入库。**不要为了"跑通"去改 `data/` 里的东西再提交**。

## 不能破的规则（违反过的都出过 bug）

1. **不运行 = 没完成。** 后端改动必须真起服务、用 curl/python 打接口看返回；前端改动必须 `vue-tsc` 通过 + 浏览器实看。不要用"代码看起来对"作为交付依据。
2. **`go:embed` 是编译期嵌入。** 改了 `web/dist` 或 `server/internal/webembed/webdist/` 之后**必须重新 `go build`**，否则服务仍返回旧页面。排查"改了没生效"先怀疑这一条。
3. **业务规则只写在 `service/` 层。** api 层只做参数校验 + 调 service；repo 层只做 CRUD。禁止在 handler 里算库存/成本。
4. **金额一律 `int`（分），数量一律 `int64`（实际值 ×1000）。** 前端换算走 `utils/money.ts`，不要在组件里手写 `/100`。
5. **service 方法首参必须是 `ctx context.Context`，并通过 `s.db(ctx)` 取句柄。** 绕过去 = 跳过租户隔离，单机版不会报错但多租户会串数据。
6. **多租户模型上的批量 `Create` 会炸。** GORM 的租户插件处理不了 slice，必须逐元素插入。
7. **新建带唯一编码的表**：不要在 gorm tag 里写 `uniqueIndex`（AutoMigrate 会建回全局唯一），改在迁移里建 `(tenant_id, code)` 复合索引。
8. **盘点确认整单只做一次快照 + 一次调账。** 放进明细循环里会把多行差异互相抹平（v0.18 修过的真实 bug）。
9. **演示数据初始化若会触发作业单据，先关 `location_block_overflow`。** 演示货架本就接近满容，否则重置会被 2005 卡在半路留下残缺数据。
10. **改前先看这条规则为什么存在。** CONVENTIONS 里每条日期打头的约定，都对应一次线上/实测事故。

## 禁区

- 不提交：`data/`、`*.db`、`license.key`、`.jwt_secret`、`release/`、`node_modules/`、`.workbuddy/`、`*.log`（`.gitignore` 已覆盖，但**不要 `git add -A` 盲提**）。
- 不改 `server/internal/pkg/license/license.go` 的 `PublicKey`，除非你同时持有私钥并重签所有授权；改错了所有人授权立即失效。
- 不硬编码任何密钥/口令/私钥。授权私钥只经环境变量 `HUOBEN_LICENSE_KEY` 或 `-key-file` 传入，**永远不落代码、不落仓库**。
- 不动 `docs/` 里已定稿的历史章节（PRD 的里程碑、CHANGELOG 的旧版本），只在顶部追加新版本。
- 生产环境的 `data/` 目录不要拿来做测试（会真删数据）。测试用 `HUOBEN_PORT` + `HUOBEN_DB_PATH` 指向临时目录。

## 交付节奏（每轮迭代）

```
读 项目现状/TODO → 定范围（一次一个） → 改后端 → go build → 改前端 → vue-tsc
→ 实跑验证 → 清理临时文件/进程 → 同步文档 → git commit → 更新 docs/项目现状.md
```

**提交信息**：`feat(vX.Y): 一句话`、`fix: …`、`docs: …`、`chore: …`。一个提交一件事，不混格式化和功能。

**每轮结束必须更新这三处，否则下一个 agent 会重犯同样的错**：

| 文件 | 更新什么 |
|---|---|
| `docs/CHANGELOG.md` 顶部 | 新版本条目（改了什么 + 怎么验证的） |
| `docs/TODO.md` 顶部 | 版本章节与勾选；**没验证到的写在 `[ ]` 里**并说明原因 |
| `docs/项目现状.md` | 版本号、决策/坑（有新发现才加）、待办 |

## 已知未验证项（别当成已完成）

- Windows 服务 `huoben.exe -install` 的**真机安装**（`services.msc` 可见 + 重启自启）：开发环境无管理员权限，只验证到"非管理员明确报错"。
- Caddy 的**真实证书签发**：本机无公网域名，只验证了应用侧 `X-Forwarded-Proto` → HSTS 与真实 IP 透传。

动这两块之前先看 `docs/TODO.md` 里对应条目的原始描述。

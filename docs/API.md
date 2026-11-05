# API 接口文档

> 前缀 `/api/v1`，除登录外均需 `Authorization: Bearer <token>`
> 统一响应 `{code, msg, data}`；列表返回 `data: {list, total}`

## 认证
| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /auth/login | body: `{username, password, tenant_code?}` → `{token, display_name, tenant_id, tenant_code?/tenant_name?/plan_code?/expire_at?}`；`tenant_code` 为企业编码（SaaS 版必填），留空 = 单机版 |
| GET | /auth/me | 当前用户信息 |

## 商品分类（M1）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /categories?keyword= | 全量列表（不分页） |
| POST | /categories | `{name, parent_id, sort}`；同级重名→4003 |
| PUT | /categories/:id | 同上 |
| DELETE | /categories/:id | 有商品/子分类→4004 |

## 商品（M1）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /products?page=&page_size=&keyword=&category_id=&status= | keyword 匹配名称/编码，条码精确 |
| GET | /products/:id | 含 prices |
| POST | /products | 见下方 ProductReq |
| PUT | /products/:id | cost_method 不可改（→2005） |
| DELETE | /products/:id | 软删，同时删价位 |

**ProductReq**：`{code*, barcode, name*, category_id, unit, cost_method*(weighted_avg|fifo), is_batch_enabled, shelf_life_days, safe_stock(×1000), status, prices:[{name*, price(分), sort}], default_price}`
- FIFO 强制 is_batch_enabled=true
- code 重复→4001；条码被占用→4005；cost_method 非法→2003

## 往来单位 供应商/客户（M1）
路径中 `:kind` = `supplier` | `customer`
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /partners/:kind?page=&page_size=&keyword= | 分页 |
| POST | /partners/:kind | `{code*, name*, contact, phone, address, opening(分), remark}` |
| PUT | /partners/:kind/:id | 同上 |
| DELETE | /partners/:kind/:id | 软删 |

## 期初库存（M1）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /opening/stocks | 全部商品+当前库存（quantity ×1000, avg_cost 分） |
| POST | /opening | `{rows:[{product_id*, quantity(×1000), unit_cost(分), batch_no?}]}`；批次商品 batch_no 默认"期初"；已建账商品→4005 |

## 批量导入（M1）
| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /import/products | multipart `file`，xlsx；返回 `{success, skipped, errors:[{row,msg}]}` |
| POST | /import/partners/:kind | 同上，kind=supplier/customer |
| GET | /import/template/:kind | 下载模板（kind=products/supplier/customer），blob |

商品导入规则：分类不存在自动创建；表头以"价"结尾的列识别为价位；成本方式列填 加权平均/先进先出；编码重复跳过。

## 错误码
见 docs/CONVENTIONS.md（1xxx 通用 / 2xxx 库存 / 3xxx 单据 / 4xxx 档案）

## 应收应付（M2）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /finance/balances?kind=customer\|supplier | 余额总览：`[{partner_id, code, name, opening, biz, settled, balance}]`，balance = opening + biz − settled（分） |
| GET | /finance/payments?kind=&partner_id=&page=&page_size= | 收付款记录 |
| POST | /finance/payments | `{kind*(receipt\|payment), partner_id*, amount*(分>0), method(cash\|wechat\|alipay\|bank), doc_date, remark}` |
| POST | /finance/payments/:id/void | 作废（不计入余额） |

## 其他出入库（M2）
复用 `POST /docs`，doc_type=`other_in` / `other_out`，partner 可空；单价即入库成本（出库成本自动按均价/FIFO 结转）。

## 打印
`/print/:id` 前端 A4 打印页（非 API）。

## 报表与看板（M3）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /reports/summary?date_from=&date_to= | 进销存汇总：期初/入/出/期末 数量与金额（按库存流水重放） |
| GET | /reports/profit?date_from=&date_to= | 销售毛利：逐单收入/结转成本/毛利 + total 合计 |
| GET | /dashboard/stats | 看板：今日销售额/毛利/单据数、库存总值、应收/应付合计、低库存预警、货位容量预警、近 14 天销售趋势、最近 8 张单据 |
| GET | /inventory?keyword= | 库存查询（商品+库存+安全库存状态） |
| GET | /inventory/:id/batches | 某商品批次明细（FIFO 顺序，剩余量>0） |

## 盘点（M3 / v0.13 按库位 / v0.15 全量留痕 + 复盘 / v0.16 差异审批）
复用 `POST /docs`，doc_type=`stocktake`；items 数量=**实盘数**（允许 0 视为盘光，批次商品除外），price 为盘盈成本（可选）。
确认时自动按差额盘盈入库（无批次号入"盘点盈"批次）/盘亏出库，备注自动记录"账面X，盈亏±Y"。

**v0.15 起**：
- 明细**记录全部已盘行**（含账实相符），不再只写差异项——差异靠 `book_qty`（确认时账面快照）事后计算
- 每条明细确认后回填 `book_qty`（账面 ×1000）、`cost`（盘亏=结转成本 / 盘盈=盘盈成本），无差异行 `cost=0`
- 复盘：`POST /docs` 再传一次盘点单，`ref_doc_id` 指向初盘单即可（备注建议写「复盘（初盘 PDxxx）」）；账面取当前库存，差额按新值调整
- 差异表与 CSV 导出为前端页面 `/print/:id/diff`，复用 `GET /docs/:id`，不新增后端接口

**差异审批（v0.16）**：开启 `stocktake_review_enabled` 后，差异率超 `stocktake_review_percent`（默认 10%）或差异金额 ≥ `stocktake_review_amount` 分（按仓库均价估算，0=不按金额）的盘点单确认后**不调账**，`status=review`（待审批）、`review_status=pending`，命中原因写入备注；账面快照照常落库。
- `POST /docs/:id/review`，body `{action:"approve"|"reject", note?}`，仅 `system` 权限（主管）。通过 → 按确认时账面快照调账并转已生效（`review_status=approved`）；驳回 → 作废、库存不动（`review_status=rejected`，原因入 void_reason）
- 待审批单可作废撤回（未调账无需回冲）；未超阈值/未开启审批的盘点单确认即调账，行为不变

| 范围 | 请求 | 行为 |
|---|---|---|
| 全仓（默认） | 不传 `scope`；items 不带或带库位 | 账面=商品全仓数量；盘盈入明细库位（默认库位），盘亏由引擎自动分配库位 |
| 按库位（逐架） | `scope=location`；每条 items 必须带 `location_id`（`0`=未分配区） | 账面=该货位数量；差额就地调整该货位；不回填默认库位；同 `(商品,库位)` 重复行返回 1001 |

> 按库位盘点时批次商品实盘填 0 不报 2002；盘盈不受库位容量限制（账实必须如实记录）。
> 一张单可同时包含多个货位（逐架录完一次提交），作废按流水原路回冲各货位。

## 系统管理（M4，均需 system 权限；标注☆除外）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | /system/users | 用户列表 / 新建 `{username*, password*(≥6), display_name*, role_id*}` |
| PUT | /system/users/:id | `{display_name, role_id, status}`；不能停用/降级自己 |
| POST | /system/users/:id/password | 重置密码 `{password}` |
| GET/POST | /system/roles | 角色列表(含 user_count) / 新建 `{name*, remark, permissions*[]}` |
| POST/DELETE | /system/roles/:id | 更新 / 删除（有用户禁删） |
| POST | /auth/password ☆ | 修改自己密码 `{old_password, new_password}` |
| GET/PUT | /system/settings ☆读 | 业务设置：allow_negative_stock / shelf_life_warn_days / location_block_overflow；小票抬头：shop_name / shop_phone / shop_addr / ticket_footer（字符串，≤60 字，留空则小票不打印该行）；打印模板（v0.21）：print_logo（data:image/ 前缀 data URL，≤220KB）、ticket_show_price/amount/partner/batch、a4_show_price/amount/signs（布尔，默认 true；关金额即送货单） |；打印模板（v0.21）：print_logo（data:image/ 前缀 data URL，≤220KB）、ticket_show_price/amount/partner/batch、a4_show_price/amount/signs（布尔，默认 true；关金额即送货单）
| GET/POST | /system/backups | 备份列表 / 立即备份（VACUUM INTO 快照） |
| GET | /system/backups/:name | 下载备份文件 |
| GET | /system/audit-logs | 操作日志 ?username=&action=&page= |

## 权限模块码
`dashboard / archive / purchase / sales / finance / stock / reports / system`，`*`=全部。
单据接口按 doc_type 细控：采购类→purchase，销售类→sales，盘点/其他出入库→stock。

## 授权（M5）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /system/license | `{mode(licensed\|trial\|expired), owner, expires_at, trial_left, machine_code}` |
| POST | /system/license | multipart `file`=license.key；校验签名+机器码后落盘 data/license.key 即时生效 |

授权过期（5001）时：全部 POST/PUT/DELETE 被拦截，GET 与本接口放行。签发：`go run ./cmd/genlicense`。

## 库位（M6 档案 / v0.12 数量账 / v0.13 容量预警）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | /locations | 列表（读权限含 sales/purchase/stock）/ 新建 `{code?, name*, remark?, sort?, capacity_mode?, capacity?, min_qty?, warn_ratio?}`，重名 4001；`capacity_mode`: qty 数量（默认）/ volume 体积(升) / weight 重量(kg)，占用=Σ(数量×单件维度)，未登记维度的商品不计占用并计入 `unknown_skus`；响应带 `used_qty`（量纲随模式）、`used_unit`（件/L/kg） |
| GET | /locations/stock | 库位库存列表；`location_id` 不传=全部、`0`=未分配；支持 `keyword`（编码/名称/条码/PLU） |
| GET | /locations/alerts | 货位预警：返回超容/已满/接近满/低于下限的库位（同列表结构） |
| PUT/DELETE | /locations/:id | 更新（含 capacity_mode）/ 删除（被商品默认库位引用、或库位仍有库存 → 1003） |

`GET /locations` 每行除档案字段外还带实时占用：
`used_qty`（数量合计 ×1000）、`sku_count`、`usage_rate`（%，未设容量为 -1）、
`status`（`normal`/`near_full`/`full`/`over`/`under`/`empty`）、`status_text`（中文）。

- 库位**参与数量账**（v0.12）：`location_inventories` 按 商品×仓库×库位 记数量，成本仍在仓库级
- 商品 `default_location_id`：入库/盘点未填库位时自动带出；出库不填则由引擎自动分配（默认库位优先 → 其余按量降序补足）
- 单据明细落 `location_id`/`location_name` 快照；移库单另存 `to_location_id`/`to_location_name`
- 指定库位出库且数量不足 → `2004`（库位账恒禁负库存）；不指定库位的负库存差额挂「未分配」以暴露差异
- **移库单**：`POST /docs`，`doc_type=transfer`，items 传 `location_id`（移出，0=未分配）+ `to_location_id`（移入），不允许相同；只改库位分布，不动总量与成本
- **容量/上下限（v0.13）**：`capacity`（0=不限）、`min_qty`（0=不限）、`warn_ratio`（默认 90）；
  占用 = 该货位所有商品数量合计。入库/移入超容时，默认只提醒；系统设置 `location_block_overflow=true` 时返回 `2005`
- 看板 `GET /dashboard/stats` 增加 `location_alerts`（结构同上），供首页展示货位预警

## 演示数据（v0.10 起提供；v0.18 起 4 套行业模板，v0.20 补充批发）
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /demo/templates | 模板清单（公开）：`[{key, name, scene, detail, shop, products, sales_docs}]`，当前 hardware 五金 / convenience 便利店 / fresh 生鲜 / wholesale 快消品批发部 |
| POST | /demo/init | 登录页公开入口，body `{confirm:"RESET", template?}`；清空业务数据并按模板写入演示数据（template 缺省 hardware） |
| POST | /system/demo-data | 系统管理入口（system 权限），body 同上，按当前操作者记审计 |

返回 `{template, template_name, categories, products, suppliers, customers, locations, purchase_docs, sales_docs, other_docs, payments, stock_value_view}`。
清空范围：单据/明细/流水/批次/库存/库位库存/收付款/商品/价位/分类/供应商/客户/库位（硬删）；保留用户、角色、系统设置、授权、操作日志。
模板差异：小票抬头、价位组合、货位容量计量方式、批次号格式（按月/按日）、单据量；同一模板固定随机种子，重置结果一致。

## 秤码解析（v0.11，电子秤标签）

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | /scale-rules | archive/system/sales/purchase/stock | 规则列表（含停用） |
| POST | /scale-rules | archive/system | 新增规则 |
| PUT | /scale-rules/:id | archive/system | 修改规则 |
| DELETE | /scale-rules/:id | archive/system | 删除规则（删除默认规则后自动补位） |
| POST | /scale/parse | archive/system/sales/purchase/stock | 解析条码 `{code}` |

**ScaleRuleReq**：`{name*, brand, model, code_len, prefixes, plu_start, plu_len, amount_start, amount_len, amount_decimals, weight_start, weight_len, weight_decimals, check_type(mod10|none), enabled, is_default, sort, remark}`

**解析结果 ScaleParseResult**：`{matched, code, reason, rule_id, rule_name, plu, amount(分), quantity(×1000), source(weight|amount), product_id, product_code, product_name, unit, price(分)}`

- 段位置从 1 开始；`amount_start=0` 表示无金额段，`weight_start=0` 表示无重量段，二者至少一项
- 匹配顺序：`code_len` → `prefixes` → mod10 校验 → `plu` 精确匹配商品 → 计算金额/数量
- 18 位码同时含金额与重量时 `source=weight`（优先重量）；13 位金额码 `source=amount`，数量按商品默认售价反算
- `matched=false` 时 `reason` 可直接展示（校验位不通过 / PLU 未对应商品 / 非秤码）
- 内置预设（首次启动写入）：顶尖 Aclas LS 13 位金额码（默认，前缀 22）、顶尖 LS 13 位重量码（前缀 21）、大华/寺冈 18 位码

示例：`2200001025257` → 顶尖 13 位金额码、PLU `00001`、金额 ¥25.25、数量 2.003kg（默认售价 ¥12.60/kg）

## 单据打印（前端路由，无后端接口）

两种版式共用 `GET /docs/:id` 的数据，均为独立的无侧边栏页面，前端直接 `window.print()`：

| 路径 | 版式 | 页面设置 | 说明 |
|---|---|---|---|
| `/print/:id` | A4 纵向 | `@page { size: A4 portrait; margin: 0 }` | 正式单据：完整明细表、大写金额、签收栏，用于存档/客户签收 |
| `/print/:id/ticket` | 58mm 热敏小票 | `@page { size: 58mm auto; margin: 0 }` | 收银小票：窄版单列流式，纸宽 58mm / 可打印 48mm（约 16 汉字/行） |

小票版式要点：
- 抬头取系统设置 `shop_name` / `shop_phone` / `shop_addr`，票尾取 `ticket_footer`，任一为空则该行不打印
- 明细：品名独占一行（超长按字符折行），下一行「数量 × 单价 …… 金额」左右对齐；有批次时补一行批次号
- `status != confirmed`（草稿/作废）在票头加方框标注「不作结算凭证」
- 移库单：明细显示「移出货位 → 移入货位」，不打印单价/金额/合计
- 打印参数由用户在选择打印对话框时决定（纸张 58mm × 自动、缩放 100%、边距无、关闭页眉页脚）；本版不做 ESC/POS 直连，见 TODO Backlog

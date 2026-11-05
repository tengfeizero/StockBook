# 数据模型 v0.1（与 PRD v0.1 配套）

> 状态：v0.1 已确认（2026-11-03，随 PRD §7 决议一并确认） | 更新：2026-11-03
> 约定：所有业务表含 `id`(自增主键)、`tenant_id`(**v0.19 起生效**：0=单机版默认租户，SaaS 租户各用各的；查询/写入由租户插件按 ctx 自动隔离)、`created_at`/`updated_at`/`deleted_at`(软删)；金额字段单位为**分**；数量字段 3 位小数存储（整数×1000）。
> 业务编码唯一性在租户内：users.username / roles.name / products.code / suppliers.code / customers.code / warehouses.name / documents.doc_no / payments.doc_no 的唯一索引均为 (tenant_id, 编码)。

## ER 总览

```
categories 1──n products 1──n product_prices
suppliers 1──n documents, customers 1──n documents
documents 1──n document_items n──1 products
warehouses 1──n inventory 1──n inventory_batch(启用批次时)
documents(生效) ──生成──> inventory_movements(库存流水, 只追加)
```

## 基础档案

### categories 商品分类
| 字段 | 类型 | 说明 |
|---|---|---|
| name | text | 分类名 |
| parent_id | int | 预留二级分类，默认0 |

### products 商品
| 字段 | 类型 | 说明 |
|---|---|---|
| code | text | 商品编码，唯一 |
| barcode | text | 条码，可空，索引 |
| plu | text | 秤码 PLU（电子秤上的货号），可空，索引；散称商品必填且唯一，冲突 4006 |
| name | text | 名称 |
| category_id | int | |
| unit | text | 基本单位（个/箱/斤…） |
| cost_method | text | `weighted_avg` / `fifo`，建后不可改（改则需库存清零） |
| is_batch_enabled | bool | 是否启用批次（fifo 商品强制 true） |
| shelf_life_days | int | 保质期天数，批次商品用 |
| safe_stock | numeric | 安全库存下限，可空 |
| volume / weight | int | 单件体积（升×1000）/ 单件重量（kg×1000），按基本单位；0=未登记（v0.17，库位按体积/重量计量时占用换算用） |
| status | text | `active` / `disabled` |
| default_price_id | int | 默认售价指向 product_prices.id |

### product_prices 商品价位（多价位）
| 字段 | 类型 | 说明 |
|---|---|---|
| product_id | int | |
| name | text | 价位名（零售价/批发价/会员价，租户自定义） |
| price | int | 分 |
| sort | int | 排序 |

### scale_rules 秤码规则（v0.11，多品牌可扩展）
| 字段 | 类型 | 说明 |
|---|---|---|
| name | text | 规则名，如「顶尖 Aclas LS · 13位金额码」 |
| brand / model | text | 品牌 / 适用型号 |
| code_len | int | 条码总位数（13 / 18），0=不限 |
| prefixes | text | 前缀，逗号分隔（`"21,22"`），空=不限 |
| plu_start / plu_len | int | PLU 段起始位（1-based）/ 长度 |
| amount_start / amount_len / amount_decimals | int | 金额段；起始位 0=该码制无金额段，小数位 2=分 |
| weight_start / weight_len / weight_decimals | int | 重量段；起始位 0=无重量段，小数位 3=克 |
| check_type | text | `mod10`（EAN 校验）/ `none` |
| enabled / is_default / sort | bool / bool / int | 启用、默认规则（全局唯一）、排序 |
| remark | text | 备注 |

> 金额段与重量段至少配置一项；两者同时存在（18 位码）时解析优先取重量。

### suppliers / customers
| 字段 | 类型 | 说明 |
|---|---|---|
| code | text | 编码 |
| name | text | 名称 |
| contact / phone / address | text | 联系信息 |
| opening_receivable | int | 期初应收（客户）/ 期初应付（供应商），分 |

### locations 库位/货架（v0.13 起带容量，v0.17 可按体积/重量计）
| 字段 | 类型 | 说明 |
|---|---|---|
| code / name / remark / sort | text / text / text / int | 编码、名称（唯一）、备注、排序 |
| capacity_mode | text | 容量计量方式（v0.17）：`qty` 数量（默认）/ `volume` 体积 / `weight` 重量 |
| capacity | numeric(3位小数) | 容量上限，量纲随 capacity_mode（×1000）；0=不限（v0.13） |
| min_qty | numeric(3位小数) | 下限，同量纲；0=不限（v0.13） |
| warn_ratio | int | 近满预警阈值百分比，默认 90（v0.13） |

> 占用计算随模式：qty = `Σ quantity`；volume = `Σ quantity×单件体积/1000`（升）；weight = `Σ quantity×单件重量/1000`（kg）。
> 商品未登记对应维度（volume/weight=0）时**不计占用**，`GET /locations` 返回 `unknown_skus` 提示补档案。
> 所有库存量纲内部 ×1000；状态判定（正常/接近满/已满/超容/低于下限/空置）逻辑与量纲无关。

### warehouses 仓库（单仓预留）
| 字段 | 类型 | 说明 |
|---|---|---|
| name | text | 默认建一条"主仓库" |

### users / roles（M4，表结构 M0 建）
- users: username, password_hash, display_name, role_id, status
- roles: name；role_permissions: role_id + 权限码

## 库存

### inventory 库存汇总（product × warehouse 一行）
| 字段 | 类型 | 说明 |
|---|---|---|
| product_id / warehouse_id | int | 联合唯一 |
| quantity | numeric(3位小数) | 当前数量 |
| avg_cost | int | 当前移动加权平均成本（分） |
| total_value | int | 库存金额（分）= quantity × avg_cost 的最新值 |

### location_inventories 库位库存（v0.12 库位数量账）
| 字段 | 类型 | 说明 |
|---|---|---|
| product_id / warehouse_id / location_id | int | 三元组联合唯一；`location_id=0` 表示**未分配**（存量升级时整体挂在这里） |
| quantity | numeric(3位小数) | 该库位当前数量，可为负数仅限「未分配 + 全局允许负库存」场景 |

> 不变式：同一 `(product_id, warehouse_id)` 的 `Σ location_inventories.quantity == inventory.quantity`。
> 成本不按库位拆——`avg_cost`/`total_value` 仍在 `inventory`（仓库级）维护，库位只拆数量。

### inventory_batch 批次库存（仅启用批次商品）
| 字段 | 类型 | 说明 |
|---|---|---|
| product_id / warehouse_id | int | |
| batch_no | text | 批次号（可自动=入库日期） |
| production_date / expire_date | date | 可空 |
| quantity_in / quantity_out | numeric | 累计进/出，剩余 = in - out |
| unit_cost | int | 该批次单位成本（FIFO 依据，加权平均商品也可记录供追溯） |

## 单据

### documents 单据主表（全部类型共用）
| 字段 | 类型 | 说明 |
|---|---|---|
| doc_no | text | 单号，业务层生成，唯一 |
| doc_type | text | 见下表 |
| status | text | `draft` / `confirmed` / `voided` / `review`（v0.16 盘点差异待审批，未调账） |
| review_status | text | 盘点审批：空=无 / `pending` 待审批 / `approved` 已通过 / `rejected` 已驳回（仅盘点单） |
| review_note / review_by / review_at | text/int/time | 审批意见 / 审批人 / 审批时间 |
| scope | text | 盘点范围：空=全仓（默认）、`location`=按库位逐架盘（v0.13，其他单据恒空） |
| partner_type / partner_id | text/int | 供应商或客户（无往来的单据为空） |
| doc_date | date | 单据日期 |
| total_qty / total_amount | numeric/int | 合计数量 / 金额（分） |
| remark | text | |
| confirmed_at / voided_at | datetime | |
| void_reason | text | 作废原因 |
| ref_doc_id | int | 关联单据（销售退货→原销售单；采购退货→原采购单），可空 |

**doc_type 枚举**：`purchase_in` 采购入库 · `purchase_order` 采购订单 · `sales_out` 销售出库 · `quotation` 报价单 · `purchase_return` 采购退货 · `sales_return` 销售退货 · `stocktake` 盘点 · `transfer` 移库（同仓库内库位间搬移，单号前缀 DB） · `other_in` 其他入库 · `other_out` 其他出库

### document_items 单据明细
| 字段 | 类型 | 说明 |
|---|---|---|
| document_id | int | |
| product_id | int | |
| quantity | numeric | 正负号由 doc_type 语义决定，明细一律存正数；**盘点单=实盘数** |
| book_qty | int | **盘点单专用**：确认时点的账面数（×1000），其他单据恒 0；差异 = quantity − book_qty（v0.15） |
| price | int | 单价（分）；盘点单为盘盈成本（可选），其他类语义见 service |
| cost | int | 出库类单据：结转单位成本（确认时回填）；入库类：入库成本 |
| price_name | text | 使用的价位名（销售单） |
| batch_id | int | 批次（启用批次时） |
| location_id / location_name | int/text | 库位 + 快照；移库单此处为**移出**库位，0=未分配 |
| to_location_id / to_location_name | int/text | 移库单**移入**库位（其他单据恒 0） |
| amount | int | 金额=数量×单价 |
| remark | text | |

### inventory_movements 库存流水（只追加，不修改——审计与对账依据）
| 字段 | 类型 | 说明 |
|---|---|---|
| document_id / document_item_id | int | 来源单据 |
| product_id / warehouse_id | int | |
| batch_id | int | 可空 |
| location_id | int | 发生库位，0=未分配；跨库位出库会拆成多条流水 |
| direction | text | `in` / `out` |
| quantity | numeric | 一律正数 |
| unit_cost | int | 结转/计入单位成本 |
| created_at | datetime | |

> 对账不变式：`inventory.quantity == sum(inventory_movements 入) - sum(出)`；
> 库位层不变式：`Σ location_inventories.quantity == inventory.quantity`（按 product+warehouse 分组）。
> 移库单只改写库位分布，不改变仓库总量与成本，故其流水成对出现（源库位 out / 目标库位 in）。

## 往来账款（待确认后实现，建议 M2）

- receivables / payables：单据确认时生成应收（sales_out）/应付（purchase_in）记录
- receipts / payments：收款单 / 付款单（doc_type 复用 documents，或独立表——实现时定）
- partner_balance 视图：期初 + 单据 - 收付款 = 当前欠款

## 租户与订阅（v0.19，SaaS）

**tenants**（平台级，自身不带 tenant_id）：`id`、`code`(企业编码，全库唯一，登录用)、`name`、`contact/phone`、`plan_code`、`status`(active/suspended)、`trial_ends_at`、`expire_at`(订阅到期)、`max_users`、`remark`

**plans**（平台维护）：`code`(trial/basic/pro)、`name`、`price_monthly`(分/月)、`max_users`、`max_products`(0=不限)、`features`(文案)、`sort`、`status`

**tenant_bills**（业务表，按租户隔离）：`bill_no`、`plan_code/plan_name`、`months`、`amount`(分)、`status`(paid，当前为线下/演示支付)、`period_start/period_end`、`operator`、`remark`

## 系统设置（key-value 表 tenant_settings，主键 tenant_id+key）

| key | 默认 | 说明 |
|---|---|---|
| allow_negative_stock | false | 负库存开关 |
| shelf_life_warn_days | 7 | 临期预警天数 |
| location_block_overflow | false | 库位超容是否禁止入库/移入（v0.13，默认只提醒不拦截） |
| shop_name | 空 | 58mm 小票抬头-店铺名称（v0.14） |
| shop_phone | 空 | 小票抬头-联系电话 |
| shop_addr | 空 | 小票抬头-店铺地址 |
| ticket_footer | 空 | 小票票尾寄语（如「谢谢惠顾，欢迎再来」） |
| stocktake_review_enabled | false | 盘点差异超阈值需主管审批（v0.16） |
| stocktake_review_percent | 10 | 差异率阈值（%，\|差异\|÷账面数；0=不按差异率，账面为 0 的行不参与） |
| stocktake_review_amount | 0 | 差异金额阈值（分，按仓库均价估算；0=不按金额） |

设置按租户存储（0 号租户=单机版，旧 settings 表数据自动迁入 0 号租户后改名 settings_legacy_v019 备份）。读写一律走 `model.GetSetting/SetSetting`。小票抬头 4 个键留空时，小票对应行不打印（不做占位），单字段上限 60 字（后端 `PUT /system/settings` 校验）。
早期文档中的 `require_confirm` / `allow_price_edit` / `expiry_warning_days` / `doc_no_prefix_*` 为规划键，当前代码未落地（单号前缀硬编码、单据确认由前端 `confirm_now` 决定），不要按旧文档配置。

## 单据确认时的库存/成本处理顺序（service 层实现规范）

1. 校验：商品状态、数量>0、批次必填校验、（默认）负库存校验
2. FIFO 商品：按批次剩余量先进先出分配出库，逐批写流水并结转成本；批次不足且允许负库存→不足部分按当前 avg_cost 结转并记预警
3. 加权平均商品：出库按 inventory.avg_cost 结转；入库后重算 avg_cost
4. 写 inventory_movements、更新 inventory / inventory_batch、回填 document_items.cost
5. （待确认）生成应收应付
6. 以上步骤在同一数据库事务内完成

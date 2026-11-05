<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUiStore } from '../stores/ui'
import { Printer, Compass } from 'lucide-vue-next'

// 使用手册：内容按「场景」组织，左侧目录吸顶，右侧按需滚动。
// 与 docs/用户手册.md 同源，改一处记得同步另一处。
interface Item {
  h?: string
  p?: string
  list?: string[]
  tip?: string
}
interface Section {
  id: string
  title: string
  items: Item[]
}

const router = useRouter()
const ui = useUiStore()
const activeId = ref('start')
const contentRef = ref<HTMLElement | null>(null)

const sections: Section[] = [
  {
    id: 'start',
    title: '快速上手（3 分钟）',
    items: [
      {
        p: '新账套按这个顺序走，后面开单才不会卡：',
        list: [
          '① 基础档案 → 商品档案：先建商品（或「批量导入」下载模板填好上传，编码必填、可带 PLU 与条码）',
          '② 基础档案 → 期初库存：录入上架数量与成本，这一步决定「库存总值」和毛利基数',
          '③ 基础档案 → 供应商 / 客户：期初应收应付在这里录',
          '④ 采购销售 → 销售出库：选客户、加商品、确认生效，库存自动扣减',
        ],
      },
      {
        h: '想先看看效果？',
        p: '登录页点「选个行业一键体验」，可选五金建材、便利店、生鲜果蔬、快消批发四套演示数据（含两个月真实单据）。注意：重置演示数据会清空当前账套，正式数据请先备份。',
        tip: '系统设置 → 演示数据页签也能随时切换行业模板，仅影响当前企业。',
      },
    ],
  },
  {
    id: 'doc',
    title: '开单与单据状态',
    items: [
      {
        p: '所有单据共用一套状态机：',
        list: [
          '草稿：随便改，不影响库存和往来账；关掉页面也不会丢',
          '生效（确认）：库存、成本、应收应付同时落账',
          '作废：反向流水回冲，库存与欠款自动还原，单据留档可查',
        ],
      },
      {
        h: '哪些单据不动库存？',
        p: '采购订单、报价单只做记录和跟单；盘点、其他出入库、移库会动库存，但移库只改货位分布、不改总量成本。',
      },
      {
        h: '开单提速技巧',
        list: [
          '商品行支持扫码：光标在商品框直接扫条码/秤码，同商品重复扫码自动累加数量',
          '开单页底部是本类型最近 8 张单据，点一下就能定位去看',
          '单价默认带出商品价，改价不会影响商品档案',
        ],
      },
    ],
  },
  {
    id: 'stock',
    title: '库存、批次与货位',
    items: [
      {
        p: '库存永远 = 期初 + 入库 − 出库，全部由流水重放得出，不手工维护。',
        list: [
          '成本方式：移动加权平均（适合大多数）或 FIFO 先进先出（强制批次，适合食品/药品）',
          '批次与保质期：开启批次的商品入库要录批次与到期日，库存查询可展开看批次明细，临期有预警',
          '负库存：默认禁止（2004），确有需要可在其他设置里放开',
          '安全库存：低于该值会在首页和库存查询里标红',
        ],
      },
      {
        h: '货位（库位）',
        p: '按货架管货：商品可设默认货位，出库自动按默认货位优先分配，不够会跨货位拆流水。移库单只在货架之间搬货，不动金额。货位可设容量与上下限，容量支持按数量 / 体积 / 重量三种计量，超容可只提醒也可硬拦截。',
      },
    ],
  },
  {
    id: 'stocktake',
    title: '盘点与差异处理',
    items: [
      {
        p: '两种盘法：全仓盘点（按商品总量）与按货架盘点（逐架盘，差额就地调整该货架，可跨货架一次提交）。',
        list: [
          '提交后记录全部已盘行（含账实相符），并留账面快照',
          '差异表可打印、可导出 CSV，盘盈盘亏与差异金额一目了然',
          '差异超过阈值（差异率默认 10%，可配差异金额）时，单据转「待审批」，不调账，主管通过后才生效',
          '按差异项可「复盘」：只盘上次有差异的商品，自动生成关联初盘的新盘点单',
        ],
      },
      {
        tip: '盘亏到 0 的商品复盘时会自动补行，否则没有地方录实际数量。',
      },
    ],
  },
  {
    id: 'print',
    title: '打印与票据',
    items: [
      {
        list: [
          'A4 单据：完整明细 + 大写金额 + 签收栏，适合存档与客户签字',
          '58mm 小票：热敏小票机，浏览器直接打印（Ctrl+P 选打印机）',
          '打印模板：其他设置 → 打印模板，可上传 Logo、控制单价/金额/客户/批次等字段显隐；把金额关掉就是一张送货单',
        ],
      },
      {
        h: '秤码（散称商品）',
        p: '电子秤标签码可直接扫进开单页：内置顶尖、大华、寺冈等常见码制，商品档案里填 PLU 即可。其他设置 → 秤码规则可新增品牌规则并在线测试。',
      },
    ],
  },
  {
    id: 'money',
    title: '资金与报表',
    items: [
      {
        list: [
          '应收应付按单据实时聚合，收款单/付款单冲抵欠款，作废会回滚',
          '进销存汇总按流水重放，期末数永远等于库存查询结果',
          '销售毛利按单据明细算，成本取当时的结转成本',
        ],
      },
      {
        tip: '对不上账时先看「单据查询」里有没有未生效或已作废的单据，再看库存流水。',
      },
    ],
  },
  {
    id: 'people',
    title: '多人使用与权限',
    items: [
      {
        p: '权限分四层：菜单可见、页面可进、接口可调、单据级（如只能看自己的单）。角色在「用户与角色」里配，改完立即生效，不用重新登录。',
        list: [
          '操作日志记录登录与所有变更类操作，密码自动脱敏',
          '多人同时用：服务端装一台机器，其他人浏览器访问 http://服务器IP:8080',
          '公网访问请务必配 HTTPS（见部署文档），否则密码是明文传输的',
        ],
      },
    ],
  },
  {
    id: 'data',
    title: '数据、备份与迁移',
    items: [
      {
        list: [
          '所有数据在程序目录的 data/ 下（SQLite 单文件），备份也是文件拷贝',
          '每天首次启动自动备份到 data/backups/，保留最近若干份',
          '其他设置 → 数据备份：可手动立即备份、下载备份文件到本地',
          '换机迁移 = 整个目录拷过去，新机器上双击运行即可',
        ],
      },
      {
        tip: '以 Windows 服务方式运行时日志在 data/logs/jxc-YYYY-MM-DD.log，排查问题先看这里。',
      },
    ],
  },
  {
    id: 'license',
    title: '授权与订阅',
    items: [
      {
        h: '单机 / 私有部署',
        p: '按机器码授权：运行 jxc.exe -machine-code 拿到机器码发给服务商，收到 license.key 后在「其他设置 → 授权信息」导入即生效。未授权可全功能试用 30 天，到期后只读（能查看和导出，不能改数据）。',
      },
      {
        h: 'SaaS 订阅',
        p: '注册开户后 14 天试用（2 用户 / 100 商品），续费在「订阅与计费」。到期只读，续费立即恢复；用户数超过套餐上限会被拦截。',
      },
    ],
  },
  {
    id: 'faq',
    title: '常见问题',
    items: [
      {
        h: '忘记管理员密码怎么办？',
        p: '让另一个有 system 权限的账号在「用户与角色」里重置；如果是唯一管理员且密码丢失，需要用备份库找回或联系服务商。',
      },
      {
        h: '库存数量和实际对不上？',
        p: '先查该商品的库存流水（库存查询 → 批次/流水），确认有没有漏录的入库或未作废的错误单据；确认无误后用盘点单盘平，差异留痕可追溯。',
      },
      {
        h: '单据开错了能改吗？',
        p: '草稿可以直接改；已生效的不能改，只能作废后重开——作废会自动回冲库存和欠款，账不会乱。',
      },
      {
        h: '点打印没反应 / 版式错位？',
        p: '浏览器打印对话框里把「边距」设为默认或最小值，纸张选 58mm（小票）或 A4；建议用 Chrome / Edge，并在打印设置里关闭「页眉页脚」。',
      },
      {
        h: '提示「授权已过期」或「订阅到期」？',
        p: '单机版导入新的 license.key；SaaS 版去「订阅与计费」续费。到期后系统变只读，数据不会丢。',
      },
      {
        h: '升级后页面还是旧的？',
        p: 'index.html 已设不缓存，正常刷新即可；若仍异常，Ctrl+F5 强制刷新或清一下浏览器缓存。',
      },
      {
        h: '扫码枪扫了没反应？',
        p: '确认商品档案里填了条码或 PLU，且光标在商品输入框；秤码需要「其他设置 → 秤码规则」里有对应品牌规则。',
      },
      {
        h: '能把系统放到云服务器上吗？',
        p: '可以。跑 jxc.exe 或用 systemd 托管，前面用 Caddy 反向代理做 HTTPS，具体见 README 的「HTTPS 部署」章节。',
      },
    ],
  },
  {
    id: 'keys',
    title: '小技巧',
    items: [
      {
        list: [
          '列表页搜索框回车即查，不用点按钮',
          '单据查询支持 URL 直达：/doc-list?doc_type=sales_out&keyword=XXX',
          '小票页可直接调起打印：单据详情 → 打印小票',
          '顶栏「?」随时重新打开新手引导',
        ],
      },
    ],
  },
]

function scrollTo(id: string) {
  activeId.value = id
  const el = document.getElementById('sec-' + id)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function startTour() {
  router.push('/')
  setTimeout(() => ui.startTour(), 400)
}

function printPage() {
  window.print()
}

onMounted(() => {
  const onScroll = () => {
    const box = contentRef.value
    if (!box) return
    for (const s of sections) {
      const el = document.getElementById('sec-' + s.id)
      if (el && el.getBoundingClientRect().top < 160) activeId.value = s.id
    }
  }
  window.addEventListener('scroll', onScroll, true)
})
</script>

<template>
  <div class="help">
    <aside class="toc">
      <div class="toc-title">使用手册</div>
      <div
        v-for="s in sections" :key="s.id" class="toc-item"
        :class="{ active: activeId === s.id }" @click="scrollTo(s.id)"
      >
        {{ s.title }}
      </div>
      <el-divider style="margin: 12px 0" />
      <el-button size="small" style="width: 100%" @click="startTour">
        <el-icon style="margin-right: 4px"><Compass /></el-icon>重新看引导
      </el-button>
      <el-button size="small" style="width: 100%; margin: 8px 0 0 0" @click="printPage">
        <el-icon style="margin-right: 4px"><Printer /></el-icon>打印本页
      </el-button>
    </aside>

    <main ref="contentRef" class="content">
      <div class="head">
        <h1>云进销存 · 使用手册</h1>
        <p>按「先建档案 → 再开单 → 最后对账」的顺序看，遇到问题直接搜下面的常见问题。</p>
      </div>

      <section v-for="s in sections" :id="'sec-' + s.id" :key="s.id" class="sec">
        <h2>{{ s.title }}</h2>
        <div v-for="(it, i) in s.items" :key="i" class="item">
          <h4 v-if="it.h">{{ it.h }}</h4>
          <p v-if="it.p">{{ it.p }}</p>
          <ul v-if="it.list">
            <li v-for="(li, j) in it.list" :key="j">{{ li }}</li>
          </ul>
          <div v-if="it.tip" class="tip">{{ it.tip }}</div>
        </div>
      </section>

      <div class="foot">
        还有问题？把现象和截图发给服务商，或用「其他设置 → 数据备份」导一份备份，便于排查。
      </div>
    </main>
  </div>
</template>

<style scoped>
.help {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}
.toc {
  width: 190px;
  flex: 0 0 190px;
  position: sticky;
  top: 0;
  padding: 4px 0;
}
.toc-title {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
  padding-left: 10px;
}
.toc-item {
  font-size: 13px;
  padding: 7px 10px;
  border-radius: 6px;
  cursor: pointer;
  color: var(--el-text-color-regular);
  border-left: 2px solid transparent;
}
.toc-item:hover {
  background: var(--el-fill-color-light);
}
.toc-item.active {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-left-color: var(--el-color-primary);
}
.content {
  flex: 1;
  min-width: 0;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 24px 28px 32px;
}
.head h1 {
  font-size: 20px;
  margin: 0 0 6px;
  font-weight: 600;
}
.head p {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.sec {
  margin-top: 26px;
  scroll-margin-top: 12px;
}
.sec h2 {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.item {
  margin-bottom: 12px;
}
.item h4 {
  font-size: 13px;
  font-weight: 600;
  margin: 12px 0 6px;
  color: var(--el-text-color-primary);
}
.item p {
  font-size: 13px;
  line-height: 1.85;
  color: var(--el-text-color-regular);
  margin: 0 0 6px;
}
.item ul {
  margin: 4px 0 6px;
  padding-left: 20px;
}
.item li {
  font-size: 13px;
  line-height: 1.9;
  color: var(--el-text-color-regular);
}
.tip {
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-radius: 6px;
  padding: 8px 10px;
  margin-top: 6px;
}
.foot {
  margin-top: 28px;
  padding-top: 14px;
  border-top: 1px dashed var(--el-border-color);
  font-size: 12.5px;
  color: var(--el-text-color-secondary);
}
</style>

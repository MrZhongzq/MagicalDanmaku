<script setup lang="ts">
/**
 * Accounts 是「账号与直播间」页——设计文档 §7.2 把它排第一，因为
 * B 站登录态失效得很快，这里是唯一能重新扫码续命的入口。
 *
 * 登录状态现在接的是真实数据（`cmd/magicd` 里 10 分钟一轮的检测循环，
 * 启动时立刻测一次）：`Account.loginState` 是 `valid`/`invalid`/`unknown`
 * 三态之一，`loginCheckedAt` 是最近一次检测时间（`null` 表示从未检测过）。
 *
 * **三态的区别很重要，不能把 unknown 当成 invalid 的弱化版显示**：
 * 探测本身失败（比如后端连不上 B 站）也会落在 unknown，与「从未检测过」
 * 是同一档。把 unknown 显示成「已失效」会在网络抖动时把用户吓得去重新
 * 扫码——账号可能什么问题都没有。见下面 `loginStateMeta` 的说明。
 *
 * 每个绑定还带着直播间开播状态（`Binding.liveStatus`，60 秒一轮心跳 +
 * 新增绑定时立即检测一次）与主播身份（`anchorUid`/`anchorName`，不是
 * 房间号）——三态设计与账号登录态完全对称，见下面 `roomLiveStateMeta`
 * 的说明，同样绝不能把 unknown 显示成「未开播」。
 */
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NCard,
  NDivider,
  NEmpty,
  NInput,
  NInputNumber,
  NModal,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  useDialog,
  useMessage,
} from 'naive-ui'
import { ApiError, request } from '@/api'
import type { Account, Binding } from '@/api'
import { useBindingsStore } from '@/stores/bindings'
import QRCodeLogin from '@/components/QRCodeLogin.vue'

const bindings = useBindingsStore()
const message = useMessage()
/**
 * useDialog() 要求外层有 NDialogProvider——App.vue 里已经套好了，生产环境
 * 一定能拿到。
 *
 * **不要在这里包 try/catch 退化成「拿不到就直接执行」。** 删账号/删绑定都
 * 是连带删规则、删授权的破坏性操作，缺 provider 是配置错误，应当响亮地
 * 抛异常，而不是悄悄跳过二次确认——静默跳过确认比抛异常更危险：抛异常
 * 至少会被看见，静默跳过会在某天这个组件被挂到一个没套 NDialogProvider
 * 的地方（Storybook、新测试、某种嵌入）时，无声地把删除操作变成不可撤销
 * 的一键删。Shell.test.ts 已经补上了 NDialogProvider，不需要这层退化。
 */
const dialog = useDialog()

const accounts = ref<Account[]>([])
const loadingAccounts = ref(false)

/** 账号参数的编辑草稿，按账号 id 存。改完点保存才提交，不是输入即生效。 */
const drafts = reactive<Record<number, { rateLimitMs: number; maxLength: number }>>({})
/** 「加直播间」输入框的草稿，同样按账号 id 存。 */
const newRoomId = reactive<Record<number, string>>({})

async function loadAccounts() {
  loadingAccounts.value = true
  try {
    accounts.value = await request<Account[]>('GET', '/api/accounts')
    for (const a of accounts.value) {
      drafts[a.id] = { rateLimitMs: a.rateLimitMs, maxLength: a.maxLength }
      if (!(a.id in newRoomId)) newRoomId[a.id] = ''
    }
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载账号列表失败')
  } finally {
    loadingAccounts.value = false
  }
}

/**
 * 「四」：账号区按账号组织，而 GET /api/bindings 可能带回你不拥有其
 * 账号的绑定（别人授权给你的）——这条不在设计文档里，是实现时才浮现的。
 * 不单独处理的话这些绑定会一个都显示不出来，而用户在别的页面明明能
 * 操作它们。所以这里只把「我拥有的账号」当作卡片来渲染；
 * 卡片之外看不到的绑定单独归进下面「他人授权给我的直播间」一块，只读。
 */
const ownedAccounts = computed(() => accounts.value.filter((a) => a.isOwner))
const ownedAccountIds = computed(() => new Set(ownedAccounts.value.map((a) => a.id)))

const bindingsByAccount = computed(() => {
  const map = new Map<number, Binding[]>()
  for (const b of bindings.list) {
    if (!ownedAccountIds.value.has(b.accountId)) continue
    const arr = map.get(b.accountId)
    if (arr) arr.push(b)
    else map.set(b.accountId, [b])
  }
  return map
})

/** 我不拥有其账号、但通过授权能看到的绑定——只读，不给启停/删除按钮。 */
const grantedBindings = computed(() =>
  bindings.list.filter((b) => !ownedAccountIds.value.has(b.accountId)),
)

/**
 * loginStateMeta 把 `Account.loginState` + `loginCheckedAt` 翻译成界面文案。
 *
 * **`unknown` 单独一档，不是 `invalid` 的弱化版**：探测本身失败（网络不通、
 * B 站接口超时）与「从未检测过」都会落在这里，与「确认失效」是完全不同
 * 的两件事，绝不能显示成「已失效」——那会在断网时把用户吓得去重新扫码。
 */
function loginStateMeta(acc: Account): {
  tagType: 'success' | 'error' | 'default'
  text: string
  detail: string
} {
  const checkedAt = acc.loginCheckedAt
  if (acc.loginState === 'valid') {
    return {
      tagType: 'success',
      text: '登录有效',
      detail: checkedAt ? `上次检测：${checkedAt}` : '上次检测：无',
    }
  }
  if (acc.loginState === 'invalid') {
    return {
      tagType: 'error',
      text: '登录已失效',
      detail: checkedAt
        ? `上次检测：${checkedAt}，确认登录已失效，请重新扫码`
        : '确认登录已失效，请重新扫码',
    }
  }
  // unknown：区分「从未检测过」与「上次检测失败」，两种情况文案不同，
  // 但都不能读成「已失效」。
  return {
    tagType: 'default',
    text: checkedAt ? '状态未知' : '尚未检测',
    detail: checkedAt
      ? `上次尝试检测于 ${checkedAt}，但未能确认结果（例如网络问题），不代表账号已失效`
      : '还没有被检测过，后台每 10 分钟轮询一次，稍等即可看到结果',
  }
}

/**
 * roomLiveStateMeta 把 `Binding.liveStatus` + `liveCheckedAt` 翻译成界面
 * 文案，与 `loginStateMeta` 是同一套三态设计。
 *
 * **`unknown` 绝不能显示成「未开播」**：探测本身失败（网络不通、被
 * 风控——`api/client.go` 里的 `riskControlCode` 在这个项目上真发生
 * 过）与「确认未开播」是完全不同的两件事。把探测失败显示成「未开播」
 * 是在用一个看起来正常的错误答案骗用户。
 */
function roomLiveStateMeta(b: Binding): {
  tagType: 'success' | 'default' | 'warning'
  text: string
  detail: string
} {
  const checkedAt = b.liveCheckedAt
  if (b.liveStatus === 'living') {
    return {
      tagType: 'success',
      text: '直播中',
      detail: checkedAt ? `上次检测：${checkedAt}` : '上次检测：无',
    }
  }
  if (b.liveStatus === 'offline') {
    return {
      tagType: 'default',
      text: '未开播',
      detail: checkedAt ? `上次检测：${checkedAt}` : '上次检测：无',
    }
  }
  // unknown：区分「从未检测过」与「上次探测失败」，两种情况文案不同，
  // 但都不能读成「未开播」。
  return {
    tagType: 'warning',
    text: checkedAt ? '状态未知' : '尚未检测',
    detail: checkedAt
      ? `上次尝试检测于 ${checkedAt}，但未能确认结果（例如网络问题或被风控），不代表未开播`
      : '加入后会立即检测一次，之后每 60 秒刷新一次',
  }
}

/** 主播身份（UID + 昵称）的展示文案，探测成功前统一显示"未知"。 */
function anchorMeta(b: Binding): { uid: string; name: string } {
  return {
    uid: b.anchorUid || '未知',
    name: b.anchorName || '未知',
  }
}

function isDirty(acc: Account): boolean {
  const d = drafts[acc.id]
  if (!d) return false
  return d.rateLimitMs !== acc.rateLimitMs || d.maxLength !== acc.maxLength
}

async function saveAccountParams(acc: Account) {
  const draft = drafts[acc.id]
  if (!draft) return
  try {
    const updated = await request<Account>(
      'PATCH',
      `/api/accounts/${encodeURIComponent(acc.name)}`,
      { rateLimitMs: draft.rateLimitMs, maxLength: draft.maxLength },
    )
    const idx = accounts.value.findIndex((a) => a.id === acc.id)
    if (idx !== -1) accounts.value[idx] = updated
    drafts[acc.id] = { rateLimitMs: updated.rateLimitMs, maxLength: updated.maxLength }
    message.success('账号参数已保存')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '保存失败')
  }
}

function confirmDeleteAccount(acc: Account) {
  dialog.warning({
    title: '删除账号',
    content: `确定要删除账号「${acc.name}」吗？这会连带删除它名下的全部直播间绑定与规则，且不可恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: () => void deleteAccount(acc),
  })
}

async function deleteAccount(acc: Account) {
  try {
    await request('DELETE', `/api/accounts/${encodeURIComponent(acc.name)}`)
    accounts.value = accounts.value.filter((a) => a.id !== acc.id)
    message.success('账号已删除')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '删除失败')
  } finally {
    // 删账号会带走它名下的全部绑定，顶部的绑定选择器必须跟着刷新，
    // 否则用户会在选择器里看到一个已经不存在的直播间。
    await bindings.refresh()
  }
}

async function toggleBinding(b: Binding, enabled: boolean) {
  try {
    await request('PATCH', `/api/bindings/${b.id}`, { enabled })
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失败')
  } finally {
    // 启停之后要自己刷新绑定列表——Shell 只在 onMounted 时拉一次，
    // 不刷新的话顶部选择器上的「（已停用）」标记不会更新。
    await bindings.refresh()
  }
}

/**
 * 删绑定与删账号是同一类破坏性操作——会连带删掉这个直播间的全部规则
 * 与授权，只是简报没明写要二次确认。既然删账号要确认，这里没理由不确认。
 */
function confirmDeleteBinding(b: Binding) {
  dialog.warning({
    title: '删除直播间',
    content: `确定要删除房间「${b.roomId}」吗？这会连带删除它的全部规则与授权，且不可恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: () => void deleteBinding(b),
  })
}

async function deleteBinding(b: Binding) {
  try {
    await request('DELETE', `/api/bindings/${b.id}`)
    message.success('已删除直播间')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '删除失败')
  } finally {
    await bindings.refresh()
  }
}

async function addBinding(acc: Account) {
  const roomId = (newRoomId[acc.id] ?? '').trim()
  if (!roomId) {
    message.warning('请输入房间号')
    return
  }
  try {
    await request('POST', '/api/bindings', { accountName: acc.name, roomId })
    newRoomId[acc.id] = ''
    message.success('已添加直播间')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '添加失败')
  } finally {
    // 加直播间之后必须刷新，否则用户加了直播间却在顶部选择器里找不到它。
    await bindings.refresh()
  }
}

// ---- 扫码弹窗：顶部「扫码添加账号」先填账号名；账号卡片的「重新扫码」
// 已经知道账号名，跳过填名字这一步 ----

const qrModalVisible = ref(false)
/** 空字符串表示还没确定账号名，要先让用户填；非空表示直接开始扫码。 */
const qrAccountName = ref('')
const qrNameInput = ref('')

function openAddAccountModal() {
  qrAccountName.value = ''
  qrNameInput.value = ''
  qrModalVisible.value = true
}

function openRescanModal(name: string) {
  qrAccountName.value = name
  qrModalVisible.value = true
}

function confirmAccountName() {
  const name = qrNameInput.value.trim()
  if (!name) {
    message.warning('请输入账号名')
    return
  }
  qrAccountName.value = name
}

function onQrSuccess(name: string) {
  qrModalVisible.value = false
  // 后端在扫码成功的同一次请求里已经同步做过一次立即检测（P5-2 任务
  // 2），不必再让用户等后台 10 分钟一轮的检测循环——重新拉一次账号列表
  // 就能拿到这次立即检测的结果。提示文案要如实说"已立即检测"，不能
  // 继续说成"下一轮才更新"，那样等于告诉用户这一步没做。
  message.success(`账号「${name}」登录成功，已立即检测一次登录状态`)
  void loadAccounts()
}

onMounted(() => void loadAccounts())
</script>

<template>
  <div class="accounts-page">
    <div class="page-header">
      <h2>账号与直播间</h2>
      <NButton type="primary" @click="openAddAccountModal">扫码添加账号</NButton>
    </div>

    <NSpin :show="loadingAccounts">
      <NEmpty v-if="ownedAccounts.length === 0" description="还没有账号，先扫码添加一个" />

      <NSpace vertical size="large" style="width: 100%">
        <NCard
          v-for="acc in ownedAccounts"
          :key="acc.id"
          class="account-card"
          :class="{ 'account-card--invalid': acc.loginState === 'invalid' }"
        >
          <template #header>
            <span>{{ acc.name }}</span>
            <span class="uid">UID {{ acc.uid }}</span>
          </template>
          <template #header-extra>
            <NTag :type="loginStateMeta(acc).tagType" size="small">
              {{ loginStateMeta(acc).text }}
            </NTag>
          </template>

          <!-- 检测详情常驻显示，不塞进悬浮提示——unknown 到底是「从未检测」
               还是「上次检测失败」，以及它不代表账号失效，这些话必须让人
               不用悬停就能看到，否则等于没说。 -->
          <div class="row login-detail">
            <span class="login-detail-text">{{ loginStateMeta(acc).detail }}</span>
          </div>

          <div class="row">
            <NButton
              size="small"
              :type="acc.loginState === 'invalid' ? 'error' : 'default'"
              @click="openRescanModal(acc.name)"
            >
              重新扫码
            </NButton>
          </div>

          <NDivider style="margin: 12px 0" />

          <div class="params-row">
            <span class="label">发送间隔（毫秒）</span>
            <NInputNumber
              v-if="drafts[acc.id]"
              v-model:value="drafts[acc.id]!.rateLimitMs"
              :min="0"
              style="width: 140px"
            />
            <span class="label">单条字数上限</span>
            <NInputNumber
              v-if="drafts[acc.id]"
              v-model:value="drafts[acc.id]!.maxLength"
              :min="1"
              :max="40"
              style="width: 100px"
            />
            <NButton
              size="small"
              type="primary"
              :disabled="!isDirty(acc)"
              @click="saveAccountParams(acc)"
            >
              保存参数
            </NButton>
            <NButton size="small" type="error" quaternary @click="confirmDeleteAccount(acc)">
              删除账号
            </NButton>
          </div>

          <NDivider style="margin: 12px 0" />

          <div class="bindings">
            <div v-for="b in bindingsByAccount.get(acc.id) ?? []" :key="b.id" class="binding-row">
              <div class="binding-row-main">
                <span class="room">房间 {{ b.roomId }}</span>
                <NTag class="binding-live-tag" :type="roomLiveStateMeta(b).tagType" size="small">
                  {{ roomLiveStateMeta(b).text }}
                </NTag>
                <NSwitch :value="b.enabled" @update:value="(v: boolean) => toggleBinding(b, v)" />
                <span class="rule-count">{{ b.ruleCount }} 条规则</span>
                <NButton size="small" text type="error" @click="confirmDeleteBinding(b)"
                  >删除</NButton
                >
              </div>
              <!-- 主播 UID + 昵称：现在能看出加的到底是谁的房间，不再只有一个
                   光秃秃的房间号——这正是加错房间号时用户想看到的第一手信息。
                   UID 明确写"主播 UID"，不写成"UID"，避免与上面账号卡片的
                   「UID {{ acc.uid }}」（账号自己的 UID）混淆，那是两个人。 -->
              <div class="row binding-anchor">
                <span class="anchor-text"
                  >主播 UID {{ anchorMeta(b).uid }} · {{ anchorMeta(b).name }}</span
                >
              </div>
              <div class="row binding-live-detail">
                <span class="binding-live-detail-text">{{ roomLiveStateMeta(b).detail }}</span>
              </div>
            </div>
            <NEmpty
              v-if="(bindingsByAccount.get(acc.id) ?? []).length === 0"
              description="这个账号还没绑定直播间"
              size="small"
            />

            <div class="add-binding-row">
              <NInput
                v-model:value="newRoomId[acc.id]"
                placeholder="房间号"
                style="width: 160px"
                @keyup.enter="addBinding(acc)"
              />
              <NButton size="small" @click="addBinding(acc)">加直播间</NButton>
            </div>
          </div>
        </NCard>
      </NSpace>
    </NSpin>

    <NDivider />

    <div class="granted-section">
      <h3>他人授权给我的直播间</h3>
      <p class="hint">
        这些直播间不归你的账号所有，是别人把权限授给了你，所以这里只读——启停与删除
        要求账号所有权，后端会拒绝（404）。
      </p>
      <NEmpty v-if="grantedBindings.length === 0" description="暂无" size="small" />
      <NSpace v-else vertical style="width: 100%">
        <div v-for="b in grantedBindings" :key="b.id" class="binding-row granted">
          <span class="account-name">{{ b.accountName }}</span>
          <span class="room">房间 {{ b.roomId }}</span>
          <NTag size="small" :type="b.enabled ? 'success' : 'default'">
            {{ b.enabled ? '已启用' : '已停用' }}
          </NTag>
          <span class="rule-count">{{ b.ruleCount }} 条规则</span>
        </div>
      </NSpace>
    </div>

    <NModal v-model:show="qrModalVisible">
      <NCard title="扫码登录" style="width: 360px" closable @close="qrModalVisible = false">
        <template v-if="!qrAccountName">
          <NSpace vertical>
            <NInput
              v-model:value="qrNameInput"
              placeholder="账号名（用于区分多个 B 站账号）"
              @keyup.enter="confirmAccountName"
            />
            <NButton type="primary" block @click="confirmAccountName">下一步</NButton>
          </NSpace>
        </template>
        <QRCodeLogin v-else :account-name="qrAccountName" @success="onQrSuccess" />
      </NCard>
    </NModal>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.account-card .uid {
  margin-left: 8px;
  font-size: 12px;
  opacity: 0.6;
}
/* 登录已失效的账号要在一堆卡片里跳出来，不能只靠一个小标签 */
.account-card--invalid {
  border: 1px solid var(--n-error-color, #d03050);
}
.row {
  display: flex;
  gap: 8px;
}
.login-detail-text {
  font-size: 12px;
  opacity: 0.7;
}
.params-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.label {
  font-size: 13px;
  opacity: 0.8;
}
.binding-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 0;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
}
.binding-row-main {
  display: flex;
  align-items: center;
  gap: 12px;
}
/* 「他人授权给我的直播间」一节用的是同一个 .binding-row 类名，但那边
   仍是单行展示（账号名+房间号+启停标签+规则数），不带主播身份/开播
   状态的详情行，所以要单独盖掉上面为"我拥有的账号"卡片改成的纵向布局。 */
.binding-row.granted {
  flex-direction: row;
  align-items: center;
  gap: 12px;
  border-bottom: none;
}
.binding-row.granted .account-name {
  font-weight: 600;
}
.binding-anchor .anchor-text,
.binding-live-detail-text {
  font-size: 12px;
  opacity: 0.7;
}
.add-binding-row {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
.granted-section .hint {
  font-size: 12px;
  opacity: 0.7;
  margin: 4px 0 12px;
}
</style>

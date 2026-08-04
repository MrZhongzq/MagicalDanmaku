<script setup lang="ts">
/**
 * Moderation 是「房管」页（设计文档 §7.2 页面 3），分两个区，且**彻底
 * 分开处理**（P5-6，用户明确纠正过两次）：
 *
 *   - 房管区：禁言、解除禁言、禁言名单。房间级操作，走 B 站直播间禁言
 *     接口，主播本人与粉丝房管都能用，对应权限点 user:block。
 *   - 拉黑区：拉黑、解除拉黑。**账号级操作，与直播间无关**——用户原话
 *     「拉黑是个账号操作，是指账号拉黑，直播间没有拉黑。主播在直播间
 *     拉黑一个人和她从评论区拉黑一个人没有区别」。
 *
 * **这不是「禁言的一个时长档位」。** 早期版本（以及原 C++ 项目的
 * `signalAddBlockUser(uid, 720, msg)`）把「拉黑」实现成「禁言 720
 * 小时」，用户纠正过两次——账号拉黑（`POST x/relation/modify`，
 * act=5）与直播间禁言是完全不同的两个 B 站接口，走完全不同的权限
 * 判定（见下）。
 *
 * 两区的权限判定是两条不同的轴：
 *
 *   - 房管区走 `user:block` 权限点——持有它的人（主播本人或被授权的
 *     房管）都能禁言/解禁。缺权限时只顶一条 PermissionWarning，
 *     **所有控件保持可用**：B 站可能刚给房管权限、刚撤，接口返回的
 *     状态可能滞后，把面板灰掉等于替用户预判一个可能是错的结论。
 *     操作失败时把后端 502 的原文原样显示，不包装成「操作失败，请
 *     重试」——那句话（例如「你不是本房间的房管」）正是操作者要看的。
 *   - 拉黑区走**账号所有权**（`binding.isOwner`，对应后端
 *     `isAccountOwner`），不是 `user:block`——房管持有 user:block
 *     能代为禁言，但不能代表账号本人去拉黑一个陌生人的社交关系。
 *     同样只警告不锁死，理由与上面一致。
 *
 * 拉黑是不可逆的对外操作（真的会影响 B 站账号的社交关系），下手前要有
 * 明确确认——用 `useDialog()`，与 Custom.vue 删规则草稿同一套模式。
 *
 * 状态回读：`GET .../blacklist-status` 是"白捡"的接口（`attribute==128`
 * 即已拉黑），让界面显示真实状态而不是"发了请求所以大概成功了"；
 * 顺带自动回填昵称。
 */
import { computed, h, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NDivider,
  NEmpty,
  NInput,
  NSelect,
  NSpin,
  NTag,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useRouter } from 'vue-router'
import { ApiError, request } from '@/api'
import type { BlacklistStatus, BlockedUser } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import PermissionWarning from '@/components/PermissionWarning.vue'
import BindingSelector from '@/components/BindingSelector.vue'

const auth = useAuthStore()
const bindings = useBindingsStore()
const message = useMessage()
const dialog = useDialog()
const router = useRouter()

/**
 * 缺 user:block 时的警告条。只在选中了直播间之后才判断——没有直播间时
 * 谈不上「缺权限」，会误导成「这个页面本身不能用」。
 */
const missingBlockPerm = computed(() => {
  const b = bindings.current
  return b !== null && !auth.hasPerm(b, 'user:block')
})

/**
 * 拉黑区的权限警告：账号所有权，不是 user:block。持有 user:block
 * （普通房管）的人会在这里看到警告——这正是"房管只能禁言"的界面表现。
 */
const missingOwnerPerm = computed(() => {
  const b = bindings.current
  return b !== null && !b.isOwner
})

// ---- 禁言名单 ----

const blockList = ref<BlockedUser[]>([])
const loadingList = ref(false)

async function loadBlockList() {
  const b = bindings.current
  if (!b) {
    blockList.value = []
    return
  }
  loadingList.value = true
  try {
    blockList.value = await request<BlockedUser[]>('GET', `/api/bindings/${b.id}/blocklist`)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载禁言名单失败')
  } finally {
    loadingList.value = false
  }
}

// 切换直播间之后名单要跟着换，不能停在上一个直播间的数据上
watch(
  () => bindings.currentId,
  () => void loadBlockList(),
  { immediate: true },
)

async function removeFromBlockList(item: BlockedUser) {
  const b = bindings.current
  if (!b) return
  try {
    await request('DELETE', `/api/bindings/${b.id}/blocklist/${encodeURIComponent(item.uid)}`)
    message.success('已从名单移除')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '移除失败')
  } finally {
    // 从名单里删一条不是破坏性操作（可以重加），不需要二次确认，
    // 但删完必须刷新，否则界面停在旧数据上、看起来像没删掉。
    await loadBlockList()
  }
}

const columns: DataTableColumns<BlockedUser> = [
  { title: 'UID', key: 'uid' },
  { title: '昵称', key: 'username' },
  { title: '原因', key: 'reason' },
  {
    title: '操作人',
    key: 'createdBy',
    render: (row) => (row.createdBy === null ? '-' : `用户 #${row.createdBy}`),
  },
  { title: '时间', key: 'createdAt' },
  {
    title: '操作',
    key: 'actions',
    render: (row) =>
      h(
        NButton,
        { size: 'small', text: true, type: 'error', onClick: () => void removeFromBlockList(row) },
        { default: () => '删除' },
      ),
  },
]

// ---- 加入名单 ----
//
// 只留 UID 输入框——昵称由后端按 UID 自动查询回填，理由框直接删掉
// （P5-6 用户原话：「这里不要禁言理由和昵称的输入，昵称自动 UID
// 获取，禁言理由无意义」）。

const addUid = ref('')

async function addToBlockList() {
  const b = bindings.current
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  const uid = addUid.value.trim()
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  try {
    await request('POST', `/api/bindings/${b.id}/blocklist`, { uid })
    message.success('已加入名单')
    addUid.value = ''
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加入名单失败')
  } finally {
    // 加完必须刷新，否则界面停在旧数据上，用户会以为没加成功
    await loadBlockList()
  }
}

// ---- 快捷禁言 / 解禁 ----

const HOUR_OPTIONS = [
  { label: '1 小时', value: 1 },
  { label: '24 小时', value: 24 },
  { label: '720 小时（30 天）', value: 720 },
]

const blockUid = ref('')
const blockHours = ref(1)

async function doBlock() {
  const b = bindings.current
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  const uid = blockUid.value.trim()
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  try {
    await request('POST', `/api/bindings/${b.id}/block`, { uid, hours: blockHours.value })
    message.success(`已禁言 UID ${uid}`)
    blockUid.value = ''
  } catch (e) {
    // 必须原样显示：后端对 B 站上游失败返回 502，文案形如「禁言失败: ...」，
    // 那句话（例如「你不是本房间的房管」）正是操作者需要看到的原因。
    // 包装成「操作失败，请重试」等于把唯一有用的信息删掉——重试没用。
    message.error(e instanceof ApiError ? e.message : '禁言失败')
  }
}

const unblockUid = ref('')

async function doUnblock() {
  const b = bindings.current
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  const uid = unblockUid.value.trim()
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  try {
    await request('POST', `/api/bindings/${b.id}/unblock`, { uid })
    message.success(`已解除 UID ${uid} 的禁言`)
    unblockUid.value = ''
  } catch (e) {
    // 同上：原样显示后端错误，不包装
    message.error(e instanceof ApiError ? e.message : '解除禁言失败')
  }
}

// ---- 拉黑区：账号级拉黑 / 解除拉黑（P5-6，独立于禁言的动作）----
//
// 走 x/relation/modify（act=5 拉黑 / act=6 取消拉黑），账号级，与直播间
// 无关。权限判定是 binding.isOwner（账号所有权），不是 user:block。

const blacklistUid = ref('')
const blacklistChecking = ref(false)
const blacklistStatus = ref<BlacklistStatus | null>(null)

/** checkBlacklistStatus 是"白捡"的状态回读——拉黑前后都可以查，
 * 让操作者在下手前后都能看到真实状态，而不是"发了请求所以大概成功了"。
 */
async function checkBlacklistStatus() {
  const b = bindings.current
  const uid = blacklistUid.value.trim()
  if (!b || !uid) {
    message.warning('请先选择直播间并填写 UID')
    return
  }
  blacklistChecking.value = true
  try {
    blacklistStatus.value = await request<BlacklistStatus>(
      'GET',
      `/api/bindings/${b.id}/blacklist-status?uid=${encodeURIComponent(uid)}`,
    )
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '查询拉黑状态失败')
  } finally {
    blacklistChecking.value = false
  }
}

async function doBlacklistRequest() {
  const b = bindings.current
  const uid = blacklistUid.value.trim()
  if (!b || !uid) return
  try {
    await request('POST', `/api/bindings/${b.id}/blacklist`, { uid })
    message.success(`已拉黑 UID ${uid}`)
    await checkBlacklistStatus()
  } catch (e) {
    // 拉黑失败的原因（例如"你不是这个账号的所有者"）正是操作者要看的，
    // 不能包装成笼统提示。
    message.error(e instanceof ApiError ? e.message : '拉黑失败')
  }
}

/** doBlacklist 下手前弹一道明确确认——拉黑是不可逆的对外操作，真的会
 * 影响 B 站账号的社交关系，不能一点就发。
 */
function doBlacklist() {
  const b = bindings.current
  const uid = blacklistUid.value.trim()
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  dialog.warning({
    title: '确认拉黑',
    content: `确定要拉黑 UID ${uid} 吗？这会真实影响该账号在 B 站的社交关系，且不是本地可撤销的操作（可以随时再次「解除拉黑」，但拉黑这个动作本身已经真实发生过）。`,
    positiveText: '确认拉黑',
    negativeText: '取消',
    onPositiveClick: () => void doBlacklistRequest(),
  })
}

async function doUnblacklist() {
  const b = bindings.current
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  const uid = blacklistUid.value.trim()
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  try {
    await request('POST', `/api/bindings/${b.id}/unblacklist`, { uid })
    message.success(`已解除 UID ${uid} 的拉黑`)
    await checkBlacklistStatus()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '解除拉黑失败')
  }
}

// 切直播间/改 UID 之后，上一次查到的状态就不再可信，必须清掉——
// 否则界面可能显示"绑定 A 里 UID X 已拉黑"，实际当前选中的是绑定 B。
watch([() => bindings.currentId, blacklistUid], () => {
  blacklistStatus.value = null
})

// ---- 自动规则：跳转到「自定义弹幕姬」页并预填一条草稿 ----
//
// Task 6 留下的松脱：当时 custom 路由还不存在，按钮跳不过去也看不出问题。
// custom 路由做出来之后（Task 11），按钮真能跳了，但「预填自动禁言模板」
// 那一半从未接过——按钮到得了地方，做不了它标签承诺的事。Task 14 把这半接上：
// 跳转时带 query（preset=automute），Custom.vue 读到后往草稿列表里插一条
// 「弹幕匹配关键词 → 禁言」的规则骨架（事件类型 danmaku、条件 text contains
// 关键词、动作 block），关键词留空由用户自己填——禁言关键词因人而异，
// 编不出默认值，硬编一个反而可能被误当成"已经配置好"直接保存。
//
// P5-6：新增 preset=autoblacklist，往草稿里插的是「弹幕匹配关键词 →
// 拉黑」（动作类型 blacklist）——与 automute 是两条**独立**的预设，
// 不共用同一条草稿骨架，呼应"拉黑规则与禁言规则要分开处理"的要求。
type CustomPreset = 'automute' | 'autoblacklist'

const presetLabel: Record<CustomPreset, string> = {
  automute: '自动禁言',
  autoblacklist: '自动拉黑',
}

function goToCustomDanmaku(preset: CustomPreset) {
  // 「自定义弹幕姬」现在已经注册路由，这个判断理论上总是为真；保留它是因为
  // router.push({name}) 找不到 name 时 vue-router 会同步抛
  // MATCHER_NOT_FOUND——万一将来路由表被改动导致 custom 临时缺席，这里
  // 也不会把同步异常炸到调用方（与 Shell.vue 里 go() 的处理一致）。
  if (!router.hasRoute('custom')) {
    message.info(`『自定义弹幕姬』页还没做，做好之后这里会跳过去配置${presetLabel[preset]}规则`)
    return
  }
  void router.push({ name: 'custom', query: { preset } })
}
</script>

<template>
  <div class="moderation-page">
    <div class="page-header">
      <h2>房管</h2>
      <!-- 房管页看不到也换不了当前是哪个绑定是真机反馈原话——操作明明打在
           某个绑定上，界面却没有任何提示。选择器要求 user:block 才不至于
           切过去就白操作一场，但只标注、不锁死，见组件头部注释。 -->
      <BindingSelector required-perm="user:block" />
    </div>

    <NEmpty v-if="!bindings.current" description="请先在顶部选择一个直播间" />

    <template v-else>
      <PermissionWarning
        v-if="missingBlockPerm"
        text="你在这个直播间没有 user:block 权限，操作会被拒绝"
      />

      <NCard title="房管区" class="section-card">
        <div class="row">
          <span class="label">快捷禁言</span>
          <NInput v-model:value="blockUid" placeholder="UID" style="width: 140px" />
          <NSelect v-model:value="blockHours" :options="HOUR_OPTIONS" style="width: 160px" />
          <NButton type="primary" @click="doBlock">禁言</NButton>
        </div>

        <div class="row">
          <span class="label">快捷解禁</span>
          <NInput v-model:value="unblockUid" placeholder="UID" style="width: 140px" />
          <NButton @click="doUnblock">解禁</NButton>
        </div>

        <NDivider style="margin: 12px 0" />

        <h4>禁言名单</h4>
        <NSpin :show="loadingList">
          <NDataTable :columns="columns" :data="blockList" :bordered="false" size="small" />
          <NEmpty v-if="blockList.length === 0" description="名单为空" size="small" />
        </NSpin>

        <div class="row add-row">
          <NInput v-model:value="addUid" placeholder="UID" style="width: 140px" />
          <NButton @click="addToBlockList">加入名单</NButton>
        </div>
      </NCard>

      <NCard class="section-card">
        <template #header>
          <span>拉黑区（账号级，与直播间无关）</span>
        </template>

        <PermissionWarning
          v-if="missingOwnerPerm"
          text="你不是这个账号的所有者，拉黑操作会被拒绝——房管只能禁言，拉黑只有账号所有者能做"
        />

        <p class="hint">
          拉黑是独立于禁言的账号动作：拉黑之后，这个账号在 B 站的黑名单里会真的多一个人，
          与直播间、与禁言完全无关。这是不可逆的对外操作，请在确认框里再次确认。
        </p>

        <div class="row">
          <span class="label">目标 UID</span>
          <NInput v-model:value="blacklistUid" placeholder="UID" style="width: 140px" />
          <NButton size="small" :loading="blacklistChecking" @click="checkBlacklistStatus">
            查询状态
          </NButton>
        </div>

        <p v-if="blacklistStatus" class="hint status-line">
          UID {{ blacklistStatus.uid }}
          <NTag v-if="blacklistStatus.nickname" size="small">{{ blacklistStatus.nickname }}</NTag>
          当前状态：
          <NTag :type="blacklistStatus.blacklisted ? 'error' : 'default'" size="small">
            {{ blacklistStatus.blacklisted ? '已拉黑' : '未拉黑' }}
          </NTag>
        </p>

        <div class="row">
          <NButton type="error" @click="doBlacklist">拉黑</NButton>
          <NButton @click="doUnblacklist">解除拉黑</NButton>
        </div>
      </NCard>

      <NCard class="section-card">
        <template #header>
          <span>自动禁言规则</span>
        </template>
        <p class="hint">
          自动禁言关键词、指定昵称自动禁言（支持通配符与正则）依赖规则引擎，在
          「自定义弹幕姬」页配置，不在这里。点「去配置」会跳过去并预填一条 「弹幕匹配关键词 →
          禁言」的规则草稿，关键词需要自己填，草稿仍要在那边 点「保存并生效」才会真正生效。
        </p>
        <NButton size="small" @click="goToCustomDanmaku('automute')">去配置</NButton>
      </NCard>

      <NCard class="section-card">
        <template #header>
          <span>自动拉黑规则</span>
        </template>
        <p class="hint">
          与「自动禁言规则」是两种**独立**的规则（P5-6：拉黑规则与禁言规则要分开处理），
          点「去配置」会跳过去并预填一条「弹幕匹配关键词 → 拉黑」的规则草稿，关键词需要
          自己填，草稿仍要在那边点「保存并生效」才会真正生效。自动拉黑一旦触发就是真实的
          账号级拉黑，配置关键词时请务必谨慎。
        </p>
        <NButton size="small" @click="goToCustomDanmaku('autoblacklist')">去配置</NButton>
      </NCard>
    </template>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
.section-card {
  margin-bottom: 16px;
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.label {
  font-size: 13px;
  opacity: 0.8;
  min-width: 64px;
}
.add-row {
  margin-top: 8px;
}
.hint {
  font-size: 13px;
  opacity: 0.8;
  margin: 0 0 8px;
}
</style>

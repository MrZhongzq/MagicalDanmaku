<script setup lang="ts">
/**
 * Moderation 是「房管」页（设计文档 §7.2 页面 3），分两个区：
 *
 *   - 房管区：禁言、解除禁言、禁言名单。主播本人与粉丝房管都能用，
 *     对应权限点 user:block。
 *   - 主播区：拉黑、解除拉黑。
 *
 * **「拉黑」不是独立的封禁动作，B 站没有独立的直播间拉黑接口**（P4-3
 * 查证结论，见悬空清单第 2 条）。原 C++ 项目的「拉黑」按钮实际调的是
 * `livedanmakuwindow.cpp:3049` 的 `signalAddBlockUser(uid, 720, msg)`——
 * 就是把禁言时长打满 720 小时（30 天，B 站最长禁言时长），走的是同一个
 * 禁言接口。所以这里「拉黑」直接接 `POST .../block`（`hours` 固定
 * 720），「解除拉黑」直接接 `POST .../unblock`，与房管区的禁言/解禁是
 * 同一个后端动作，只是时长固定、界面上分开摆放。**文案必须诚实**：
 * 不能让用户以为这是一个独立于禁言之外的功能。
 *
 * 全页最重要的一条，来自用户原话：
 *
 * > 主播账号本人和粉丝房管的区别在于主播可以禁言和拉黑，房管只能禁言，
 * > 此处如果发现没有房管权限应该提示警告，但是面板不应该全是灰的锁死。
 * > 如果有人非要在不是房管的直播间开启房管功能，b 站会回退操作失败，
 * > 把操作失败写日志。
 *
 * 原因：我们无法可靠预判某账号在某直播间到底有没有房管权限——B 站可能
 * 刚给、刚撤，或者接口返回的状态是滞后的。把面板灰掉等于「我判断你没
 * 权限所以不让你试」，而这个判断本身可能是错的。所以缺 user:block 时
 * 只顶一条 PermissionWarning，**所有控件保持可用**；操作失败时把后端
 * 返回的 error 原样显示——后端对 B 站上游失败返回 502，文案形如
 * 「禁言失败: ...」，那句话正是操作者要看的（例如「你不是本房间的房管」），
 * **不能包装成「操作失败，请重试」**，重试没用，原因才有用。
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
  NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useRouter } from 'vue-router'
import { ApiError, request } from '@/api'
import type { BlockedUser } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import PermissionWarning from '@/components/PermissionWarning.vue'
import BindingSelector from '@/components/BindingSelector.vue'

const auth = useAuthStore()
const bindings = useBindingsStore()
const message = useMessage()
const router = useRouter()

/**
 * 缺 user:block 时的警告条。只在选中了直播间之后才判断——没有直播间时
 * 谈不上「缺权限」，会误导成「这个页面本身不能用」。
 */
const missingBlockPerm = computed(() => {
  const b = bindings.current
  return b !== null && !auth.hasPerm(b, 'user:block')
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

const addUid = ref('')
const addUsername = ref('')
const addReason = ref('')

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
    await request('POST', `/api/bindings/${b.id}/blocklist`, {
      uid,
      username: addUsername.value.trim(),
      reason: addReason.value.trim(),
    })
    message.success('已加入名单')
    addUid.value = ''
    addUsername.value = ''
    addReason.value = ''
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

// ---- 主播区：拉黑 / 解除拉黑 ----
//
// 「拉黑」= 禁言 720 小时（B 站最长时限），走与房管区完全相同的接口，
// 只是时长固定。「保持不锁死」原则同样适用：非主播账号点这两个按钮
// 一样可用，B 站会在真正调用时回退操作失败，把原因原样透出。

/** 拉黑固定用 B 站允许的最长禁言时长——这就是「拉黑」在协议层的真实含义。 */
const BLACKLIST_HOURS = 720

const ownerBlockUid = ref('')
const ownerUnblockUid = ref('')

async function doOwnerBlock() {
  const b = bindings.current
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  const uid = ownerBlockUid.value.trim()
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  try {
    await request('POST', `/api/bindings/${b.id}/block`, { uid, hours: BLACKLIST_HOURS })
    message.success(`已拉黑 UID ${uid}（禁言 ${BLACKLIST_HOURS} 小时）`)
    ownerBlockUid.value = ''
  } catch (e) {
    // 与房管区的禁言一致：原样显示后端错误（例如「你不是本直播间的主播」），
    // 不包装成笼统提示——重试没用，原因才有用。
    message.error(e instanceof ApiError ? e.message : '拉黑失败')
  }
}

async function doOwnerUnblock() {
  const b = bindings.current
  if (!b) {
    message.warning('请先选择直播间')
    return
  }
  const uid = ownerUnblockUid.value.trim()
  if (!uid) {
    message.warning('请输入 UID')
    return
  }
  try {
    await request('POST', `/api/bindings/${b.id}/unblock`, { uid })
    message.success(`已解除 UID ${uid} 的拉黑`)
    ownerUnblockUid.value = ''
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '解除拉黑失败')
  }
}

// ---- 自动禁言规则：跳转到「自定义弹幕姬」页并预填一条草稿 ----
//
// Task 6 留下的松脱：当时 custom 路由还不存在，按钮跳不过去也看不出问题。
// custom 路由做出来之后（Task 11），按钮真能跳了，但「预填自动禁言模板」
// 那一半从未接过——按钮到得了地方，做不了它标签承诺的事。Task 14 把这半接上：
// 跳转时带 query（preset=automute），Custom.vue 读到后往草稿列表里插一条
// 「弹幕匹配关键词 → 禁言」的规则骨架（事件类型 danmaku、条件 text contains
// 关键词、动作 block），关键词留空由用户自己填——禁言关键词因人而异，
// 编不出默认值，硬编一个反而可能被误当成"已经配置好"直接保存。
function goToCustomDanmaku() {
  // 「自定义弹幕姬」现在已经注册路由，这个判断理论上总是为真；保留它是因为
  // router.push({name}) 找不到 name 时 vue-router 会同步抛
  // MATCHER_NOT_FOUND——万一将来路由表被改动导致 custom 临时缺席，这里
  // 也不会把同步异常炸到调用方（与 Shell.vue 里 go() 的处理一致）。
  if (!router.hasRoute('custom')) {
    message.info('『自定义弹幕姬』页还没做，做好之后这里会跳过去配置自动禁言规则')
    return
  }
  void router.push({ name: 'custom', query: { preset: 'automute' } })
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
          <NInput v-model:value="addUsername" placeholder="昵称（可选）" style="width: 140px" />
          <NInput v-model:value="addReason" placeholder="原因（可选）" style="width: 200px" />
          <NButton @click="addToBlockList">加入名单</NButton>
        </div>
      </NCard>

      <NCard class="section-card">
        <template #header>
          <span>主播区</span>
        </template>
        <template #header-extra>
          <NTooltip>
            <template #trigger>
              <NTag type="info" size="small">拉黑＝禁言到顶</NTag>
            </template>
            B 站没有独立的直播间拉黑接口——「拉黑」在这里是禁言 720 小时 （30 天，B
            站允许的最长禁言时长），走的是与房管区完全相同的接口， 不是另一个独立的封禁动作
          </NTooltip>
        </template>

        <p class="hint">
          拉黑（禁言 720 小时，B 站最长时限）——这不是一个独立的封禁动作，只是把
          禁言时长打满，与上面「房管区」的禁言/解禁走同一个接口
        </p>

        <div class="row">
          <span class="label">拉黑</span>
          <NInput v-model:value="ownerBlockUid" placeholder="UID" style="width: 140px" />
          <NButton type="primary" @click="doOwnerBlock">拉黑</NButton>
        </div>
        <div class="row">
          <span class="label">解除拉黑</span>
          <NInput v-model:value="ownerUnblockUid" placeholder="UID" style="width: 140px" />
          <NButton @click="doOwnerUnblock">解除拉黑</NButton>
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
        <NButton size="small" @click="goToCustomDanmaku">去配置</NButton>
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

<script setup lang="ts">
/**
 * MemberEditor 是「授权管理」区的实现（设计文档 §7.2，权限预设见 §7.3）。
 *
 * **权限点清单从 `GET /api/meta/permissions` 拉，不在这里硬编码。**
 * 硬编码就是第二处定义——后端权限点从七个改成六个（删掉了
 * account:manage）就是活生生的例子，硬编码的清单当时就已经漂了。
 * 这里唯一的真相来源是 permissionOptions，PRESETS 只是它的一个子集
 * 的一键勾选，不是另一份权限点定义。
 *
 * **PRESETS 展开成权限点数组，不是角色名。** 存储层（P3 的设计）只有
 * 权限点，没有角色，所以点预设按钮只是往当前勾选里并入一组权限点，
 * 提交给后端的永远是 permissions: Permission[]。预设之间、预设与手动
 * 勾选之间都是可叠加的，不是互斥单选——点了「运营」之后用户还能自己
 * 再勾别的，也能再点「房管」，两组会并在一起而不是互相替换。
 */
import { h, onMounted, ref } from 'vue'
import {
  NButton,
  NCheckbox,
  NCheckboxGroup,
  NDataTable,
  NEmpty,
  NInput,
  NSpin,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ApiError, request } from '@/api'
import type { Member, Permission } from '@/api'

const props = defineProps<{ bindingId: number }>()

const message = useMessage()
const dialog = useDialog()

/** 「值 + 中文说明」，与 GET /api/meta/permissions 的返回形状一致。 */
interface MetaItem {
  value: string
  label: string
}

/**
 * 权限预设（设计文档 §7.3）：按钮只是一键勾选一组复选框。
 *
 * 这里的每个值都必须是合法权限点——如果哪天后端又删/改了权限点，
 * TypeScript 的 Permission 联合类型会在编译期报错提醒改这里，
 * 而不是像纯字符串硬编码那样悄悄在运行期失效。
 */
const PRESETS: Record<string, Permission[]> = {
  运营: ['rule:read', 'rule:write', 'event:read'],
  房管: ['user:block', 'event:read'],
  观察: ['rule:read', 'event:read'],
}

const permissionOptions = ref<MetaItem[]>([])

async function loadPermissionOptions() {
  try {
    permissionOptions.value = await request<MetaItem[]>('GET', '/api/meta/permissions')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载权限点清单失败')
  }
}

const members = ref<Member[]>([])
const loadingMembers = ref(false)

async function loadMembers() {
  loadingMembers.value = true
  try {
    members.value = await request<Member[]>('GET', `/api/bindings/${props.bindingId}/members`)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载授权列表失败')
  } finally {
    loadingMembers.value = false
  }
}

onMounted(() => {
  void loadPermissionOptions()
  void loadMembers()
})

function labelOf(p: string): string {
  return permissionOptions.value.find((o) => o.value === p)?.label ?? p
}

// ---- 表单：新增授权 / 编辑已有成员共用一套 ----

/** null 表示「新增」；非 null 时是正在编辑的用户名，用户名输入框会禁用。 */
const editingUsername = ref<string | null>(null)
const usernameInput = ref('')
const selectedPermissions = ref<Permission[]>([])

function startNew() {
  editingUsername.value = null
  usernameInput.value = ''
  selectedPermissions.value = []
}

function startEdit(m: Member) {
  editingUsername.value = m.username
  usernameInput.value = m.username
  selectedPermissions.value = [...m.permissions]
}

/**
 * 点预设按钮：把这组权限点并入当前已勾选的，而不是替换。
 *
 * 用 Set 去重是因为重复点同一个预设、或几个预设有重叠权限点时，
 * 不该在 selectedPermissions 里留下重复项。
 */
function applyPreset(name: string) {
  const preset = PRESETS[name]
  if (!preset) return
  selectedPermissions.value = Array.from(new Set([...selectedPermissions.value, ...preset]))
}

async function submitGrant() {
  const username = (editingUsername.value ?? usernameInput.value).trim()
  if (!username) {
    message.warning('请输入用户名')
    return
  }
  if (selectedPermissions.value.length === 0) {
    // 后端对空列表的语义是「显式走 DELETE」，不接受用空数组当撤销的快捷方式，
    // 前端在这里先挡一道，省一次必然失败的往返
    message.warning('请至少选择一个权限点。撤销授权请用列表里的「撤销」按钮')
    return
  }
  try {
    await request<Member>(
      'PUT',
      `/api/bindings/${props.bindingId}/members/${encodeURIComponent(username)}`,
      { permissions: selectedPermissions.value },
    )
    message.success('已保存授权')
    startNew()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '保存授权失败')
  } finally {
    await loadMembers()
  }
}

function confirmRevoke(m: Member) {
  dialog.warning({
    title: '撤销授权',
    content: `确定要撤销用户「${m.username}」在这个绑定上的全部授权吗？`,
    positiveText: '撤销',
    negativeText: '取消',
    onPositiveClick: () => void revokeMember(m),
  })
}

async function revokeMember(m: Member) {
  try {
    await request(
      'DELETE',
      `/api/bindings/${props.bindingId}/members/${encodeURIComponent(m.username)}`,
    )
    message.success('已撤销授权')
    // 撤销的正是当前正在编辑的这一行时，表单要跟着复位，
    // 否则界面还停在一个已经不存在的成员上，保存会变成误建一条新授权
    if (editingUsername.value === m.username) startNew()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '撤销授权失败')
  } finally {
    await loadMembers()
  }
}

const columns: DataTableColumns<Member> = [
  { title: '用户名', key: 'username' },
  {
    title: '权限点',
    key: 'permissions',
    render: (row) => row.permissions.map((p) => labelOf(p)).join('、') || '-',
  },
  {
    title: '操作',
    key: 'actions',
    render: (row) =>
      h('div', { style: 'display: flex; gap: 8px' }, [
        h(
          NButton,
          { size: 'small', text: true, onClick: () => startEdit(row) },
          { default: () => '编辑' },
        ),
        h(
          NButton,
          { size: 'small', text: true, type: 'error', onClick: () => confirmRevoke(row) },
          { default: () => '撤销' },
        ),
      ]),
  },
]
</script>

<template>
  <div class="member-editor">
    <NSpin :show="loadingMembers">
      <NDataTable :columns="columns" :data="members" :bordered="false" size="small" />
      <NEmpty v-if="members.length === 0" description="这个绑定上还没有任何授权" size="small" />
    </NSpin>

    <div class="form">
      <h4>{{ editingUsername ? `编辑「${editingUsername}」的授权` : '新增授权' }}</h4>

      <div class="row">
        <span class="label">用户名</span>
        <NInput
          v-model:value="usernameInput"
          :disabled="editingUsername !== null"
          placeholder="被授权人的用户名"
          style="width: 200px"
        />
      </div>

      <div class="row presets">
        <span class="label">快捷预设</span>
        <NButton
          v-for="name in Object.keys(PRESETS)"
          :key="name"
          size="small"
          @click="applyPreset(name)"
        >
          {{ name }}
        </NButton>
        <span class="hint">点预设只是一键勾选，勾完还能自己再加减</span>
      </div>

      <div class="row">
        <span class="label">权限点</span>
        <NCheckboxGroup v-model:value="selectedPermissions">
          <NCheckbox
            v-for="opt in permissionOptions"
            :key="opt.value"
            :value="opt.value"
            :label="opt.label"
          />
        </NCheckboxGroup>
      </div>

      <div class="row">
        <NButton type="primary" @click="submitGrant">保存</NButton>
        <NButton v-if="editingUsername !== null" @click="startNew">取消编辑</NButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.form {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}
.label {
  font-size: 13px;
  opacity: 0.8;
  min-width: 64px;
}
.hint {
  font-size: 12px;
  opacity: 0.7;
}
</style>

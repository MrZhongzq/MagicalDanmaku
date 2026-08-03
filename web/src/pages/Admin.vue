<script setup lang="ts">
/**
 * Admin 是「管理」页（设计文档 §7.2 页面 6），分三块：
 *
 *   一、改自己的密码（所有人）
 *   二、用户管理（仅管理员）：列表、新建、删除、重置他人密码
 *   三、授权管理（对当前绑定有 member:manage 的人）：委托给 MemberEditor
 *
 * **改自己的密码一律要带旧密码，管理员也不例外。** 这不是前端随口定的
 * 规矩，是后端 handleChangePassword 里真实修过的一处安全缺陷：原本
 * `case caller.IsAdmin` 写在 `case caller.Username == target` 前面，
 * Go 的无标签 switch 取第一个为真的分支，导致管理员改自己的密码完全
 * 跳过旧密码校验——会话 Cookie 被劫持后可以永久接管账号。所以这里
 * 「改自己的密码」与「管理员重置他人密码」必须是两个不同的表单：
 * 前者旧密码必填，后者压根不出现旧密码这个字段（后端那个分支也不看它）。
 *
 * **账号所有者拿不到自己绑定上的 member:manage，这是刻意的**（P4 设计
 * 文档 §4.2：perm.OwnerBypass 排除了 MemberManage）。所有者已有的权力
 * 全是收缩性的（删账号、删绑定、连带清空全部授权），能清空别人的访问
 * 推不出能凭空赋予新人访问——把第三方拉进授权体系是管理员级别的决定。
 * 所以「我是账号所有者但打不开授权管理」是正常现象，界面文案必须说清
 * 「授权管理需要 member:manage 权限，请让管理员授予」，**不能**写成
 * 「你不是所有者」——所有者身份在这里根本不是判定标准，写错了会误导
 * 所有者去找账号转让而不是找管理员要权限。
 */
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NCheckbox,
  NEmpty,
  NInput,
  NModal,
  NTag,
  useDialog,
  useMessage,
} from 'naive-ui'
import { ApiError, request } from '@/api'
import type { User } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import PermissionWarning from '@/components/PermissionWarning.vue'
import MemberEditor from '@/components/MemberEditor.vue'
import BindingSelector from '@/components/BindingSelector.vue'

const auth = useAuthStore()
const bindings = useBindingsStore()
const message = useMessage()
/**
 * useDialog() 要求外层有 NDialogProvider——App.vue 里已经套好了。
 * 不在这里包 try/catch 退化成「拿不到就直接执行」：删用户是破坏性操作，
 * 缺 provider 是配置错误，应当响亮地抛异常，而不是悄悄跳过二次确认。
 * 与 Accounts.vue 的处理方式一致，测试里用 vi.mock 顶掉 useDialog。
 */
const dialog = useDialog()

// ---- 一、改自己的密码：旧密码必填 ----

const oldPassword = ref('')
const newPassword = ref('')

async function changeOwnPassword() {
  if (!auth.user) return
  if (!oldPassword.value) {
    message.warning('请输入旧密码')
    return
  }
  if (newPassword.value.length < 8) {
    message.warning('新密码至少 8 个字符')
    return
  }
  try {
    await request('POST', `/api/users/${encodeURIComponent(auth.user.username)}/password`, {
      oldPassword: oldPassword.value,
      newPassword: newPassword.value,
    })
    message.success('密码已修改，当前会话已失效，请重新登录')
    oldPassword.value = ''
    newPassword.value = ''
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '修改密码失败')
  }
}

// ---- 二、用户管理：仅管理员 ----

const users = ref<User[]>([])
const loadingUsers = ref(false)

async function loadUsers() {
  if (!auth.user?.isAdmin) return
  loadingUsers.value = true
  try {
    users.value = await request<User[]>('GET', '/api/users')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '加载用户列表失败')
  } finally {
    loadingUsers.value = false
  }
}

const newUsername = ref('')
const newUserPassword = ref('')
const newUserIsAdmin = ref(false)

async function createUser() {
  const username = newUsername.value.trim()
  if (!username) {
    message.warning('请输入用户名')
    return
  }
  if (newUserPassword.value.length < 8) {
    message.warning('密码至少 8 个字符')
    return
  }
  try {
    await request('POST', '/api/users', {
      username,
      password: newUserPassword.value,
      isAdmin: newUserIsAdmin.value,
    })
    message.success('用户已创建')
    newUsername.value = ''
    newUserPassword.value = ''
    newUserIsAdmin.value = false
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '创建用户失败')
  } finally {
    await loadUsers()
  }
}

/** 重置他人密码的弹窗状态。与上面「改自己的密码」是两个独立的表单——
 * 这里压根没有旧密码输入框，因为管理员重置他人密码不需要旧密码。 */
const resetTarget = ref<string | null>(null)
const resetPasswordValue = ref('')
const resetModalVisible = computed({
  get: () => resetTarget.value !== null,
  set: (v: boolean) => {
    if (!v) resetTarget.value = null
  },
})

function openResetPassword(u: User) {
  resetTarget.value = u.username
  resetPasswordValue.value = ''
}

async function submitResetPassword() {
  if (!resetTarget.value) return
  if (resetPasswordValue.value.length < 8) {
    message.warning('新密码至少 8 个字符')
    return
  }
  try {
    await request('POST', `/api/users/${encodeURIComponent(resetTarget.value)}/password`, {
      newPassword: resetPasswordValue.value,
    })
    message.success(`已重置用户「${resetTarget.value}」的密码`)
    resetTarget.value = null
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '重置密码失败')
  }
}

/** 删除用户是破坏性操作，必须二次确认——与 Accounts.vue 删账号同一套处理。 */
function confirmDeleteUser(u: User) {
  dialog.warning({
    title: '删除用户',
    content: `确定要删除用户「${u.username}」吗？此操作不可恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: () => void deleteUser(u),
  })
}

async function deleteUser(u: User) {
  try {
    await request('DELETE', `/api/users/${encodeURIComponent(u.username)}`)
    message.success('用户已删除')
  } catch (e) {
    // 原样显示后端错误：比如删除自己会得到 409「不能删除自己」，
    // 还拥有账号的用户会得到 409「还拥有 B 站账号，请先转移或删除这些账号」
    message.error(e instanceof ApiError ? e.message : '删除用户失败')
  } finally {
    await loadUsers()
  }
}

// ---- 三、授权管理：委托给 MemberEditor，这里只做权限门禁判断 ----

/** 缺 member:manage 时的提示语——见文件顶部注释，不能写成「你不是所有者」。 */
const MEMBER_MANAGE_HINT = '授权管理需要 member:manage 权限，请让管理员授予'

const canManageMembers = computed(() => {
  const b = bindings.current
  return b !== null && auth.hasPerm(b, 'member:manage')
})

onMounted(() => void loadUsers())
</script>

<template>
  <div class="admin-page">
    <h2>管理</h2>

    <NCard title="改我的密码" class="section-card">
      <div class="row">
        <span class="label">旧密码</span>
        <NInput
          v-model:value="oldPassword"
          type="password"
          show-password-on="click"
          placeholder="旧密码"
          style="width: 200px"
        />
      </div>
      <div class="row">
        <span class="label">新密码</span>
        <NInput
          v-model:value="newPassword"
          type="password"
          show-password-on="click"
          placeholder="至少 8 个字符"
          style="width: 200px"
        />
      </div>
      <NButton type="primary" @click="changeOwnPassword">修改密码</NButton>
    </NCard>

    <NCard v-if="auth.user?.isAdmin" title="用户管理" class="section-card">
      <div class="user-list">
        <div v-for="u in users" :key="u.id" class="user-row">
          <span class="username">{{ u.username }}</span>
          <NTag v-if="u.isAdmin" size="small" type="warning">管理员</NTag>
          <span class="created">{{ u.createdAt }}</span>
          <NButton size="small" text @click="openResetPassword(u)">重置密码</NButton>
          <NButton size="small" text type="error" @click="confirmDeleteUser(u)">删除</NButton>
        </div>
        <NEmpty v-if="!loadingUsers && users.length === 0" description="还没有用户" size="small" />
      </div>

      <div class="row new-user-row">
        <NInput v-model:value="newUsername" placeholder="用户名" style="width: 160px" />
        <NInput
          v-model:value="newUserPassword"
          type="password"
          show-password-on="click"
          placeholder="初始密码（至少 8 个字符）"
          style="width: 200px"
        />
        <NCheckbox v-model:checked="newUserIsAdmin">管理员</NCheckbox>
        <NButton type="primary" @click="createUser">创建用户</NButton>
      </div>
    </NCard>

    <!--
      管理页三块里只有「授权管理」是绑定维度的（改自己密码、用户管理都
      与具体绑定无关），选择器只放在这一块的卡片头上，不放在页面顶端——
      放在页面顶端会让人误以为「改我的密码」「用户管理」也受它影响。
    -->
    <NCard title="授权管理" class="section-card">
      <template #header-extra>
        <BindingSelector required-perm="member:manage" />
      </template>
      <NEmpty v-if="!bindings.current" description="请先在顶部选择一个直播间" />
      <PermissionWarning v-else-if="!canManageMembers" :text="MEMBER_MANAGE_HINT" />
      <MemberEditor v-else :key="bindings.current.id" :binding-id="bindings.current.id" />
    </NCard>

    <NModal v-model:show="resetModalVisible">
      <NCard
        :title="`重置「${resetTarget}」的密码`"
        style="width: 360px"
        closable
        @close="resetTarget = null"
      >
        <p class="hint">管理员重置他人密码不需要旧密码。</p>
        <NInput
          v-model:value="resetPasswordValue"
          type="password"
          show-password-on="click"
          placeholder="新密码（至少 8 个字符）"
          style="width: 100%; margin-bottom: 12px"
        />
        <NButton type="primary" block @click="submitResetPassword">确认重置</NButton>
      </NCard>
    </NModal>
  </div>
</template>

<style scoped>
.section-card {
  margin-bottom: 16px;
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
  min-width: 56px;
}
.user-list {
  margin-bottom: 12px;
}
.user-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 0;
}
.user-row .username {
  font-weight: 600;
  min-width: 100px;
}
.user-row .created {
  font-size: 12px;
  opacity: 0.7;
  margin-right: auto;
}
.new-user-row {
  margin-top: 8px;
}
.hint {
  font-size: 12px;
  opacity: 0.7;
  margin: 0 0 8px;
}
</style>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import {
  NLayout,
  NLayoutHeader,
  NLayoutSider,
  NMenu,
  NSelect,
  NButton,
  NSpin,
  useMessage,
} from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'

const auth = useAuthStore()
const bindings = useBindingsStore()
const route = useRoute()
const router = useRouter()
const message = useMessage()

const MENU = [
  { label: '账号与直播间', key: 'accounts' },
  { label: '房管', key: 'moderation' },
  { label: '弹幕姬', key: 'danmaku' },
  { label: '自定义弹幕姬', key: 'custom' },
  { label: '统计', key: 'stats' },
  { label: '日志', key: 'logs' },
  { label: '管理', key: 'admin' },
]

const bindingOptions = computed(() =>
  bindings.list.map((b) => ({
    label: `${b.accountName} @ ${b.roomId}${b.enabled ? '' : '（已停用）'}`,
    value: b.id,
  })),
)

onMounted(() => void bindings.refresh())

function go(key: string) {
  // 未实现的页面还没注册路由。按 name 推一个不存在的路由，vue-router 会
  // **同步抛** MATCHER_NOT_FOUND——通配符只匹配未解析的 path，不匹配未
  // 解析的 name，走不到兜底那一步。而 go 是从组件事件调的，Vue 在 dev
  // 模式下会把这个错误重新抛出去。
  //
  // 现在看不出症状（/accounts 是唯一的真路由，停在原地和跳回去长得一样），
  // 但 Task 5 一加第二个路由就会变成「点了没反应而控制台在报错」。
  if (!router.hasRoute(key)) {
    message.info('这个页面还没做')
    return
  }
  void router.push({ name: key })
}

/**
 * 退出登录后一定要跳回登录页，不管后端请求成不成功。
 *
 * auth.logout() 内部用 finally 保证本地状态一定清空，但请求本身在
 * 网络层失败时（不是 401，那条已经全局处理了）会把异常抛出来。不接住
 * 的话就是本地已经登出、却卡在原页面不跳转，还留一个未处理的 rejection
 * ——每个请求都会 401，用户却看不出发生了什么。
 */
function doLogout() {
  auth
    .logout()
    .catch(() => {})
    .finally(() => router.push('/login'))
}
</script>

<template>
  <NLayout has-sider position="absolute">
    <NLayoutSider bordered :width="180" content-style="padding-top: 12px">
      <NMenu :value="String(route.name)" :options="MENU" @update:value="go" />
    </NLayoutSider>
    <NLayout>
      <NLayoutHeader bordered class="header">
        <div class="left">
          <NSpin v-if="bindings.loading" size="small" />
          <NSelect
            v-else
            :value="bindings.currentId"
            :options="bindingOptions"
            placeholder="没有可用的直播间"
            style="width: 260px"
            @update:value="bindings.select"
          />
        </div>
        <div class="right">
          <span class="who">{{ auth.user?.username }}</span>
          <NButton text size="small" @click="doLogout"> 退出 </NButton>
        </div>
      </NLayoutHeader>
      <NLayout content-style="padding: 16px">
        <RouterView />
      </NLayout>
    </NLayout>
  </NLayout>
</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 16px;
}
.right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.who {
  font-size: 13px;
  opacity: 0.8;
}
</style>

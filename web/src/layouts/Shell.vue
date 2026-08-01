<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { NLayout, NLayoutHeader, NLayoutSider, NMenu, NSelect, NButton, NSpin } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'

const auth = useAuthStore()
const bindings = useBindingsStore()
const route = useRoute()
const router = useRouter()

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
  void router.push({ name: key })
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
          <NButton text size="small" @click="auth.logout().then(() => router.push('/login'))">
            退出
          </NButton>
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

<script setup lang="ts">
/**
 * QRCodeLogin 是扫码登录的弹窗内容：拿二维码 → 轮询状态 → 成功后通知父组件。
 *
 * 二维码怎么画：Naive UI 2.40 自带 NQrCode（已用
 * `node -e "require('naive-ui')"` 确认过），直接拿它渲染 B 站给的登录
 * 地址，不为了这一个字符串再装一个 qrcode 库。
 *
 * 用 `type="svg"` 而不是默认的 canvas：jsdom 不实现
 * HTMLCanvasElement.getContext，canvas 模式在测试环境里会报
 * "Not implemented" 的噪音；svg 模式两边都能画，产物也更小。
 */
import { onMounted, onUnmounted, ref } from 'vue'
import { NButton, NQrCode, NSpin } from 'naive-ui'
import { ApiError, request } from '@/api'

const props = defineProps<{
  /** 这次扫码要建号或换 Cookie 的账号名。 */
  accountName: string
}>()

const emit = defineEmits<{
  /** 扫码成功时把账号名报给父组件，父组件据此关弹窗、刷新账号列表。 */
  success: [accountName: string]
}>()

/**
 * 轮询间隔。真实值不重要——测试从不等待它，要么直接调 pollOnce，
 * 要么用 vi.useFakeTimers() 手动推进。
 */
const POLL_INTERVAL_MS = 2000

type Status = 'starting' | 'waiting' | 'scanned' | 'success' | 'expired' | 'error'

const status = ref<Status>('starting')
const qrUrl = ref('')
const qrKey = ref('')
const errorMessage = ref('')

/**
 * polling 是「轮询定时器是否还活着」的显式状态，与 timer 变量本身分开放：
 * timer 是普通变量，读它不会触发 Vue 的响应式追踪，用它算出来的 computed
 * 首次求值后就再也不会重新计算。测试要能确定性地断言「停止了」，
 * 所以单独放一个 ref。
 */
const polling = ref(false)

let timer: ReturnType<typeof setInterval> | null = null

function stopPolling() {
  if (timer !== null) {
    clearInterval(timer)
    timer = null
  }
  polling.value = false
}

/**
 * pollOnce 轮询一次扫码状态。**必须自己钉住两条硬要求**：
 *
 *   1. 拿到 success 就停止轮询并 emit 给父组件——不停的话，弹窗关掉
 *      之后组件卸载才会停，中间这段时间还在白白打后端。
 *   2. 拿到 404（扫码会话不存在或已过期）就停止轮询——后端的扫码会话
 *      TTL 只有 3 分钟，过期后每次轮询都是 404，不停下来的话会一直
 *      打后端，而用户看到的只是二维码一直转圈，不知道该干什么。
 *
 * 暴露成组件方法（见文件末尾 defineExpose）而不是纯内部函数，
 * 是为了让测试能绕开定时器直接、确定性地驱动它。
 */
async function pollOnce() {
  // polling 为 false 说明已经因为 success/expired/error 停过了。
  // 这道守卫不只是防御性代码：测试靠它验证「停止后再调用也不会真的
  // 发请求」，而不仅仅是状态字段变了。
  if (!polling.value || !qrKey.value) return
  try {
    const res = await request<{ status: string; account?: string }>(
      'POST',
      `/api/accounts/qrcode/${encodeURIComponent(qrKey.value)}`,
    )
    if (res.status === 'success') {
      stopPolling()
      status.value = 'success'
      emit('success', res.account ?? props.accountName)
      return
    }
    if (res.status === 'expired') {
      // 后端自己判断二维码过期（B 站那边的状态），与下面 404 分支
      // 是同一件事的两种表现形式，处理方式一致：停、提示重新发起。
      stopPolling()
      status.value = 'expired'
      return
    }
    if (res.status === 'waiting' || res.status === 'scanned') {
      status.value = res.status
    }
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      // 扫码会话的内存表按 TTL 过期，过期后 take() 找不到就是 404
      stopPolling()
      status.value = 'expired'
      errorMessage.value = e.message
      return
    }
    stopPolling()
    status.value = 'error'
    errorMessage.value = e instanceof ApiError ? e.message : '轮询扫码状态失败'
  }
}

async function start() {
  stopPolling()
  status.value = 'starting'
  errorMessage.value = ''
  try {
    const res = await request<{ key: string; url: string }>('POST', '/api/accounts/qrcode', {
      name: props.accountName,
    })
    qrKey.value = res.key
    qrUrl.value = res.url
    status.value = 'waiting'
    polling.value = true
    timer = setInterval(() => void pollOnce(), POLL_INTERVAL_MS)
  } catch (e) {
    status.value = 'error'
    errorMessage.value = e instanceof ApiError ? e.message : '发起扫码失败'
  }
}

/** 过期或出错之后用户点「重新发起」，走完整的开始流程，而不是接着轮询旧 key。 */
function restart() {
  void start()
}

onMounted(() => void start())
onUnmounted(stopPolling)

defineExpose({ pollOnce, restart, polling })
</script>

<template>
  <div class="qr-login">
    <NSpin :show="status === 'starting'">
      <div v-if="qrUrl" class="qr-box">
        <NQrCode :value="qrUrl" :size="200" type="svg" />
      </div>
      <div v-else class="qr-placeholder" />
    </NSpin>

    <p class="hint">
      <template v-if="status === 'starting'">正在生成二维码…</template>
      <template v-else-if="status === 'waiting'">请用手机 B 站客户端扫描二维码</template>
      <template v-else-if="status === 'scanned'">已扫描，请在手机上确认登录</template>
      <template v-else-if="status === 'success'">登录成功</template>
      <template v-else-if="status === 'expired'">二维码已过期，请重新发起</template>
      <template v-else-if="status === 'error'">{{ errorMessage || '出错了' }}</template>
    </p>

    <NButton v-if="status === 'expired' || status === 'error'" @click="restart">重新发起</NButton>
  </div>
</template>

<style scoped>
.qr-login {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}
.qr-box {
  padding: 8px;
  background: #fff;
  border-radius: 4px;
}
.qr-placeholder {
  width: 200px;
  height: 200px;
}
.hint {
  font-size: 13px;
  opacity: 0.85;
  margin: 0;
  text-align: center;
}
</style>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NCard, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import { ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const message = useMessage()

const username = ref('')
const password = ref('')
const submitting = ref(false)

async function submit() {
  if (!username.value || !password.value) {
    message.warning('请填写用户名与密码')
    return
  }
  submitting.value = true
  try {
    await auth.login(username.value, password.value)
    // 登录前想去哪就回哪，直接进来的回首页
    const to = router.currentRoute.value.query.redirect
    await router.replace(typeof to === 'string' ? to : '/accounts')
  } catch (e) {
    // 后端的错误文案本来就是写给操作者看的，原样显示
    message.error(e instanceof ApiError ? e.message : '登录失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <NCard title="神奇弹幕" class="login-card">
      <NForm @submit.prevent="submit">
        <NFormItem label="用户名">
          <NInput v-model:value="username" placeholder="用户名" @keyup.enter="submit" />
        </NFormItem>
        <NFormItem label="密码">
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            placeholder="密码"
            @keyup.enter="submit"
          />
        </NFormItem>
        <NButton type="primary" block :loading="submitting" @click="submit">登录</NButton>
      </NForm>
      <template #footer>
        <span class="hint"
          >首次使用的管理员密码由 <code>magicd migrate</code> 打印，只打印一次</span
        >
      </template>
    </NCard>
  </div>
</template>

<style scoped>
.login-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}
.login-card {
  width: 360px;
}
.hint {
  font-size: 12px;
  opacity: 0.7;
}
</style>

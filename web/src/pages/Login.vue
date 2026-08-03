<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NCard, NForm, NFormItem, NInput, NLayout, useMessage } from 'naive-ui'
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
  <!--
    全站的暗色背景不是靠某处写死的 CSS 颜色，是 App.vue 里 NConfigProvider
    的 darkTheme 主题变量，由 NLayout/NLayoutXxx 这类组件实际刷色——Shell.vue
    整个已登录区域外层就是一个 position:absolute 的 NLayout。登录页此前用的是
    裸 div，没有任何组件把背景刷成深色，浏览器默认的白色就透出来了，卡片却是
    深色组件，看起来像没做完。这里换成同样的 NLayout，背景色自动跟主题走，
    不必在这里手写一个可能与主题脱节的颜色值。
  -->
  <NLayout position="absolute" class="login-layout" content-style="height: 100%">
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
  </NLayout>
</template>

<style scoped>
.login-layout {
  height: 100%;
}
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

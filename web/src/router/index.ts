import { createRouter, createWebHistory } from 'vue-router'
import { setUnauthorizedHandler } from '@/api'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/pages/Login.vue') },
    {
      path: '/',
      component: () => import('@/layouts/Shell.vue'),
      children: [
        { path: '', redirect: '/accounts' },
        { path: 'accounts', name: 'accounts', component: () => import('@/pages/Accounts.vue') },
        {
          path: 'moderation',
          name: 'moderation',
          component: () => import('@/pages/Moderation.vue'),
        },
        { path: 'logs', name: 'logs', component: () => import('@/pages/Logs.vue') },
        { path: 'admin', name: 'admin', component: () => import('@/pages/Admin.vue') },
        // 其余页面在后续任务里逐个加进来
      ],
    },
    // 后端做了 SPA 回退（任何非 /api 路径都返回 index.html），
    // 所以这里必须自己兜住未知路径，否则用户看到的是空白页
    { path: '/:pathMatch(.*)*', redirect: '/accounts' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // 刷新页面后 store 是空的，先问一次后端「我是谁」。
  // Cookie 是 HttpOnly，前端读不到，这是唯一的恢复手段。
  if (auth.loading) {
    await auth.fetchMe()
  }

  if (to.name === 'login') {
    return auth.isLoggedIn ? { path: '/accounts' } : true
  }
  if (!auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

// 任何请求拿到 401 都把人送回登录页。会话可能在使用中过期，
// 不处理的话用户会对着一个每次操作都报错的界面不知所措。
setUnauthorizedHandler(() => {
  const auth = useAuthStore()
  auth.user = null
  void router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
})

export default router

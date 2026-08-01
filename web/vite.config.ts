import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    // 直接输出到 Go 的 embed 目录，不做中间同步。
    // 多一个同步步骤就多一处会忘记执行的地方，而忘记的表现是
    // 「改了前端却没生效」，很难查。
    outDir: fileURLToPath(new URL('../server/internal/webui/dist', import.meta.url)),
    emptyOutDir: true,
    // 产物要 embed 进二进制并提交进仓库，关掉 sourcemap 省体积
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      // 开发时前端跑在 5173，后端跑在 8080。
      // 必须带 cookie，会话是 HttpOnly Cookie。
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false,
      },
    },
  },
})

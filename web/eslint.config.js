// ESLint 9 默认只认 flat config，不再读 .eslintrc.cjs。
// 这里用 @vue/eslint-config-typescript 提供的 defineConfigWithVueTs 辅助函数
// 拼出等价于原先 .eslintrc.cjs 里 `plugin:vue/vue3-recommended` +
// `@vue/eslint-config-typescript` + `@vue/eslint-config-prettier` 的规则集。
import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import prettierConfig from '@vue/eslint-config-prettier'

export default defineConfigWithVueTs(
  { ignores: ['dist/**'] },
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,
  prettierConfig,
  {
    rules: {
      // 组件名允许单个单词：页面组件就叫 Login、Logs，强行凑两个词更难读
      'vue/multi-word-component-names': 'off',
    },
  },
)

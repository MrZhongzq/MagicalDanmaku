<script setup lang="ts">
/**
 * BindingSelector 是「账号 + 直播间」选择器的唯一实现。
 *
 * 用户原话：「除了首页账号和直播间之外，必须每个栏目，都是账号+直播间选择，
 * 不能泛泛」——房管/弹幕姬/自定义弹幕姬/统计/日志/管理(授权管理块) 六处
 * 都要能看到、也能换「现在改的是哪个绑定」。此前只有 `Shell.vue` 顶部导航栏
 * 渲染过这块 UI；页面正文里完全没有回显，长表单页（比如弹幕姬）往下滚动后
 * 顶部选择器整个不在视野里，操作者容易忘了自己正在改哪个绑定。这里把
 * `Shell.vue` 原本内联的那块选择器逻辑抽出来，`Shell.vue` 与各页面正文
 * 共用同一份实现，不留两套判断「加载中/出错/正常」的分支。
 *
 * `requiredPerm` 可选：传入后，会在候选项文案里标出「当前账号在这个绑定
 * 没有这个权限」，帮用户在切换之前就看出「切过去大概率也操作不了」。
 * **只是提示，不影响选项能不能选、更不会把整个选择器锁死**——用户在 P4
 * 就明确要求过「面板不应该全是灰的锁死」，`PermissionWarning.vue` 已经在
 * 负责选中绑定之后的权限提示，这里不重复造一套锁死逻辑，也不跟它抢戏。
 *
 * **P6 任务 1**：候选项文案在房间号后面追加主播昵称（`· 昵称`），用户
 * 原话「记房间号有点困难」。数据是现成的——P5-2 已经把主播 UID/昵称
 * 取回来存进 `Binding.anchorName`（账号与直播间页显示的「主播 UID xxx
 * · 桃酥Su--」用的就是这一份），这里直接读，不再调接口。昵称拿不到时
 * （探测失败、或这个绑定还没被心跳探测过）`anchorName` 是空串，此时
 * 整个 `· 昵称` 片段都不拼，仍然只显示房间号——不能显示成空或「未知」
 * 把房间号挤掉，房间号才是用户唯一确定能找到自己直播间的线索。
 */
import { computed } from 'vue'
import { NAlert, NButton, NSelect, NSpin } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useBindingsStore } from '@/stores/bindings'
import type { Permission } from '@/api'

const props = defineProps<{
  requiredPerm?: Permission
}>()

const auth = useAuthStore()
const bindings = useBindingsStore()

const options = computed(() =>
  bindings.list.map((b) => {
    const nickname = b.anchorName ? ` · ${b.anchorName}` : ''
    const base = `${b.accountName} @ ${b.roomId}${nickname}${b.enabled ? '' : '（已停用）'}`
    const lacksPerm = props.requiredPerm !== undefined && !auth.hasPerm(b, props.requiredPerm)
    return {
      label: lacksPerm ? `${base}（无 ${props.requiredPerm} 权限）` : base,
      value: b.id,
    }
  }),
)
</script>

<template>
  <div class="binding-selector">
    <NSpin v-if="bindings.loading" size="small" />
    <NSelect
      v-else
      :value="bindings.currentId"
      :options="options"
      placeholder="没有可用的直播间"
      style="width: 260px"
      @update:value="bindings.select"
    />

    <!--
      GET /api/bindings 非 401 失败时不能一条提示都不出现——不然选择器只会
      渲染 placeholder="没有可用的直播间"，与「这个账号确实没绑过直播间」
      在界面上完全无法区分。401 已经由全局 setUnauthorizedHandler 处理过，
      走到这里说明是别的错误，原样显示 + 提供重试。
    -->
    <NAlert
      v-if="bindings.loadError"
      type="error"
      title="加载直播间列表失败"
      class="binding-selector-error"
    >
      <div class="binding-selector-error-body">
        <span>{{ bindings.loadError }}</span>
        <NButton size="small" @click="bindings.refresh()">重试</NButton>
      </div>
    </NAlert>
  </div>
</template>

<style scoped>
.binding-selector {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.binding-selector-error {
  margin: 0;
}
.binding-selector-error-body {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>

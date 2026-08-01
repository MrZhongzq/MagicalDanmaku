<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'

/**
 * SaveBar 是右上角的保存按钮。
 *
 * 用户的要求是「设置被更改以后按下保存才生效」——所以改动只存在
 * 前端内存里，dirty 为真时按钮才亮。按下之后由各页自己负责
 * 「写库 → 调 reload」两步，本组件只管呈现。
 */
const props = defineProps<{
  dirty: boolean
  saving: boolean
}>()

const emit = defineEmits<{ save: [] }>()
</script>

<template>
  <div class="save-bar">
    <NTag v-if="props.dirty" type="warning" size="small">有未保存的改动</NTag>
    <NButton
      type="primary"
      size="small"
      :disabled="!props.dirty"
      :loading="props.saving"
      @click="emit('save')"
    >
      保存并生效
    </NButton>
  </div>
</template>

<style scoped>
.save-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>

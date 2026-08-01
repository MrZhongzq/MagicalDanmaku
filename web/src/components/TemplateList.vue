<script setup lang="ts">
import { NDynamicInput } from 'naive-ui'

/**
 * TemplateList 编辑一组弹幕模板字符串，对应 spec.Action.Template。
 *
 * 后端 `rules.Action.Validate()` 要求 danmaku 动作至少有一条模板，
 * 所以这里 `:min="1"`——用户删不到 0 条，保存时不会因为空模板列表
 * 被后端 422 拒绝（虽然本任务不负责保存，但草稿状态本身不该先天非法）。
 *
 * 用 naive-ui 的 `preset="input"` 而不是自己拼 #default 插槽：
 * 这组模板就是纯字符串列表，用不到自定义每项的编辑器。
 */
const props = defineProps<{
  modelValue: string[]
  placeholder?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [string[]] }>()

// NDynamicInput 的 @update:value 类型是泛型 `<T>(value: T[]) => void`
// （naive-ui 要兼容 preset="pair" 等非字符串场景），所以处理函数的参数
// 要能接受任意元素类型的数组才能赋值成功；这里用完之后立刻转回 string[]，
// 因为 preset="input" 时 naive-ui 内部实际产出的就是字符串数组。
function onUpdate(v: unknown[]) {
  emit('update:modelValue', v as string[])
}
</script>

<template>
  <NDynamicInput
    :value="props.modelValue"
    preset="input"
    :min="1"
    :placeholder="props.placeholder ?? '弹幕模板'"
    @update:value="onUpdate"
  />
</template>

<script setup lang="ts">
import { NAlert } from 'naive-ui'

/**
 * PermissionWarning 在缺权限时给一条警告，但**不锁面板**。
 *
 * 设计文档 §7.2 页面 3 写死了这条：我们无法可靠预判某账号在某直播间
 * 到底有没有房管权限——B 站可能刚给、刚撤，或接口返回的状态是滞后的。
 * 把面板灰掉等于「我判断你没权限所以不让你试」，而这个判断本身可能是错的。
 *
 * 正确做法是让他试，失败了把 B 站返回的原因如实显示出来。
 */
defineProps<{ text: string }>()
</script>

<template>
  <NAlert type="warning" :bordered="false" style="margin-bottom: 12px">
    {{ text }}
  </NAlert>
</template>

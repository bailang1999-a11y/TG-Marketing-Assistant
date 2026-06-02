<template></template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { api, type Task } from '../api/client'
import { useUiStore } from '../stores/ui'
import { taskStatusDisplay } from '../utils/taskDisplay'

const ui = useUiStore()
let timer: number | null = null
let busy = false

function isFinished(status: string) {
  return ['success', 'partial_success', 'failed', 'completed', 'finished', 'stopped', 'cancelled'].includes(String(status || '').toLowerCase())
}

function taskDoneMessage(task: Task) {
  const summary = task.summary || {}
  const parts = [
    typeof summary.total === 'number' ? `总数 ${summary.total}` : '',
    typeof summary.normal === 'number' ? `正常 ${summary.normal}` : '',
    typeof summary.failed === 'number' ? `失败 ${summary.failed}` : '',
    typeof summary.timeout === 'number' ? `超时 ${summary.timeout}` : ''
  ].filter(Boolean)
  return parts.length ? parts.join('，') : taskStatusDisplay(task.status)
}

async function pollTrackedTasks() {
  if (busy || !ui.trackedTasks.length || !localStorage.getItem('codex3_token')) return
  busy = true
  try {
    const tasks = await api.tasks({ limit: 200 })
    for (const tracked of [...ui.trackedTasks]) {
      const task = tasks.find((item) => item.id === tracked.id)
      if (!task) continue
      if (!isFinished(task.status)) continue
      ui.untrackTask(tracked.id)
      const failed = ['failed', 'stopped', 'cancelled'].includes(String(task.status).toLowerCase())
      ui.toast({
        title: `${tracked.title}已完成`,
        message: taskDoneMessage(task),
        tone: failed ? 'error' : task.status === 'partial_success' ? 'warning' : 'success',
        duration: 7000
      })
    }
  } catch {
    // Keep global task notifications quiet on transient navigation/auth failures.
  } finally {
    busy = false
  }
}

onMounted(() => {
  timer = window.setInterval(pollTrackedTasks, 30000) // 优化：4s -> 30s，降低数据库压力
  void pollTrackedTasks()
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>

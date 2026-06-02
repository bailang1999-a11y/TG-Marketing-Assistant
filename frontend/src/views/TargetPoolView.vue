<template>
  <div class="page-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">目标群组</h1>
        <p class="page-subtitle">批量导入 Telegram 目标链接，或直接把终端里的个人频道批量加入目标池，统一支持现有分组或新建分组。</p>
      </div>
      <div class="page-actions">
        <select v-model="membershipRefreshKind" class="min-h-10 rounded-lg px-3 text-sm">
          <option value="terminal">主账号池</option>
          <option value="listener">监听账号池</option>
          <option value="all">全部账号</option>
        </select>
        <GlassButton variant="secondary" :loading="refreshingMemberships" @click="refreshMemberships">刷新群内账号状态</GlassButton>
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <GlassCard v-if="activeRefreshTask" class="mb-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">群内账号状态刷新</h2>
          <p class="mt-1 text-sm text-steel">{{ taskStatusText(activeRefreshTask.status) }} · 更新 {{ formatDate(activeRefreshTask.updated_at) }}</p>
        </div>
        <span class="status-pill" :data-tone="taskStatusTone(activeRefreshTask.status)">{{ activeRefreshTask.progress || 0 }}%</span>
      </div>
      <div class="progress-track mt-4">
        <div class="progress-fill" :style="{ width: `${activeRefreshTask.progress || 0}%` }"></div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-4">
        <div v-for="item in refreshTaskSummaryCards" :key="item.label" class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-steel">{{ item.label }}</div>
          <div class="mt-1 text-lg font-black text-white">{{ item.value }}</div>
        </div>
      </div>
    </GlassCard>

    <GlassCard>
      <div class="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_360px]">
        <div>
          <div class="mb-3">
            <h2 class="font-bold">导入目标</h2>
            <p class="mt-1 text-sm text-steel">一行一个链接，例如 https://t.me/AIGOGGGG</p>
          </div>
          <textarea
            v-model="form.content"
            class="min-h-[23rem] w-full resize-y rounded-2xl p-3.5 text-sm leading-6 text-white"
            placeholder="https://t.me/AIGOGGGG&#10;https://t.me/AnotherTarget&#10;t.me/TargetName&#10;@PublicTarget"
          ></textarea>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <input ref="fileInput" class="hidden" type="file" accept=".txt,.csv,.list" @change="readFile" />
            <GlassButton variant="secondary" @click="fileInput?.click()">从文件读取</GlassButton>
            <GlassButton variant="primary" :disabled="!form.content.trim() || importing" :loading="importing" @click="importTargets">导入目标</GlassButton>
            <GlassButton variant="ghost" :disabled="!form.content.trim() || importing" @click="form.content = ''">清空文本</GlassButton>
          </div>
        </div>

        <div class="space-y-4">
          <div class="app-card p-4">
            <h3 class="font-bold">导入到分组</h3>
            <div class="mt-3 space-y-3">
              <label class="flex items-center gap-2 text-sm">
                <input v-model="importGroupMode" type="radio" value="existing" />
                <span>已有分组</span>
              </label>
              <select
                v-model="form.group_id"
                class="min-h-11 w-full rounded-lg px-3 text-sm"
                :disabled="importGroupMode !== 'existing'"
              >
                <option value="">不分组</option>
                <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
              </select>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="importGroupMode" type="radio" value="new" />
                <span>新建分组</span>
              </label>
              <input
                v-model="form.new_group_name"
                class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
                :disabled="importGroupMode !== 'new'"
                placeholder="输入新分组名称"
              />
            </div>
          </div>
          <div class="app-card p-4">
            <h3 class="font-bold">从终端批量加入</h3>
            <p class="mt-2 text-sm leading-6 text-steel">读取终端资料里的个人频道批量加入目标池，没有个人频道的终端会在结果里提示失败。</p>
            <div class="mt-4 grid gap-3">
              <label class="flex items-center gap-2 text-sm">
                <input v-model="terminalImport.scope" type="radio" value="all" />
                <span>全部终端</span>
              </label>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="terminalImport.scope" type="radio" value="group" />
                <span>终端组</span>
              </label>
              <select
                v-model="terminalImport.terminal_group_id"
                class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
                :disabled="terminalImport.scope !== 'group'"
              >
                <option value="">选择终端组</option>
                <option v-for="group in terminalGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
              </select>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="terminalImport.scope" type="radio" value="terminal" />
                <span>单个终端</span>
              </label>
              <select
                v-model="terminalImport.terminal_id"
                class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
                :disabled="terminalImport.scope !== 'terminal'"
              >
                <option value="">选择终端</option>
                <option v-for="terminal in sourceTerminals" :key="terminal.id" :value="terminal.id">{{ terminalLabel(terminal) }}</option>
              </select>
            </div>
            <div class="mt-4 rounded-2xl border border-white/10 bg-white/5 px-3 py-3 text-sm text-steel">
              <div>命中终端：{{ selectedTerminalSources.length }}</div>
              <div class="mt-1">已设置个人频道：{{ readyTerminalSources.length }}</div>
            </div>
            <div class="mt-4">
              <GlassButton variant="primary" class="w-full" :loading="importing" :disabled="!canImportTerminals" @click="importFromTerminals">
                终端批量加入目标池
              </GlassButton>
            </div>
          </div>
          <div class="app-card p-4 text-sm leading-6 text-steel">
            <div class="font-bold text-white">支持格式</div>
            <p class="mt-2">支持 `https://t.me/AIGOGGGG`、`t.me/AIGOGGGG`、`@AIGOGGGG`，每行导入一个目标。</p>
          </div>
        </div>
      </div>
    </GlassCard>

    <GlassCard v-if="summary">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">导入结果</h2>
          <p class="mt-1 text-sm text-steel">目标分组：{{ summary.group_name || '不分组' }}</p>
        </div>
        <span class="status-pill" data-tone="success">成功 {{ summary.success }}</span>
      </div>
      <div class="grid gap-3 sm:grid-cols-4">
        <div v-for="card in resultCards" :key="card.label" class="metric-card app-card p-4" :data-tone="card.accent">
          <div class="text-sm text-steel">{{ card.label }}</div>
          <div class="mt-2 text-2xl font-black" :class="card.tone">{{ card.value }}</div>
        </div>
      </div>
      <div class="mt-5 max-h-80 overflow-auto rounded-2xl border border-white/10 scrollbar-thin">
        <div v-for="item in summary.items" :key="`${item.line}-${item.status}`" class="grid grid-cols-[1fr_140px_100px_1fr] gap-3 border-b border-white/8 px-3 py-2 text-sm">
          <span class="min-w-0 truncate">{{ item.line }}</span>
          <span class="min-w-0 truncate">{{ item.identifier || '-' }}</span>
          <span :class="statusClass(item.status)">{{ item.status }}</span>
          <span class="min-w-0 truncate text-steel">{{ item.reason || typeLabel(item.type || '') }}</span>
        </div>
      </div>
    </GlassCard>

    <GlassCard class="mb-4">
      <div class="mb-5">
        <h2 class="font-bold">自动批量加群 (终端加入目标池)</h2>
        <p class="mt-1 text-sm text-steel">配置并派发自动加群任务（JoinChannel），任务会在后台异步执行。</p>
      </div>
      <div class="grid gap-6 md:grid-cols-2">
        <div class="app-card space-y-3 p-4">
          <div class="font-bold">1. 选择终端范围 (谁去加群)</div>
          <div class="grid gap-3 pt-2">
            <label class="flex items-center gap-2 text-sm">
              <input v-model="joinTargets.terminal_scope" type="radio" value="all" />
              <span>全部终端</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="joinTargets.terminal_scope" type="radio" value="group" />
              <span>终端组</span>
            </label>
            <select
              v-model="joinTargets.terminal_group_id"
              class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
              :disabled="joinTargets.terminal_scope !== 'group'"
            >
              <option value="">选择终端组</option>
              <option v-for="group in terminalGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="joinTargets.terminal_scope" type="radio" value="terminal" />
              <span>单个终端</span>
            </label>
            <select
              v-model="joinTargets.terminal_id"
              class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
              :disabled="joinTargets.terminal_scope !== 'terminal'"
            >
              <option value="">选择终端</option>
              <option v-for="terminal in sourceTerminals" :key="terminal.id" :value="terminal.id">{{ terminalLabel(terminal) }}</option>
            </select>
          </div>
        </div>
        <div class="app-card flex flex-col p-4">
          <div class="font-bold">2. 选择目标范围 (加入哪些群组)</div>
          <div class="grid gap-3 pt-2 flex-col">
            <label class="flex items-center gap-2 text-sm">
              <input v-model="joinTargets.target_scope" type="radio" value="all" />
              <span>全部目标池群组</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="joinTargets.target_scope" type="radio" value="group" />
              <span>指定目标池分组</span>
            </label>
            <select
              v-model="joinTargets.target_group_id"
              class="min-h-11 w-full rounded-lg px-3 text-sm disabled:opacity-50"
              :disabled="joinTargets.target_scope !== 'group'"
            >
              <option value="">选择目标分组</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
            <div class="mt-auto pt-6">
              <GlassButton variant="primary" class="w-full bg-indigo-500/20 text-indigo-200 border-indigo-500/30 hover:bg-indigo-500/30" :loading="joining" :disabled="!canJoinTargets" @click="createJoinTargetsTask">
                一键派发加群任务 (Join Targets)
              </GlassButton>
            </div>
          </div>
        </div>
      </div>
    </GlassCard>

    <GlassCard class="flex-1">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-3">
          <GroupSelect v-model="filterGroupID" :groups="groups" :loading="groupLoading" @create="createGroup" />
          <input v-model="targetKeyword" class="min-h-10 rounded-lg px-3 text-sm" placeholder="搜索目标名称或链接" />
        </div>
        <div class="flex flex-wrap gap-2">
          <span class="status-pill" data-tone="info">共 {{ filteredTargets.length }} 个目标</span>
          <span class="status-pill" data-tone="success">有效 {{ targetHealth.active }}</span>
          <span class="status-pill" :data-tone="targetHealth.invalid > 0 ? 'warning' : 'info'">失效 {{ targetHealth.invalid }}</span>
          <span class="status-pill" :data-tone="targetHealth.unchecked > 0 ? 'warning' : 'info'">未校验 {{ targetHealth.unchecked }}</span>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[1280px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="py-2">标识符</th>
              <th>名称</th>
              <th>类型</th>
              <th>规模</th>
              <th>所属分组</th>
              <th>历史通知</th>
              <th>有效账号</th>
              <th>失效账号</th>
              <th>最近校验</th>
              <th>验证门槛</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="target in pagedTargets" :key="target.id" class="border-t border-white/8 align-top">
              <td class="py-3 font-semibold">{{ target.identifier }}</td>
              <td>{{ target.name || '-' }}</td>
              <td>{{ typeLabel(target.type) }}</td>
              <td>{{ target.size || 0 }}</td>
              <td>
                <div class="flex flex-wrap gap-2">
                  <span
                    v-for="label in groupNames(target)"
                    :key="`${target.id}-${label}`"
                    class="status-pill"
                    data-tone="info"
                  >
                    {{ label }}
                  </span>
                </div>
              </td>
              <td>{{ target.notification_count || 0 }}</td>
              <td>
                <button class="font-semibold text-neon hover:underline" @click="openMemberships(target)">
                  {{ target.active_member_count ?? target.linked_terminals ?? 0 }}
                </button>
              </td>
              <td>
                <span class="status-pill" :data-tone="(target.invalid_member_count || 0) > 0 ? 'warning' : 'info'">
                  {{ target.invalid_member_count || 0 }}
                </span>
              </td>
              <td class="text-steel">{{ target.last_membership_check_at ? formatDate(target.last_membership_check_at) : '未校验' }}</td>
              <td>
                <span class="status-pill" :data-tone="target.has_verification ? 'warning' : 'info'">{{ target.has_verification ? '需要' : '无' }}</span>
              </td>
              <td class="text-steel">{{ formatDate(target.created_at) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="!filteredTargets.length" class="py-8 text-center text-sm text-steel">暂无目标</div>
      </div>
      <div v-if="targetPageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
        <span>第 {{ targetPage }} / {{ targetPageCount }} 页</span>
        <GlassButton variant="secondary" size="sm" :disabled="targetPage <= 1" @click="targetPage--">上一页</GlassButton>
        <GlassButton variant="secondary" size="sm" :disabled="targetPage >= targetPageCount" @click="targetPage++">下一页</GlassButton>
      </div>
    </GlassCard>

    <GlassCard v-if="selectedMembershipTarget">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">群内账号状态</h2>
          <p class="mt-1 text-sm text-steel">{{ selectedMembershipTarget.name || selectedMembershipTarget.identifier }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <select v-model="membershipAccountKind" class="min-h-10 rounded-lg px-3 text-sm">
            <option value="terminal">主账号</option>
            <option value="listener">监听账号</option>
            <option value="all">全部账号</option>
          </select>
          <select v-model="membershipStatusFilter" class="min-h-10 rounded-lg px-3 text-sm">
            <option value="all">全部状态</option>
            <option value="active">仍在群内</option>
            <option value="invalid">失效账号</option>
          </select>
          <GlassButton variant="secondary" :loading="membershipLoading" @click="loadMemberships()">刷新列表</GlassButton>
          <GlassButton variant="secondary" :loading="refreshingMemberships" @click="refreshSelectedMemberships">实时校验</GlassButton>
          <GlassButton variant="ghost" @click="closeMemberships">关闭</GlassButton>
        </div>
      </div>
      <div class="mb-4 flex flex-wrap gap-2 text-sm">
        <span class="status-pill" data-tone="info">记录 {{ filteredMemberships.length }}</span>
        <span class="status-pill" data-tone="success">有效 {{ membershipHealth.active }}</span>
        <span class="status-pill" :data-tone="membershipHealth.invalid > 0 ? 'warning' : 'info'">失效 {{ membershipHealth.invalid }}</span>
      </div>
      <div class="overflow-x-auto rounded-2xl border border-white/10">
        <table class="w-full min-w-[920px] text-left text-sm">
          <thead class="text-steel">
            <tr>
              <th class="px-3 py-2">账号</th>
              <th>账号状态</th>
              <th>群内状态</th>
              <th>原因</th>
              <th>上次校验</th>
              <th>最后在群</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in visibleMemberships" :key="item.id" class="border-t border-white/8">
              <td class="px-3 py-3">
                <div class="font-semibold text-white">{{ item.account_label || item.phone || item.account_id }}</div>
                <div class="mt-1 text-xs text-steel">{{ item.nickname || accountKindLabel(item.account_kind) }}</div>
              </td>
              <td>{{ item.account_status || '-' }}<span v-if="item.risk_status" class="text-steel"> / {{ item.risk_status }}</span></td>
              <td>
                <span class="status-pill" :data-tone="item.active ? 'success' : 'warning'">{{ item.status_text || item.status }}</span>
              </td>
              <td class="max-w-[260px] truncate text-steel">{{ item.status_reason || '-' }}</td>
              <td class="text-steel">{{ item.last_checked_at ? formatDate(item.last_checked_at) : '未校验' }}</td>
              <td class="text-steel">{{ item.last_seen_at ? formatDate(item.last_seen_at) : '-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="membershipLoading" class="py-8 text-center text-sm text-steel">正在读取群内账号状态…</div>
        <div v-else-if="!filteredMemberships.length" class="py-8 text-center text-sm text-steel">当前筛选下没有账号加入记录。</div>
        <div v-else-if="filteredMemberships.length > visibleMemberships.length" class="border-t border-white/8 py-3 text-center text-xs text-steel">
          已显示前 {{ visibleMemberships.length }} 条，可缩小筛选范围继续查看。
        </div>
      </div>
    </GlassCard>

    <p v-if="error" class="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import GroupSelect from '../components/GroupSelect.vue'
import { api, type Group, type Target, type TargetImportSummary, type TargetMembership, type Terminal, type Task } from '../api/client'
import { useUiStore } from '../stores/ui'

const targets = ref<Target[]>([])
const groups = ref<Group[]>([])
const terminalGroups = ref<Group[]>([])
const sourceTerminals = ref<Terminal[]>([])
const summary = ref<TargetImportSummary | null>(null)
const ui = useUiStore()
const filterGroupID = ref('')
const targetKeyword = ref('')
const targetPage = ref(1)
const targetPageSize = 100
const importGroupMode = ref<'existing' | 'new'>('existing')
const loading = ref(false)
const importing = ref(false)
const joining = ref(false)
const refreshingMemberships = ref(false)
const membershipLoading = ref(false)
const groupLoading = ref(false)
const error = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const selectedMembershipTarget = ref<Target | null>(null)
const memberships = ref<TargetMembership[]>([])
const membershipRefreshKind = ref<'terminal' | 'listener' | 'all'>('terminal')
const membershipAccountKind = ref<'terminal' | 'listener' | 'all'>('terminal')
const membershipStatusFilter = ref<'all' | 'active' | 'invalid'>('all')
const activeRefreshTask = ref<Task | null>(null)
let refreshTaskTimer: ReturnType<typeof window.setInterval> | null = null
const form = reactive({
  content: '',
  group_id: '',
  new_group_name: ''
})
const terminalImport = reactive<{
  scope: 'all' | 'group' | 'terminal'
  terminal_group_id: string
  terminal_id: string
}>({
  scope: 'all',
  terminal_group_id: '',
  terminal_id: ''
})

const joinTargets = reactive<{
  terminal_scope: 'all' | 'group' | 'terminal'
  terminal_group_id: string
  terminal_id: string
  target_scope: 'all' | 'group'
  target_group_id: string
}>({
  terminal_scope: 'all',
  terminal_group_id: '',
  terminal_id: '',
  target_scope: 'all',
  target_group_id: ''
})

const groupMap = computed(() => new Map(groups.value.map((group) => [group.id, group.name])))
const filteredTargets = computed(() => {
  const keyword = targetKeyword.value.trim().toLowerCase()
  if (!keyword) return targets.value
  return targets.value.filter((target) => {
    return [target.identifier, target.name, typeLabel(target.type)].some((value) => String(value || '').toLowerCase().includes(keyword))
  })
})
const targetPageCount = computed(() => Math.max(1, Math.ceil(filteredTargets.value.length / targetPageSize)))
const pagedTargets = computed(() => {
  const start = (targetPage.value - 1) * targetPageSize
  return filteredTargets.value.slice(start, start + targetPageSize)
})
const targetHealth = computed(() => {
  return filteredTargets.value.reduce(
    (acc, target) => {
      acc.active += Number(target.active_member_count ?? target.linked_terminals ?? 0)
      acc.invalid += Number(target.invalid_member_count || 0)
      if (!target.last_membership_check_at) acc.unchecked++
      return acc
    },
    { active: 0, invalid: 0, unchecked: 0 }
  )
})
const filteredMemberships = computed(() => {
  if (membershipStatusFilter.value === 'active') return memberships.value.filter((item) => item.active)
  if (membershipStatusFilter.value === 'invalid') return memberships.value.filter((item) => !item.active)
  return memberships.value
})
const visibleMemberships = computed(() => filteredMemberships.value.slice(0, 500))
const membershipHealth = computed(() => {
  return memberships.value.reduce(
    (acc, item) => {
      if (item.active) acc.active++
      else acc.invalid++
      return acc
    },
    { active: 0, invalid: 0 }
  )
})
const refreshTaskSummaryCards = computed(() => {
  const summary = activeRefreshTask.value?.summary || {}
  return [
    { label: '总记录', value: numericSummary(summary, 'total') },
    { label: '仍有效', value: numericSummary(summary, 'active') },
    { label: '已移除', value: numericSummary(summary, 'removed') },
    { label: '待复查', value: numericSummary(summary, 'skipped') + numericSummary(summary, 'failed') }
  ]
})
const selectedTerminalSources = computed(() => {
  if (terminalImport.scope === 'all') return sourceTerminals.value
  if (terminalImport.scope === 'group') return sourceTerminals.value.filter((terminal) => terminal.group_id === terminalImport.terminal_group_id)
  if (terminalImport.scope === 'terminal') return sourceTerminals.value.filter((terminal) => terminal.id === terminalImport.terminal_id)
  return []
})
const readyTerminalSources = computed(() => selectedTerminalSources.value.filter((terminal) => !!terminal.homepage?.trim()))
const canImportTerminals = computed(() => {
  if (importing.value) return false
  if (terminalImport.scope === 'group' && !terminalImport.terminal_group_id) return false
  if (terminalImport.scope === 'terminal' && !terminalImport.terminal_id) return false
  return selectedTerminalSources.value.length > 0
})

const canJoinTargets = computed(() => {
  if (joining.value) return false
  if (joinTargets.terminal_scope === 'group' && !joinTargets.terminal_group_id) return false
  if (joinTargets.terminal_scope === 'terminal' && !joinTargets.terminal_id) return false
  if (joinTargets.target_scope === 'group' && !joinTargets.target_group_id) return false
  return true
})

const resultCards = computed(() => {
  const data = summary.value
  if (!data) return []
  return [
    { label: '成功', value: data.success, tone: 'text-neon', accent: 'success' },
    { label: '失败', value: data.failed, tone: 'text-danger', accent: 'danger' },
    { label: '重复', value: data.duplicate, tone: 'text-amber', accent: 'warning' },
    { label: '跳过', value: data.skipped, tone: 'text-steel', accent: 'info' }
  ]
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [groupData, targetData, terminalGroupData, terminalData] = await Promise.all([
      api.groups('target'),
      api.targets(filterGroupID.value),
      api.groups('terminal'),
      api.terminals()
    ])
    groups.value = groupData
    targets.value = targetData
    terminalGroups.value = terminalGroupData
    sourceTerminals.value = terminalData
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取目标池失败'
  } finally {
    loading.value = false
  }
}

async function createGroup(name: string) {
  groupLoading.value = true
  error.value = ''
  try {
    await api.createGroup('target', name)
    groups.value = await api.groups('target')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '创建分组失败'
  } finally {
    groupLoading.value = false
  }
}

async function importTargets() {
  importing.value = true
  error.value = ''
  summary.value = null
  try {
    const payload = {
      content: form.content,
      group_id: importGroupMode.value === 'existing' ? form.group_id : '',
      new_group_name: importGroupMode.value === 'new' ? form.new_group_name.trim() : ''
    }
    summary.value = await api.importTargets(payload)
    form.content = ''
    form.new_group_name = ''
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导入目标失败'
  } finally {
    importing.value = false
  }
}

async function importFromTerminals() {
  importing.value = true
  error.value = ''
  summary.value = null
  try {
    summary.value = await api.importTerminalTargets({
      scope: terminalImport.scope,
      terminal_group_id: terminalImport.scope === 'group' ? terminalImport.terminal_group_id : '',
      terminal_id: terminalImport.scope === 'terminal' ? terminalImport.terminal_id : '',
      group_id: importGroupMode.value === 'existing' ? form.group_id : '',
      new_group_name: importGroupMode.value === 'new' ? form.new_group_name.trim() : ''
    })
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '从终端加入目标池失败'
  } finally {
    importing.value = false
  }
}

async function createJoinTargetsTask() {
  joining.value = true
  error.value = ''
  try {
    const res = await api.joinTargets({
      terminal_scope: joinTargets.terminal_scope,
      terminal_group_id: joinTargets.terminal_scope === 'group' ? joinTargets.terminal_group_id : '',
      terminal_id: joinTargets.terminal_scope === 'terminal' ? joinTargets.terminal_id : '',
      target_scope: joinTargets.target_scope,
      target_group_id: joinTargets.target_scope === 'group' ? joinTargets.target_group_id : ''
    })
    ui.toast({
      title: '自动加群任务已启动',
      message: `任务 ${res.task.id} 已开始执行，可到任务与日志查看进度。`,
      tone: 'success'
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '自动加群任务创建失败'
  } finally {
    joining.value = false
  }
}

async function refreshMemberships() {
  refreshingMemberships.value = true
  error.value = ''
  try {
    const res = await api.refreshTargetMemberships({
      account_kind: membershipRefreshKind.value,
      target_scope: filterGroupID.value ? 'group' : 'all',
      target_group_id: filterGroupID.value || ''
    })
    trackRefreshTask(res.task)
    ui.toast({
      title: '群内账号状态刷新已启动',
      message: `任务 ${res.task.id} 正在后台校验账号是否仍在目标群内。`,
      tone: 'success'
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '刷新群内账号状态失败'
  } finally {
    refreshingMemberships.value = false
  }
}

async function refreshSelectedMemberships() {
  if (!selectedMembershipTarget.value) return
  refreshingMemberships.value = true
  error.value = ''
  try {
    const res = await api.refreshTargetMemberships({
      account_kind: membershipAccountKind.value,
      target_scope: 'target',
      target_id: selectedMembershipTarget.value.id
    })
    trackRefreshTask(res.task)
    ui.toast({
      title: '目标群状态校验已启动',
      message: `任务 ${res.task.id} 已开始校验 ${selectedMembershipTarget.value.name || selectedMembershipTarget.value.identifier}。`,
      tone: 'success'
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '目标群状态校验失败'
  } finally {
    refreshingMemberships.value = false
  }
}

async function openMemberships(target: Target) {
  selectedMembershipTarget.value = target
  await loadMemberships(target.id)
}

async function loadMemberships(targetID = selectedMembershipTarget.value?.id || '') {
  if (!targetID) return
  membershipLoading.value = true
  error.value = ''
  try {
    memberships.value = await api.targetMemberships(targetID, membershipAccountKind.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '读取群内账号状态失败'
  } finally {
    membershipLoading.value = false
  }
}

function closeMemberships() {
  selectedMembershipTarget.value = null
  memberships.value = []
}

function trackRefreshTask(task: Task) {
  activeRefreshTask.value = task
  refreshingMemberships.value = true
  if (refreshTaskTimer) window.clearInterval(refreshTaskTimer)
  refreshTaskTimer = window.setInterval(pollRefreshTask, 8000) // 优化：2.5s -> 8s
}

async function pollRefreshTask() {
  const taskID = activeRefreshTask.value?.id
  if (!taskID) {
    stopRefreshTaskPolling()
    return
  }
  try {
    const tasks = await api.tasks({ type: 'target_membership_refresh', limit: 50 })
    const next = tasks.find((task) => task.id === taskID)
    if (next) activeRefreshTask.value = next
    if (next && isTaskFinished(next.status)) {
      stopRefreshTaskPolling()
      await load()
      if (selectedMembershipTarget.value) {
        await loadMemberships()
      }
      ui.toast({
        title: '群内账号状态已刷新',
        message: refreshTaskDoneMessage(next),
        tone: next.status === 'failed' ? 'error' : 'success'
      })
    }
  } catch (err) {
    stopRefreshTaskPolling()
    error.value = err instanceof Error ? err.message : '读取刷新任务进度失败'
  }
}

function stopRefreshTaskPolling() {
  if (refreshTaskTimer) {
    window.clearInterval(refreshTaskTimer)
    refreshTaskTimer = null
  }
  refreshingMemberships.value = false
}

async function readFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  form.content = await file.text()
  input.value = ''
}

function groupNames(target: Target) {
  const ids = target.group_ids?.length ? target.group_ids : target.group_id ? [target.group_id] : []
  if (!ids.length) return ['不分组']
  return ids.map((groupID) => groupMap.value.get(groupID) || '未知分组')
}

function terminalLabel(terminal: Terminal) {
  const phone = terminal.phone_display || terminal.phone || '未设置手机号'
  const nickname = terminal.nickname?.trim()
  return nickname ? `${phone} / ${nickname}` : phone
}

function typeLabel(type: string) {
  const labels: Record<string, string> = {
    channel: '公开目标',
    invite: '邀请链接',
    private_channel: '私有频道'
  }
  return labels[type] || type || '-'
}

function accountKindLabel(kind: string) {
  if (kind === 'listener') return '监听账号'
  if (kind === 'terminal') return '主账号'
  return kind || '账号'
}

function statusClass(status: string) {
  if (status === 'success') return 'text-neon'
  if (status === 'failed') return 'text-danger'
  if (status === 'duplicate') return 'text-amber'
  return 'text-steel'
}

function formatDate(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function numericSummary(summary: Record<string, unknown>, key: string) {
  const value = Number(summary[key])
  return Number.isFinite(value) ? value : 0
}

function isTaskFinished(status: string) {
  return ['success', 'failed', 'partial_success', 'stopped', 'dry_run'].includes(String(status || '').toLowerCase())
}

function taskStatusText(status: string) {
  const labels: Record<string, string> = {
    queued: '排队中',
    pending: '等待中',
    running: '运行中',
    success: '已完成',
    partial_success: '部分完成',
    failed: '失败',
    stopped: '已停止'
  }
  return labels[status] || status || '未知'
}

function taskStatusTone(status: string) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'partial_success') return 'warning'
  return 'info'
}

function refreshTaskDoneMessage(task: Task) {
  const summary = task.summary || {}
  return `有效 ${numericSummary(summary, 'active')}，移除 ${numericSummary(summary, 'removed')}，待复查 ${numericSummary(summary, 'skipped') + numericSummary(summary, 'failed')}。`
}

async function resumeRefreshTask() {
  try {
    const tasks = await api.tasks({ type: 'target_membership_refresh', limit: 50 })
    const running = tasks.find((task) => !isTaskFinished(task.status))
    if (running) trackRefreshTask(running)
  } catch {
    // 页面恢复任务失败不影响目标池主列表。
  }
}

watch(filterGroupID, load)
watch([targetKeyword, filterGroupID], () => {
  targetPage.value = 1
})
watch(targetPageCount, (count) => {
  if (targetPage.value > count) targetPage.value = count
})
watch(importGroupMode, (mode) => {
  if (mode === 'existing') {
    form.new_group_name = ''
  } else {
    form.group_id = ''
  }
})

watch(
  () => terminalImport.scope,
  (mode) => {
    if (mode !== 'group') terminalImport.terminal_group_id = ''
    if (mode !== 'terminal') terminalImport.terminal_id = ''
  }
)

watch(
  () => joinTargets.terminal_scope,
  (mode) => {
    if (mode !== 'group') joinTargets.terminal_group_id = ''
    if (mode !== 'terminal') joinTargets.terminal_id = ''
  }
)
watch(
  () => joinTargets.target_scope,
  (mode) => {
    if (mode !== 'group') joinTargets.target_group_id = ''
  }
)

watch(membershipAccountKind, async () => {
  if (selectedMembershipTarget.value) await loadMemberships()
})

onMounted(async () => {
  await load()
  await resumeRefreshTask()
})
onUnmounted(stopRefreshTaskPolling)
</script>

<template>
  <div class="page-shell outreach-shell">
    <div class="outreach-topbar">
      <div class="min-w-0">
        <div class="eyebrow">OUTREACH COMMAND</div>
        <h1 class="page-title outreach-title">监听设置</h1>
        <p class="page-subtitle outreach-subtitle">配置监听关键词，按会员查看任务与实时线索，监听资源默认走后台监听矩阵。</p>
      </div>
      <div class="outreach-actions">
        <span class="status-pill" data-tone="success">15 天行为窗口</span>
        <span class="status-pill" :data-tone="botDeliveryAvailable ? 'success' : 'warning'">
          {{ botRunning ? 'Bot 推送运行中' : botDeliveryAvailable ? 'Bot 可启动' : 'Bot 待配置' }}
        </span>
        <span class="status-pill" :data-tone="listenerStatus?.running ? 'success' : 'warning'">
          {{ listenerStatus?.running ? '监听中' : '未启动' }}
        </span>
        <span v-if="listenerStatus?.running" class="status-pill" :data-tone="listenerHealthTone">
          {{ listenerStatus.health_text || listenerHealthText }}
        </span>
        <GlassButton v-if="!listenerStatus?.running" variant="primary" size="sm" :loading="listenerBusy" @click="startListener">开始监听</GlassButton>
        <GlassButton v-else variant="danger" size="sm" :loading="listenerBusy" @click="stopListener">停止监听</GlassButton>
        <GlassButton :variant="botRunning ? 'danger' : 'success'" size="sm" :loading="botBusy" @click="toggleBotPushRuntime">
          {{ botRunning ? '停止 Bot 推送' : '启动 Bot 推送' }}
        </GlassButton>
        <label v-if="isAdmin" class="member-scope-control">
          <span>会员视图</span>
          <select v-model="memberFilterID">
            <option value="">全部会员</option>
            <option v-for="user in memberUsers" :key="user.id" :value="user.id">{{ userLabel(user) }}</option>
          </select>
        </label>
        <GlassButton variant="secondary" size="sm" :loading="loading || loadingLookups" @click="refreshAll">刷新</GlassButton>
      </div>
    </div>

    <div class="outreach-stat-grid">
      <div class="outreach-stat">
        <span>当前规则</span>
        <strong>{{ activeRule ? ruleDisplayName(activeRule) : '未保存' }}</strong>
      </div>
      <div class="outreach-stat">
        <span>关键词</span>
        <strong>{{ keywordCount }} 个 · {{ matchModeLabel }}</strong>
      </div>
      <div class="outreach-stat">
        <span>监听目标</span>
        <strong>{{ listenerTargetCoverageText }}</strong>
      </div>
      <div class="outreach-stat">
        <span>命中线索</span>
        <strong>{{ listenerStatus?.match_count ?? 0 }} 次 / {{ leads.length }} 条</strong>
      </div>
      <div class="outreach-stat">
        <span>实时同步</span>
        <strong>{{ inboxSocketConnected ? 'WS 加速' : inboxPolling ? '秒级轮询' : '暂停' }}</strong>
      </div>
      <div class="outreach-stat">
        <span>监听心跳</span>
        <strong>{{ listenerHeartbeatText }}</strong>
      </div>
    </div>

    <div class="outreach-workspace">
      <div class="outreach-left-stack">
        <GlassCard padding="none" class="outreach-panel outreach-config-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">01 ORCHESTRATION</div>
              <h2>任务编排</h2>
            </div>
            <span class="status-pill" :data-tone="listenerStatus?.running ? 'success' : 'warning'">
              {{ listenerStatus?.running ? '运行中' : '待启动' }}
            </span>
          </div>

          <div class="config-grid">
            <section class="config-section keyword-section">
              <div class="section-title">
                <div>
                  <h3>关键词设置</h3>
                  <p>一行一个，按当前匹配模式命中。</p>
                </div>
                <span>{{ keywordCount }} 个</span>
              </div>
              <textarea
                v-model="keywordsText"
                class="outreach-textarea"
                placeholder="合作&#10;请教&#10;多少钱&#10;教程"
              ></textarea>
              <div v-if="keywordList.length" class="keyword-chip-row">
                <button v-for="keyword in keywordList" :key="keyword" type="button" @click="removeKeyword(keyword)">
                  {{ keyword }} <span>×</span>
                </button>
              </div>
              <div class="segmented">
                <button
                  type="button"
                  :class="{ active: matchMode === 'fuzzy' }"
                  @click="matchMode = 'fuzzy'"
                >
                  模糊匹配
                </button>
                <button
                  type="button"
                  :class="{ active: matchMode === 'exact' }"
                  @click="matchMode = 'exact'"
                >
                  精确匹配
                </button>
              </div>
            </section>

            <section class="config-section routing-section">
              <div class="section-title">
                <div>
                  <h3>默认资源</h3>
                  <p>主动触达不再单独选择资源范围，统一使用监听矩阵全部可用目标和账号。</p>
                </div>
              </div>

              <div class="resource-default-grid">
                <div>
                  <span>监听目标</span>
                  <strong>全部监听矩阵目标</strong>
                  <small>后台监听矩阵统一维护</small>
                </div>
                <div>
                  <span>监听账号</span>
                  <strong>全部监听矩阵账号</strong>
                  <small>自动过滤不可用账号</small>
                </div>
                <div>
                  <span>私信动作</span>
                  <strong>已从本页移除</strong>
                  <small>本模块只负责捕获与推送</small>
                </div>
              </div>

              <div class="switch-row">
                <div>
                  <strong>Bot 推送</strong>
                  <span>{{ pushToBot ? '任务内开启' : '任务内关闭' }} · {{ botRunning ? '推送运行中' : botDeliveryAvailable ? '可启动' : '先配置 Bot' }}</span>
                </div>
                <input v-model="pushToBot" type="checkbox" />
              </div>
            </section>

            <section class="config-section monitor-section">
              <div class="section-title">
                <div>
                  <h3>监听矩阵账号</h3>
                  <p>这里仅展示后台监听矩阵的可用账号，保存任务时默认使用全部账号，不再绑定单独账号组。</p>
                </div>
                <span>{{ monitorAccountSummary }}</span>
              </div>

              <div class="monitor-toolbar">
                <input v-model="monitorKeyword" class="outreach-input" placeholder="搜索手机号 / 昵称 / 状态" />
                <span class="matrix-source-pill">全部矩阵账号</span>
              </div>
              <div class="monitor-bulk-row">
                <span>显示 {{ visibleMonitorAccounts.length }} / {{ filteredMonitorAccounts.length }}</span>
                <span>当前任务默认使用全部可用监听矩阵账号</span>
              </div>

              <div class="terminal-list scrollbar-thin">
                <div
                  v-for="account in visibleMonitorAccounts"
                  :key="account.id"
                  class="terminal-option matrix-preview-option"
                >
                  <div class="matrix-status-dot" :data-tone="listenerAccountReady(account) ? 'success' : 'warning'"></div>
                  <div class="min-w-0">
                    <strong>{{ listenerAccountLabel(account) }}</strong>
                    <span>{{ listenerAccountMeta(account) }}</span>
                  </div>
                </div>
                <div v-if="filteredMonitorAccountOverflow > 0" class="list-overflow-note">还有 {{ filteredMonitorAccountOverflow }} 个监听号，可继续搜索缩小范围。</div>
                <div v-if="!filteredMonitorAccounts.length" class="empty-mini">当前监听矩阵账号组没有可用账号</div>
              </div>
            </section>
          </div>

          <div class="config-footer">
            <div class="listener-meta">
              <span>最近事件</span>
              <strong>{{ formatDateTime(listenerStatus?.last_event_at) }}</strong>
            </div>
            <div class="listener-meta">
              <span>心跳状态</span>
              <strong>{{ listenerStatus?.health_text || listenerHealthText }}</strong>
            </div>
            <GlassButton variant="primary" class="save-rule-btn" :loading="savingRule" @click="saveRule">保存任务</GlassButton>
          </div>
        </GlassCard>

        <GlassCard padding="none" class="outreach-panel outreach-task-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">02 TASKS</div>
              <h2>监听任务</h2>
            </div>
            <span class="status-pill" data-tone="info">{{ listenerTasks.length }} 个任务</span>
          </div>
          <div class="panel-filter-row">
            <input v-model="ruleKeyword" class="outreach-input" placeholder="搜索任务 / 关键词 / 分组" />
          </div>

          <div class="listener-task-list scrollbar-thin">
            <div v-for="rule in filteredListenerTasks" :key="rule.id" class="listener-task-card">
              <div class="task-main">
                <div class="task-icon">◌</div>
                <div class="min-w-0">
                  <strong>{{ ruleDisplayName(rule) }}</strong>
                  <span>{{ ruleOwnerLabel(rule) }} · {{ ruleKeywords(rule).length }} 个关键词 · {{ rule.match_mode === 'exact' ? '精准匹配' : '模糊匹配' }}</span>
                </div>
              </div>
              <div class="task-keywords">
                <button v-for="keyword in ruleKeywords(rule).slice(0, 6)" :key="keyword" type="button" @click="editRule(rule)">
                  {{ keyword }}
                </button>
                <span v-if="ruleKeywords(rule).length > 6">+{{ ruleKeywords(rule).length - 6 }}</span>
              </div>
              <div class="task-metrics">
                <div>
                  <span>资源范围</span>
                  <strong>全部监听矩阵</strong>
                </div>
                <div>
                  <span>监听账号</span>
                  <strong>{{ ruleMonitorGroupLabel(rule) }}</strong>
                </div>
                <div>
                  <span>Bot</span>
                  <strong>{{ rule.push_to_bot ? '推送' : '关闭' }}</strong>
                </div>
              </div>
              <div class="task-actions">
                <span class="status-pill" :data-tone="ruleStatusTone(rule)">{{ ruleStatusText(rule) }}</span>
                <GlassButton
                  v-if="!(listenerStatus?.running && listenerStatus.rule_id === rule.id)"
                  size="sm"
                  variant="success"
                  :loading="taskActionBusy === `start-${rule.id}`"
                  @click="startRule(rule)"
                >
                  启动
                </GlassButton>
                <GlassButton
                  v-else
                  size="sm"
                  variant="secondary"
                  :loading="taskActionBusy === `pause-${rule.id}`"
                  @click="pauseRule(rule)"
                >
                  暂停
                </GlassButton>
                <GlassButton size="sm" variant="ghost" @click="editRule(rule)">修改</GlassButton>
                <GlassButton size="sm" variant="danger" :loading="taskActionBusy === `delete-${rule.id}`" @click="deleteRule(rule)">删除</GlassButton>
              </div>
            </div>

            <div v-if="!filteredListenerTasks.length" class="empty-state compact-empty">
              <strong>还没有监听任务</strong>
              <span>配置关键词和监听账号后，点击保存任务。</span>
            </div>
          </div>
        </GlassCard>

        <GlassCard padding="none" class="outreach-panel outreach-inbox-panel">
          <div class="panel-head">
            <div>
              <div class="eyebrow">03 INBOX</div>
              <h2>汇聚收件箱</h2>
            </div>
            <div class="panel-pills">
              <span class="status-pill" :data-tone="inboxSocketConnected ? 'success' : inboxPolling ? 'warning' : 'warning'">
                {{ inboxSocketConnected ? 'WS 实时加速' : inboxPolling ? '秒级同步中' : '同步暂停' }}
              </span>
              <span class="status-pill" data-tone="info">{{ lastInboxSyncedAt ? `更新 ${formatTimeOnly(lastInboxSyncedAt)}` : '等待同步' }}</span>
              <span class="status-pill" data-tone="warning">{{ leads.length }} 条</span>
            </div>
          </div>
          <div class="panel-filter-row">
            <input v-model="leadKeyword" class="outreach-input" placeholder="搜索用户 / 来源群 / 关键词 / 消息内容" />
            <span v-if="isAdmin" class="member-filter-pill">{{ selectedMemberLabel }}</span>
          </div>

          <div class="lead-stream scrollbar-thin">
            <article
              v-for="lead in visibleLeads"
              :key="lead.id"
              class="lead-card"
            >
              <div class="lead-card-top">
                <div class="lead-avatar">{{ leadDisplayName(lead).slice(0, 1).toUpperCase() }}</div>
                <div class="min-w-0">
                  <strong>{{ leadDisplayName(lead) }}</strong>
                  <span>{{ leadAccountLabel(lead) }}</span>
                </div>
                <time>{{ formatTimeOnly(lead.hit_time || lead.hit_at || lead.created_at) }}</time>
              </div>
              <div class="lead-meta-grid">
                <div v-if="isAdmin">
                  <span>归属会员</span>
                  <strong>{{ leadOwnerLabel(lead) }}</strong>
                </div>
                <div>
                  <span>来源</span>
                  <strong>{{ lead.source_chat_name || '未记录来源群组' }}</strong>
                </div>
                <div>
                  <span>关键词</span>
                  <strong>{{ lead.trigger_word || '未记录' }}</strong>
                </div>
              </div>
              <p class="lead-message">{{ lead.trigger_message || '暂无消息内容' }}</p>
              <div class="lead-history">
                <span>15 天记录 {{ (lead.recent_history || []).length }} 条</span>
                <span>{{ formatDateTime(lead.hit_time || lead.hit_at || lead.created_at) }}</span>
              </div>
              <div v-if="lead.recent_history?.length" class="history-strip">
                <div v-for="(item, index) in lead.recent_history.slice(0, 2)" :key="`${lead.id}-${index}`">
                  <span>{{ item.keyword || '未记录关键词' }}</span>
                  <strong>{{ item.source_chat_name || '未知群组' }}</strong>
                </div>
              </div>
            </article>

            <div v-if="filteredLeadOverflow > 0" class="list-overflow-note">已显示前 {{ visibleLeads.length }} 条，还有 {{ filteredLeadOverflow }} 条可通过搜索定位。</div>
            <div v-if="!filteredLeads.length && !loading" class="empty-state">
              <strong>当前还没有真实捕获线索</strong>
              <span>监听矩阵目标命中关键词后，会进入这里。</span>
            </div>
          </div>
        </GlassCard>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, type BotConfig, type ListenerAccount, type SCRMKeywordRule, type SCRMLead, type SCRMListenerStatus, type TaskLog, type User } from '../api/client'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'
import { useUserStore } from '../stores/user'
import { taskDisplayName } from '../utils/taskDisplay'

const ui = useUiStore()
const userStore = useUserStore()

const loading = ref(false)
const loadingLookups = ref(false)
const savingRule = ref(false)
const listenerBusy = ref(false)
const botBusy = ref(false)
const taskActionBusy = ref('')

const activeRule = ref<SCRMKeywordRule | null>(null)
const rules = ref<SCRMKeywordRule[]>([])
const listenerAccounts = ref<ListenerAccount[]>([])
const users = ref<User[]>([])
const listenerStatus = ref<SCRMListenerStatus | null>(null)
const botConfig = ref<BotConfig | null>(null)

const keywordsText = ref('合作\n买\n请教\n多少钱\nU')
const matchMode = ref<'fuzzy' | 'exact'>('fuzzy')
const pushToBot = ref(false)
const monitorKeyword = ref('')
const ruleKeyword = ref('')
const leadKeyword = ref('')
const memberFilterID = ref('')
const monitorDisplayLimit = 80
const leadDisplayLimit = 120

const leads = ref<SCRMLead[]>([])
const inboxPolling = ref(false)
const inboxRefreshing = ref(false)
const lastInboxSyncedAt = ref<string | null>(null)
const inboxSocketConnected = ref(false)
let inboxTimer: ReturnType<typeof window.setInterval> | null = null
let inboxSocket: WebSocket | null = null
let lastInboxLogID = ''

const botDeliveryAvailable = computed(() => Boolean(botConfig.value?.enabled && botConfig.value?.last_test_status === 'success'))
const botRunning = computed(() => Boolean(botConfig.value?.running))
const isAdmin = computed(() => userStore.user?.role === 'admin')
const memberUsers = computed(() => users.value.filter((user) => user.role === 'user'))
const selectedMemberLabel = computed(() => {
  if (!memberFilterID.value) return '全部会员'
  return userLabel(users.value.find((user) => user.id === memberFilterID.value) || null)
})

const scrmLogSocketURL = computed(() => {
  const token = localStorage.getItem('codex3_token')
  if (!token) return ''

  const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''
  const base =
    apiBase !== ''
      ? apiBase.replace(/^http/i, 'ws')
      : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`

  const params = new URLSearchParams({
    access_token: token,
    category: 'scrm_listener'
  })
  if (isAdmin.value && memberFilterID.value) {
    params.set('user_id', memberFilterID.value)
  }
  return `${base}/api/v1/ws/logs?${params.toString()}`
})

const keywordList = computed(() =>
  Array.from(
    new Set(
      keywordsText.value
        .split('\n')
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
)

const keywordCount = computed(() => keywordList.value.length)
const matchModeLabel = computed(() => (matchMode.value === 'exact' ? '精确匹配' : '模糊匹配'))
const selectedMonitorAccounts = computed(() => listenerAccounts.value.filter(listenerAccountReady))
const listenerTasks = computed(() => rules.value)
const listenerHealthTone = computed(() => {
  switch (listenerStatus.value?.health_status) {
    case 'healthy':
      return 'success'
    case 'idle':
    case 'starting':
      return 'info'
    case 'silent':
      return 'warning'
    case 'stale':
      return 'danger'
    default:
      return listenerStatus.value?.running ? 'warning' : 'neutral'
  }
})
const listenerHealthText = computed(() => {
  if (!listenerStatus.value?.running) return '监听未启动'
  return listenerStatus.value.health_text || '等待监听心跳'
})
const listenerTargetCoverageText = computed(() => {
  const total = listenerStatus.value?.target_count ?? 0
  const covered = listenerStatus.value?.covered_target_count
  if (typeof covered === 'number') return `${covered}/${total} 个`
  return `${total} 个`
})
const listenerHeartbeatText = computed(() => {
  if (!listenerStatus.value?.running) return '未启动'
  if (!listenerStatus.value.last_heartbeat_at) return '等待心跳'
  return formatRelativeTime(listenerStatus.value.last_heartbeat_at)
})
const monitorAccountSummary = computed(() => {
  const ready = selectedMonitorAccounts.value.length
  const total = listenerAccounts.value.length
  if (!isAdmin.value) return '管理员配置'
  return `${ready}/${total || 0}`
})
const filteredMonitorAccounts = computed(() => {
  const keyword = normalizeKeyword(monitorKeyword.value)
  if (!keyword) return listenerAccounts.value
  return listenerAccounts.value.filter((account) => {
    return [
      listenerAccountLabel(account),
      listenerAccountMeta(account),
      account.phone,
      account.phone_display,
      account.nickname,
      account.status,
      account.status_text,
      account.risk_status
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const visibleMonitorAccounts = computed(() => filteredMonitorAccounts.value.slice(0, monitorDisplayLimit))
const filteredMonitorAccountOverflow = computed(() => Math.max(0, filteredMonitorAccounts.value.length - visibleMonitorAccounts.value.length))
const filteredListenerTasks = computed(() => {
  const keyword = normalizeKeyword(ruleKeyword.value)
  if (!keyword) return listenerTasks.value
  return listenerTasks.value.filter((rule) => {
    return [
      rule.name,
      ruleDisplayName(rule),
      rule.status,
      ruleStatusText(rule),
      ruleOwnerLabel(rule),
      ruleMonitorGroupLabel(rule),
      ...ruleKeywords(rule)
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const filteredLeads = computed(() => {
  const keyword = normalizeKeyword(leadKeyword.value)
  if (!keyword) return leads.value
  return leads.value.filter((lead) => {
    return [
      leadDisplayName(lead),
      leadAccountLabel(lead),
      leadOwnerLabel(lead),
      lead.source_chat_name,
      lead.trigger_word,
      lead.trigger_message,
      lead.target_id
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const visibleLeads = computed(() => filteredLeads.value.slice(0, leadDisplayLimit))
const filteredLeadOverflow = computed(() => Math.max(0, filteredLeads.value.length - visibleLeads.value.length))

function scrmScopeParams() {
  if (!isAdmin.value || !memberFilterID.value) return undefined
  return { user_id: memberFilterID.value }
}

async function refreshAll() {
  await Promise.all([loadLookups(), loadRules(), loadLeads(), loadListenerStatus(), loadBotConfig()])
}

async function loadLookups() {
  loadingLookups.value = true
  try {
    if (isAdmin.value) {
      const [listenerAccountData, userData] = await Promise.all([
        api.listenerAccounts(),
        api.users()
      ])
      listenerAccounts.value = listenerAccountData
      users.value = userData
    }
  } catch (err) {
    ui.toast({ title: '分组读取失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    loadingLookups.value = false
  }
}

async function loadRules() {
  try {
    const data = await api.scrmRules(scrmScopeParams())
    rules.value = data
    const latest = [...data].sort((a, b) => {
      const aTime = new Date(a.updated_at || a.created_at).getTime()
      const bTime = new Date(b.updated_at || b.created_at).getTime()
      return bTime - aTime
    })[0] || null

    activeRule.value = latest
    if (!latest) {
      return
    }

    const savedWords = Array.isArray(latest.keywords?.list)
      ? latest.keywords.list.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
      : []
    if (savedWords.length) {
      keywordsText.value = savedWords.join('\n')
    } else if (typeof latest.keywords?.text === 'string' && latest.keywords.text.trim()) {
      keywordsText.value = latest.keywords.text.trim()
    }

    matchMode.value = latest.match_mode === 'exact' ? 'exact' : 'fuzzy'
    pushToBot.value = Boolean(latest.push_to_bot)
  } catch (err) {
    ui.toast({ title: '规则读取失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  }
}

async function loadBotConfig() {
  try {
    botConfig.value = await api.botConfig()
  } catch {
    botConfig.value = null
  }
}

async function loadLeads(options: { silent?: boolean } = {}) {
  if (!options.silent) {
    loading.value = true
  }
  try {
    const raw = await api.scrmLeads(scrmScopeParams())
    leads.value = raw
    lastInboxSyncedAt.value = new Date().toISOString()
  } catch (err) {
    if (!options.silent) {
      ui.toast({ title: '捕获流读取失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
    }
  } finally {
    if (!options.silent) {
      loading.value = false
    }
  }
}

async function loadListenerStatus() {
  try {
    listenerStatus.value = await api.scrmListenerStatus()
  } catch {
    listenerStatus.value = null
  }
}

async function saveRule() {
  if (!keywordList.value.length) {
    ui.toast({ title: '关键词不能为空', message: '请至少输入一个关键词，每行一个。', tone: 'warning' })
    return false
  }

  savingRule.value = true
  try {
    const rule = await api.createScrmRule({
      id: activeRule.value?.id,
      name: activeRule.value?.name || `雷达监听任务 ${new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`,
      keywords: {
        list: keywordList.value,
        text: keywordsText.value
      },
      monitor_terminal_ids: [],
      match_mode: matchMode.value,
      push_to_bot: pushToBot.value,
      strike_enabled: false
    })
    activeRule.value = rule
    await loadRules()

    if (pushToBot.value && !botDeliveryAvailable) {
      ui.toast({
        title: '任务已保存',
        message: '监听任务已保存；Bot 推送开关已记录，请先在 Bot 配置里完成连接测试并启动。',
        tone: 'warning',
        duration: 4200
      })
      return true
    }

    ui.toast({ title: '监听任务已保存', message: '关键词、匹配模式和 Bot 推送已保存；监听资源默认使用全部矩阵账号。', tone: 'success' })
    return true
  } catch (err) {
    ui.toast({ title: '指令推送失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
    return false
  } finally {
    savingRule.value = false
  }
}

async function startListener() {
  listenerBusy.value = true
  try {
    const saved = await saveRule()
    if (!saved) {
      return
    }
    const result = activeRule.value?.id ? await api.startScrmRule(activeRule.value.id) : await api.startScrmListener()
    ui.toast({
      title: '监听已启动',
      message: taskDisplayName(result.task),
      tone: 'success'
    })
    await refreshAll()
  } catch (err) {
    ui.toast({ title: '启动监听失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    listenerBusy.value = false
  }
}

async function startRule(rule: SCRMKeywordRule) {
  taskActionBusy.value = `start-${rule.id}`
  try {
    const result = await api.startScrmRule(rule.id)
    ui.toast({ title: '监听任务已启动', message: taskDisplayName(result.task), tone: 'success' })
    await refreshAll()
  } catch (err) {
    ui.toast({ title: '启动任务失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    taskActionBusy.value = ''
  }
}

async function pauseRule(rule: SCRMKeywordRule) {
  taskActionBusy.value = `pause-${rule.id}`
  try {
    await api.pauseScrmRule(rule.id)
    ui.toast({ title: '监听任务已暂停', message: ruleDisplayName(rule), tone: 'success' })
    await refreshAll()
  } catch (err) {
    ui.toast({ title: '暂停任务失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    taskActionBusy.value = ''
  }
}

function editRule(rule: SCRMKeywordRule) {
  activeRule.value = rule
  const savedWords = Array.isArray(rule.keywords?.list)
    ? rule.keywords.list.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : []
  keywordsText.value = savedWords.length ? savedWords.join('\n') : typeof rule.keywords?.text === 'string' ? rule.keywords.text : ''
  matchMode.value = rule.match_mode === 'exact' ? 'exact' : 'fuzzy'
  pushToBot.value = Boolean(rule.push_to_bot)
  ui.toast({ title: '已载入任务', message: '修改关键词、匹配模式或 Bot 推送后点击保存任务。', tone: 'info' })
}

async function deleteRule(rule: SCRMKeywordRule) {
  taskActionBusy.value = `delete-${rule.id}`
  try {
    await api.deleteScrmRule(rule.id)
    if (activeRule.value?.id === rule.id) {
      activeRule.value = null
    }
    await loadRules()
    ui.toast({ title: '监听任务已删除', message: ruleDisplayName(rule), tone: 'success' })
  } catch (err) {
    ui.toast({ title: '删除任务失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    taskActionBusy.value = ''
  }
}

async function toggleBotPushRuntime() {
  botBusy.value = true
  try {
    if (botRunning.value) {
      const result = await api.stopBotPush()
      botConfig.value = result.config
      ui.toast({ title: 'Bot 推送已停止', message: '监听任务仍可继续运行。', tone: 'success' })
    } else {
      const result = await api.startBotPush()
      botConfig.value = result.config
      ui.toast({ title: 'Bot 推送已启动', message: '命中线索会推送到配置的 Chat ID。', tone: 'success' })
    }
  } catch (err) {
    ui.toast({ title: 'Bot 推送操作失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    botBusy.value = false
  }
}

function removeKeyword(keyword: string) {
  keywordsText.value = keywordList.value.filter((item) => item !== keyword).join('\n')
}

async function stopListener() {
  listenerBusy.value = true
  try {
    await api.stopScrmListener()
    ui.toast({ title: '监听已停止', message: '后台监听进程已经停止。', tone: 'success' })
    await loadListenerStatus()
  } catch (err) {
    ui.toast({ title: '停止监听失败', message: err instanceof Error ? err.message : '错误', tone: 'error' })
  } finally {
    listenerBusy.value = false
  }
}

async function refreshInboxRealtime() {
  if (inboxRefreshing.value) return
  inboxRefreshing.value = true
  try {
    await Promise.all([
      loadListenerStatus(),
      loadLeads({ silent: true })
    ])
  } finally {
    inboxRefreshing.value = false
  }
}

function startInboxRealtime() {
  stopInboxRealtime()
  inboxPolling.value = true
  inboxTimer = window.setInterval(() => {
    void refreshInboxRealtime()
  }, 1000)
  connectInboxLogStream()
}

function stopInboxRealtime() {
  if (inboxTimer) {
    window.clearInterval(inboxTimer)
    inboxTimer = null
  }
  if (inboxSocket) {
    inboxSocket.close()
    inboxSocket = null
  }
  inboxSocketConnected.value = false
  inboxPolling.value = false
}

function connectInboxLogStream() {
  if (!scrmLogSocketURL.value) return
  if (inboxSocket) {
    inboxSocket.close()
    inboxSocket = null
  }

  inboxSocket = new WebSocket(scrmLogSocketURL.value)
  inboxSocket.onopen = () => {
    inboxSocketConnected.value = true
  }
  inboxSocket.onclose = () => {
    inboxSocketConnected.value = false
  }
  inboxSocket.onerror = () => {
    inboxSocketConnected.value = false
  }
  inboxSocket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data) as { type?: string; data?: TaskLog[]; error?: string }
      if (payload.error || payload.type !== 'logs' || !Array.isArray(payload.data) || !payload.data.length) {
        return
      }

      const latest = payload.data[0]
      if (latest.id === lastInboxLogID) return

      const hasFreshSCRMEvent = payload.data.some((log) => {
        if (lastInboxLogID && log.id === lastInboxLogID) return false
        return ['match', 'message_received', 'ready', 'start'].includes(log.action)
      })
      lastInboxLogID = latest.id

      if (hasFreshSCRMEvent) {
        void refreshInboxRealtime()
      }
    } catch {
      inboxSocketConnected.value = false
    }
  }
}

function leadDisplayName(lead: SCRMLead) {
  return lead.user_nickname?.trim() || lead.user_account?.trim() || lead.target_id || '未识别用户'
}

function leadAccountLabel(lead: SCRMLead) {
  return lead.user_account?.trim() || lead.target_id || '未记录账号'
}

function listenerAccountLabel(account: ListenerAccount) {
  return account.phone_display?.trim() || account.phone?.trim() || account.nickname?.trim() || account.id
}

function listenerAccountMeta(account: ListenerAccount) {
  const status = account.status_text?.trim() || account.status?.trim() || '未检查'
  const joined = typeof account.joined_target_count === 'number' ? `已挂载 ${account.joined_target_count}/${account.target_total_count || 0}` : '未统计挂载'
  return `${account.nickname?.trim() || '未命名监听号'} · ${status} · ${joined}`
}

function listenerAccountReady(account: ListenerAccount) {
  const status = normalizeKeyword(account.status)
  const risk = normalizeKeyword(account.risk_status)
  return !['abnormal', 'disabled', 'banned'].includes(status) && !['restricted', 'banned', 'disabled'].includes(risk)
}

function userLabel(user?: User | null) {
  if (!user) return '未知会员'
  const telegram = user.telegram_username || user.telegram_user_id
  return telegram ? `${user.username} · ${telegram}` : user.username
}

function ownerIDFromTenant(value?: string | null) {
  if (!value || value === '00000000-0000-0000-0000-000000000000') return ''
  return value
}

function ruleCreator(rule: SCRMKeywordRule) {
  if (rule.creator) return rule.creator
  const ownerID = rule.owner_user_id || ownerIDFromTenant(rule.tenant_id)
  if (!isAdmin.value && userStore.user) {
    return {
      id: userStore.user.id,
      username: userStore.user.username,
      role: userStore.user.role,
      telegram_user_id: userStore.user.telegram_user_id,
      telegram_username: userStore.user.telegram_username
    }
  }
  if (!ownerID) {
    return userStore.user
      ? {
          id: userStore.user.id,
          username: userStore.user.username,
          role: userStore.user.role,
          telegram_user_id: userStore.user.telegram_user_id,
          telegram_username: userStore.user.telegram_username
        }
      : null
  }
  const user = users.value.find((item) => item.id === ownerID)
  if (!user) return null
  return {
    id: user.id,
    username: user.username,
    role: user.role,
    telegram_user_id: user.telegram_user_id,
    telegram_username: user.telegram_username
  }
}

function ruleDisplayName(rule: SCRMKeywordRule) {
  const inferredBotUser = !rule.bot_user && /^bot\s*radar$/i.test(rule.name || '')
    ? { id: '', nickname: 'Bot用户', username: '', telegram_user_id: '', status: '', plan: '' }
    : null
  return taskDisplayName({
    name: rule.name,
    type: 'scrm_listener',
    status: rule.status,
    creator: ruleCreator(rule),
    bot_user: rule.bot_user || inferredBotUser
  })
}

function ownerLabel(ownerUserID?: string | null, tenantID?: string | null) {
  const ownerID = ownerUserID || ownerIDFromTenant(tenantID)
  if (!ownerID) return '管理员 / 全局'
  return userLabel(users.value.find((user) => user.id === ownerID) || null)
}

function ruleOwnerLabel(rule: SCRMKeywordRule) {
  if (!isAdmin.value) return '当前会员'
  return ownerLabel(rule.owner_user_id, rule.tenant_id)
}

function leadOwnerLabel(lead: SCRMLead) {
  if (!isAdmin.value) return '当前会员'
  return ownerLabel(lead.owner_user_id, lead.tenant_id)
}

function ruleMonitorGroupLabel(rule: SCRMKeywordRule) {
  return '全部监听矩阵账号'
}

function ruleKeywords(rule: SCRMKeywordRule) {
  if (Array.isArray(rule.keywords?.list)) {
    return rule.keywords.list.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
  }
  if (typeof rule.keywords?.text === 'string') {
    return rule.keywords.text.split('\n').map((item) => item.trim()).filter(Boolean)
  }
  return []
}

function ruleStatusText(rule: SCRMKeywordRule) {
  if (listenerStatus.value?.running && listenerStatus.value.rule_id === rule.id) return '运行中'
  return ({ active: '已保存', running: '运行中', paused: '已暂停', stopped: '已停止' } as Record<string, string>)[rule.status] || rule.status || '已保存'
}

function ruleStatusTone(rule: SCRMKeywordRule) {
  if (listenerStatus.value?.running && listenerStatus.value.rule_id === rule.id) return 'success'
  if (rule.status === 'paused' || rule.status === 'stopped') return 'info'
  if (rule.status === 'failed') return 'danger'
  return 'warning'
}

function normalizeKeyword(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

function formatDateTime(value?: string | null) {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

function formatRelativeTime(value?: string | null) {
  if (!value) return '--'
  const time = new Date(value).getTime()
  if (Number.isNaN(time)) return value
  const diffSeconds = Math.max(0, Math.floor((Date.now() - time) / 1000))
  if (diffSeconds < 60) return `${diffSeconds} 秒前`
  const diffMinutes = Math.floor(diffSeconds / 60)
  if (diffMinutes < 60) return `${diffMinutes} 分钟前`
  const diffHours = Math.floor(diffMinutes / 60)
  return `${diffHours} 小时前`
}

function formatTimeOnly(value?: string | null) {
  if (!value) return ''
  return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

watch(memberFilterID, async () => {
  await Promise.all([loadRules(), loadLeads()])
  stopInboxRealtime()
  startInboxRealtime()
})

onMounted(() => {
  void refreshAll().finally(() => {
    startInboxRealtime()
  })
})

onUnmounted(() => {
  stopInboxRealtime()
})
</script>

<style scoped>
.outreach-shell {
  min-height: calc(100vh - 6.25rem);
  gap: 1rem;
  overflow: visible;
  padding: 0.25rem 0 1.25rem;
}

.outreach-topbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  flex-shrink: 0;
}

.eyebrow {
  color: rgba(147, 164, 198, 0.9);
  font-size: 0.68rem;
  font-weight: 900;
  letter-spacing: 0;
  text-transform: uppercase;
}

.outreach-title {
  margin-top: 0.15rem;
  font-size: 2.35rem;
}

.outreach-subtitle {
  max-width: none;
  font-size: 0.88rem;
}

.outreach-actions,
.panel-pills {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}

.member-scope-control {
  display: grid;
  min-width: 12.5rem;
  gap: 0.22rem;
}

.member-scope-control span {
  color: rgba(147, 164, 198, 0.9);
  font-size: 0.62rem;
  font-weight: 900;
  letter-spacing: 0;
}

.member-scope-control select {
  min-height: 2.15rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.72);
  padding: 0 0.65rem;
  color: white;
  font-size: 0.78rem;
  font-weight: 800;
}

.outreach-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: 0.65rem;
  flex-shrink: 0;
}

.outreach-stat,
.lead-meta-grid > div,
.listener-meta {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.055), transparent 30%),
    rgba(15, 23, 42, 0.56);
  box-shadow: 0 8px 22px rgba(2, 6, 23, 0.24);
}

.outreach-stat {
  min-height: 4.15rem;
  padding: 0.75rem 0.9rem;
}

.outreach-stat span,
.lead-meta-grid span,
.listener-meta span {
  display: block;
  color: rgba(147, 164, 198, 0.94);
  font-size: 0.7rem;
  font-weight: 800;
}

.outreach-stat strong,
.lead-meta-grid strong,
.listener-meta strong {
  display: block;
  margin-top: 0.35rem;
  overflow: hidden;
  color: white;
  font-size: 0.95rem;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.outreach-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
  gap: 1rem;
  min-height: 0;
}

.outreach-left-stack {
  display: grid;
  grid-template-columns: minmax(390px, 0.42fr) minmax(560px, 0.58fr);
  grid-template-rows: minmax(18rem, auto) minmax(34rem, auto);
  gap: 1rem;
  min-width: 0;
  min-height: 0;
}

.outreach-config-panel {
  grid-row: 1 / span 2;
}

.outreach-panel {
  display: flex;
  min-height: 0;
  overflow: hidden;
}

.outreach-config-panel,
.outreach-inbox-panel,
.outreach-task-panel {
  flex-direction: column;
}

.panel-head {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background:
    linear-gradient(90deg, rgba(0, 242, 254, 0.08), transparent 45%),
    rgba(15, 23, 42, 0.22);
  padding: 0.85rem 1rem;
}

.panel-head h2,
.section-title h3 {
  margin: 0.12rem 0 0;
  color: white;
  font-size: 1rem;
  font-weight: 900;
}

.config-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.75rem;
  min-height: 0;
  flex: 1;
  align-content: start;
  overflow: visible;
  padding: 0.8rem;
}

.monitor-section {
  grid-column: auto;
}

.config-section {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 0.7rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.045), transparent 24%),
    rgba(2, 8, 23, 0.24);
  padding: 0.8rem;
}

.section-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.section-title p {
  margin: 0.25rem 0 0;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
  line-height: 1.55;
}

.section-title > span {
  flex-shrink: 0;
  border-radius: 999px;
  border: 1px solid rgba(79, 172, 254, 0.24);
  background: rgba(0, 242, 254, 0.08);
  padding: 0.25rem 0.55rem;
  color: #7deeff;
  font-size: 0.72rem;
  font-weight: 800;
}

.outreach-textarea {
  min-height: 10.5rem;
  flex: 0 0 auto;
  resize: none;
  border-radius: 8px;
  padding: 0.85rem 0.95rem;
  font-size: 0.88rem;
  line-height: 1.7;
}

.segmented {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
}

.segmented button {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.045);
  color: rgba(200, 210, 234, 0.95);
  padding: 0.7rem 0.75rem;
  font-size: 0.85rem;
  font-weight: 900;
}

.segmented button:hover,
.segmented button.active {
  border-color: rgba(79, 172, 254, 0.5);
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.16), rgba(79, 172, 254, 0.12));
  color: #7deeff;
  box-shadow: 0 10px 24px rgba(79, 172, 254, 0.16);
}

.keyword-chip-row,
.task-keywords {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.keyword-chip-row button,
.task-keywords button,
.task-keywords span {
  border: 1px solid rgba(244, 114, 182, 0.2);
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(244, 114, 182, 0.14), rgba(34, 197, 94, 0.08));
  color: #f9a8d4;
  padding: 0.32rem 0.58rem;
  font-size: 0.72rem;
  font-weight: 800;
}

.keyword-chip-row span {
  color: #fecdd3;
}

.resource-default-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.55rem;
}

.resource-default-grid > div {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(34, 197, 94, 0.08), transparent 42%),
    rgba(255, 255, 255, 0.035);
  padding: 0.7rem 0.75rem;
}

.resource-default-grid span,
.resource-default-grid small {
  display: block;
  overflow: hidden;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.68rem;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-default-grid strong {
  display: block;
  margin: 0.26rem 0;
  overflow: hidden;
  color: white;
  font-size: 0.88rem;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.85rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.72rem 0.78rem;
}

.switch-row strong,
.terminal-option strong {
  display: block;
  color: white;
  font-size: 0.86rem;
  font-weight: 900;
}

.switch-row span,
.terminal-option span {
  display: block;
  margin-top: 0.22rem;
  overflow: hidden;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.terminal-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  align-content: start;
  gap: 0.5rem;
  min-height: 0;
  max-height: 16rem;
  overflow: auto;
  padding-right: 0.15rem;
}

.outreach-input {
  min-height: 2.65rem;
  width: 100%;
  border-radius: 8px;
  padding: 0 0.85rem;
  color: white;
  font-size: 0.85rem;
}

.panel-filter-row,
.monitor-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0.6rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding: 0.7rem 0.8rem;
}

.monitor-toolbar {
  border-bottom: 0;
  padding: 0;
}

.monitor-bulk-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.6rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.76rem;
}

.monitor-bulk-row button {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.045);
  padding: 0.42rem 0.62rem;
  color: rgba(224, 231, 255, 0.94);
  font-weight: 800;
}

.monitor-bulk-row button:hover {
  border-color: rgba(79, 172, 254, 0.36);
  color: #7deeff;
}

.matrix-source-pill,
.member-filter-pill {
  border: 1px dashed rgba(34, 197, 94, 0.32);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.12), rgba(245, 158, 11, 0.08));
  color: #a7f3d0;
  padding: 0.68rem 0.78rem;
  font-size: 0.84rem;
  font-weight: 900;
  white-space: nowrap;
}

.member-filter-pill {
  display: inline-flex;
  align-items: center;
  border-style: solid;
  border-color: rgba(79, 172, 254, 0.28);
  background: rgba(0, 242, 254, 0.08);
  color: #7deeff;
}

.terminal-option {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: 0.62rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.62rem 0.7rem;
  cursor: pointer;
}

.terminal-option:hover {
  border-color: rgba(79, 172, 254, 0.3);
  background: rgba(79, 172, 254, 0.08);
}

.matrix-preview-option {
  cursor: default;
}

.matrix-status-dot {
  width: 0.58rem;
  height: 0.58rem;
  margin-top: 0.32rem;
  border-radius: 999px;
  background: #f59e0b;
  box-shadow: 0 0 0 4px rgba(245, 158, 11, 0.1);
}

.matrix-status-dot[data-tone='success'] {
  background: #22c55e;
  box-shadow: 0 0 0 4px rgba(34, 197, 94, 0.1);
}

.config-footer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(190px, 260px);
  gap: 0.75rem;
  flex-shrink: 0;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding: 0.8rem;
}

.listener-meta {
  padding: 0.65rem 0.85rem;
}

.save-rule-btn {
  width: 100%;
}

.lead-stream {
  display: block;
  column-count: 3;
  column-gap: 0.75rem;
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 0.85rem;
}

.outreach-task-panel {
  min-height: 18rem;
}

.outreach-inbox-panel {
  min-height: 34rem;
}

.listener-task-list {
  display: grid;
  align-content: start;
  gap: 0.65rem;
  min-height: 0;
  flex: 1;
  overflow: auto;
  overflow-x: hidden;
  padding: 0.8rem;
}

.listener-task-card {
  display: grid;
  grid-template-columns: 1fr;
  align-items: start;
  gap: 0.75rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(34, 197, 94, 0.06), rgba(244, 114, 182, 0.045)),
    rgba(15, 23, 42, 0.5);
  padding: 0.72rem;
}

.task-main {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 0.62rem;
}

.task-icon {
  display: grid;
  width: 2.1rem;
  height: 2.1rem;
  place-items: center;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.2), rgba(79, 172, 254, 0.14));
  color: #a7f3d0;
  font-weight: 900;
}

.task-main strong,
.task-metrics strong {
  display: block;
  overflow: hidden;
  color: white;
  font-size: 0.86rem;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-main span,
.task-metrics span {
  display: block;
  margin-top: 0.2rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.7rem;
}

.task-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(5.75rem, 1fr));
  gap: 0.45rem;
}

.task-metrics > div {
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  padding: 0.45rem 0.55rem;
}

.task-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-start;
  gap: 0.45rem;
}

.lead-card {
  display: block;
  width: 100%;
  break-inside: avoid;
  margin: 0 0 0.75rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-top-color: rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.05), transparent 30%),
    rgba(15, 23, 42, 0.48);
  padding: 0.8rem;
  text-align: left;
}

.lead-card:hover {
  transform: translateY(-2px);
  border-color: rgba(79, 172, 254, 0.44);
  background:
    linear-gradient(135deg, rgba(0, 242, 254, 0.12), rgba(79, 172, 254, 0.06)),
    rgba(15, 23, 42, 0.62);
  box-shadow: 0 16px 32px rgba(79, 172, 254, 0.14);
}

.lead-card-top {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.65rem;
}

.lead-avatar {
  display: grid;
  width: 2.4rem;
  height: 2.4rem;
  place-items: center;
  border: 1px solid rgba(79, 172, 254, 0.35);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(0, 242, 254, 0.92), rgba(79, 172, 254, 0.9));
  color: #02131f;
  font-weight: 900;
}

.lead-card-top strong {
  display: block;
  overflow: hidden;
  color: white;
  font-size: 0.94rem;
  font-weight: 900;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lead-card-top span,
.lead-card-top time {
  display: block;
  margin-top: 0.2rem;
  overflow: hidden;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lead-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.48rem;
  margin-top: 0.75rem;
}

.lead-meta-grid > div {
  padding: 0.52rem 0.62rem;
}

.lead-message {
  display: -webkit-box;
  min-height: 3.6rem;
  margin: 0.65rem 0 0;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  color: rgba(248, 251, 255, 0.92);
  font-size: 0.82rem;
  line-height: 1.48;
}

.lead-history {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.7rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
}

.history-strip {
  display: grid;
  gap: 0.35rem;
  margin-top: 0.62rem;
}

.history-strip > div {
  display: flex;
  justify-content: space-between;
  gap: 0.6rem;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 0.42rem 0.55rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.72rem;
}

.history-strip strong {
  color: white;
  font-weight: 800;
}

.empty-state,
.empty-mini,
.list-overflow-note {
  display: flex;
  min-height: 8rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  color: rgba(147, 164, 198, 0.92);
  text-align: center;
}

.list-overflow-note {
  min-height: 3.2rem;
  break-inside: avoid;
  column-span: all;
  padding: 0.8rem;
  color: rgba(147, 164, 198, 0.92);
  font-size: 0.78rem;
}

.lead-stream .empty-state {
  break-inside: avoid;
  column-span: all;
}

.empty-state strong {
  color: white;
}

.empty-state span {
  margin-top: 0.45rem;
  font-size: 0.82rem;
}

.empty-mini {
  min-height: 5rem;
  font-size: 0.78rem;
}

@media (max-width: 1500px) {
  .outreach-workspace {
    grid-template-columns: minmax(0, 1fr);
  }

  .outreach-left-stack {
    grid-template-columns: minmax(360px, 0.45fr) minmax(520px, 0.55fr);
  }

  .lead-stream {
    column-count: 2;
  }
}

@media (max-width: 1180px) {
  .outreach-shell {
    height: auto;
    min-height: calc(100vh - 6.25rem);
    overflow: visible;
  }

  .outreach-topbar,
  .outreach-actions {
    align-items: flex-start;
  }

  .outreach-topbar,
  .outreach-workspace {
    grid-template-columns: 1fr;
  }

  .outreach-topbar {
    display: grid;
  }

  .outreach-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .outreach-workspace,
  .outreach-left-stack {
    display: grid;
    grid-template-columns: 1fr;
  }

  .outreach-left-stack {
    grid-template-rows: auto auto;
  }

  .outreach-config-panel {
    grid-row: auto;
  }

  .config-grid {
    grid-template-columns: 1fr;
  }

  .listener-task-card {
    grid-template-columns: 1fr;
  }

  .task-actions {
    justify-content: flex-start;
  }

  .outreach-panel {
    min-height: 26rem;
  }

  .lead-stream {
    column-count: 1;
  }
}

@media (max-width: 720px) {
  .outreach-title {
    font-size: 1.85rem;
  }

  .outreach-stat-grid,
  .resource-default-grid,
  .config-footer {
    grid-template-columns: 1fr;
  }

  .panel-head {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>

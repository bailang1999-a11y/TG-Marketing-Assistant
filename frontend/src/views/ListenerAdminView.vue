<template>
  <div class="page-shell listener-matrix-shell">
    <div class="page-header">
      <div>
        <h1 class="page-title">监听矩阵</h1>
        <p class="page-subtitle">监听账号、监听群、监听代理使用独立数据库，不与账号管理、目标池、网络节点同步。</p>
      </div>
      <div class="page-actions">
        <GlassButton variant="secondary" :loading="refreshingMemberships" @click="refreshListenerMemberships">刷新监听群内状态</GlassButton>
        <GlassButton variant="secondary" :loading="loading" @click="load">刷新</GlassButton>
      </div>
    </div>

    <GlassCard v-if="activeAccountTask" class="membership-task-card">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">监听账号状态检测</h2>
          <p class="mt-1 text-sm text-steel">{{ taskStatusText(activeAccountTask.status) }} · 更新 {{ formatDateTime(activeAccountTask.updated_at) }}</p>
        </div>
        <span class="status-pill" :data-tone="taskStatusTone(activeAccountTask.status)">{{ activeAccountTask.progress || 0 }}%</span>
      </div>
      <div class="progress-track mt-4">
        <div class="progress-fill" :style="{ width: `${activeAccountTask.progress || 0}%` }"></div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-4">
        <div v-for="item in accountTaskSummaryCards" :key="item.label" class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-steel">{{ item.label }}</div>
          <div class="mt-1 text-lg font-black text-white">{{ item.value }}</div>
        </div>
      </div>
    </GlassCard>

    <GlassCard v-if="activeMembershipTask" class="membership-task-card">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">监听群内账号状态刷新</h2>
          <p class="mt-1 text-sm text-steel">{{ taskStatusText(activeMembershipTask.status) }} · 更新 {{ formatDateTime(activeMembershipTask.updated_at) }}</p>
        </div>
        <span class="status-pill" :data-tone="taskStatusTone(activeMembershipTask.status)">{{ activeMembershipTask.progress || 0 }}%</span>
      </div>
      <div class="progress-track mt-4">
        <div class="progress-fill" :style="{ width: `${activeMembershipTask.progress || 0}%` }"></div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-4">
        <div v-for="item in membershipTaskSummaryCards" :key="item.label" class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-steel">{{ item.label }}</div>
          <div class="mt-1 text-lg font-black text-white">{{ item.value }}</div>
        </div>
      </div>
    </GlassCard>

    <GlassCard v-if="activeJoinTask" class="membership-task-card">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">监听号自动加入监听群</h2>
          <p class="mt-1 text-sm text-steel">{{ taskStatusText(activeJoinTask.status) }} · 更新 {{ formatDateTime(activeJoinTask.updated_at) }}</p>
        </div>
        <span class="status-pill" :data-tone="taskStatusTone(activeJoinTask.status)">{{ activeJoinTask.progress || 0 }}%</span>
      </div>
      <div class="progress-track mt-4">
        <div class="progress-fill" :style="{ width: `${activeJoinTask.progress || 0}%` }"></div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-4">
        <div v-for="item in joinTaskSummaryCards" :key="item.label" class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-steel">{{ item.label }}</div>
          <div class="mt-1 text-lg font-black text-white">{{ item.value }}</div>
        </div>
      </div>
    </GlassCard>

    <GlassCard v-if="activeProxyTask" class="membership-task-card">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-bold">监听代理延迟检测</h2>
          <p class="mt-1 text-sm text-steel">{{ taskStatusText(activeProxyTask.status) }} · 更新 {{ formatDateTime(activeProxyTask.updated_at) }}</p>
        </div>
        <span class="status-pill" :data-tone="taskStatusTone(activeProxyTask.status)">{{ activeProxyTask.progress || 0 }}%</span>
      </div>
      <div class="progress-track mt-4">
        <div class="progress-fill" :style="{ width: `${activeProxyTask.progress || 0}%` }"></div>
      </div>
      <div class="mt-4 grid gap-3 sm:grid-cols-4">
        <div v-for="item in proxyTaskSummaryCards" :key="item.label" class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-steel">{{ item.label }}</div>
          <div class="mt-1 text-lg font-black text-white">{{ item.value }}</div>
        </div>
      </div>
    </GlassCard>

    <div class="grid gap-4 md:grid-cols-4">
      <GlassCard v-for="card in cards" :key="card.label" class="metric-card p-4" :data-tone="card.tone">
        <div class="text-sm text-steel">{{ card.label }}</div>
        <div class="mt-2 text-2xl font-black">{{ card.value }}</div>
      </GlassCard>
    </div>

    <div class="listener-action-grid">
      <GlassCard class="upload-panel action-card" data-kind="account">
        <div class="panel-title-row">
          <span class="panel-icon">☎</span>
          <div>
            <h2>上传监听号</h2>
            <p>选择文件、文件夹或 zip 后弹出分组确认。</p>
          </div>
        </div>
        <div class="drop-zone" @click="pickUpload('account')" @dragover.prevent @drop.prevent="dropUpload($event, 'account')">
          <strong>选择文件 / 文件夹 / Zip 路径</strong>
          <span>支持文本列表、账号文件夹、压缩包名称识别。</span>
        </div>
        <div class="panel-mini-grid">
          <span>监听号</span><strong>{{ accounts.length }}</strong>
          <span>分组</span><strong>{{ accountGroups.length }}</strong>
        </div>
      </GlassCard>

      <GlassCard class="upload-panel action-card" data-kind="target">
        <div class="panel-title-row">
          <span class="panel-icon">◎</span>
          <div>
            <h2>导入监听群</h2>
            <p>手动输入 Telegram 群组或频道链接，一行一个。</p>
          </div>
        </div>
        <textarea v-model="targetTextContent" class="manual-import-input" placeholder="https://t.me/example&#10;@groupname"></textarea>
        <GlassButton class="w-full" variant="primary" :disabled="!targetTextContent.trim()" @click="openTextImport('target')">导入监听群</GlassButton>
        <div class="panel-mini-grid">
          <span>监听群</span><strong>{{ targets.length }}</strong>
          <span>分组</span><strong>{{ targetGroups.length }}</strong>
        </div>
      </GlassCard>

      <GlassCard class="upload-panel action-card" data-kind="proxy">
        <div class="panel-title-row">
          <span class="panel-icon">↯</span>
          <div>
            <h2>导入代理</h2>
            <p>手动输入 socks5 / sk5 / http，免前缀按默认协议解析。</p>
          </div>
        </div>
        <textarea v-model="proxyTextContent" class="manual-import-input" placeholder="sk5://1.2.3.4:1080&#10;http://user:pass@5.6.7.8:8080"></textarea>
        <div class="grid grid-cols-2 gap-2">
          <select v-model="proxyProtocol" class="min-h-11 rounded-lg px-3 text-sm">
            <option value="socks5">socks5 / sk5</option>
            <option value="http">http</option>
          </select>
          <label class="auto-assign-box">
            <input v-model="autoAssign" type="checkbox" /> 自动分配
          </label>
        </div>
        <GlassButton class="mt-3 w-full" variant="primary" :disabled="!proxyTextContent.trim()" @click="openTextImport('proxy')">导入代理</GlassButton>
      </GlassCard>

      <GlassCard class="upload-panel action-card" data-kind="assign">
        <div class="panel-title-row">
          <span class="panel-icon">⇄</span>
          <div>
            <h2>代理分配</h2>
            <p>按最少绑定优先均匀分配。</p>
          </div>
        </div>
        <div class="space-y-3">
          <select v-model="accountGroupID" class="min-h-11 w-full rounded-lg px-3 text-sm">
            <option value="">全部监听号</option>
            <option v-for="group in accountGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
          <select v-model="assignProxyGroupID" class="min-h-11 w-full rounded-lg px-3 text-sm">
            <option value="">选择代理分组</option>
            <option v-for="group in proxyGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
          <GlassButton class="w-full" variant="primary" :loading="assigning" :disabled="!assignProxyGroupID" @click="assignProxies">均匀分配代理</GlassButton>
          <div v-if="assignment" class="mini-result">成功 {{ assignment.assigned }}，跳过 {{ assignment.skipped }}</div>
        </div>
      </GlassCard>

      <GlassCard class="upload-panel action-card" data-kind="join">
        <div class="panel-title-row">
          <span class="panel-icon">➕</span>
          <div>
            <h2>自动加监听群</h2>
            <p>按未覆盖监听群优先，配合每日上限和间隔控制风控。</p>
          </div>
        </div>
        <div class="space-y-3">
          <select v-model="joinAccountGroupID" class="min-h-11 w-full rounded-lg px-3 text-sm">
            <option value="">全部监听号</option>
            <option v-for="group in accountGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
          <select v-model="joinTargetGroupID" class="min-h-11 w-full rounded-lg px-3 text-sm">
            <option value="">全部监听群</option>
            <option v-for="group in targetGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
          <div class="grid grid-cols-2 gap-2">
            <label class="limit-field">
              <span>每号每日</span>
              <input v-model.number="joinDailyLimit" min="1" max="200" type="number" />
            </label>
            <label class="limit-field">
              <span>间隔分钟</span>
              <input v-model.number="joinIntervalMinutes" min="1" max="1440" type="number" />
            </label>
          </div>
          <label class="limit-field">
            <span>本次最多加群</span>
            <input v-model.number="joinMaxTargets" min="0" type="number" placeholder="0 表示全部" />
          </label>
          <GlassButton class="w-full" variant="primary" :loading="joiningTargets" :disabled="!accounts.length || !targets.length" @click="createListenerJoinTask">启动自动加群</GlassButton>
        </div>
      </GlassCard>
    </div>

    <input ref="fileInput" class="hidden" type="file" multiple accept=".zip,.txt,.csv,.session,.json" @change="pickFiles" />

    <GlassCard class="list-card">
      <div class="list-toolbar">
        <div>
          <h2>监听账号列表</h2>
          <p>展示独立监听号库状态、出口和监听群加入进度。</p>
        </div>
        <div class="toolbar-actions">
          <input v-model="accountKeyword" class="min-h-10 rounded-lg px-3 text-sm" placeholder="搜索手机号 / 昵称 / IP" />
          <select v-model="accountFilterGroupID" class="min-h-10 rounded-lg px-3 text-sm">
            <option value="">全部监听号</option>
            <option v-for="group in accountGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
          <span class="status-pill" data-tone="info">显示 {{ pagedAccounts.length }} / {{ filteredAccounts.length }}</span>
          <GlassButton variant="ghost" :loading="healthSettingsLoading && healthSettingsMode === 'account'" @click="openHealthSettings('account')">定时设置</GlassButton>
          <GlassButton variant="danger" :disabled="!selectedAccountIDs.length" @click="deleteSelected('account')">删除已选 {{ selectedAccountIDs.length }}</GlassButton>
          <GlassButton variant="secondary" :loading="checkingAccounts" @click="checkAccounts">一键检测监听账号状态</GlassButton>
          <GlassButton variant="danger" :loading="deletingAbnormal" @click="deleteAbnormalAccounts">一键删除异常账号</GlassButton>
        </div>
      </div>
      <div class="table-scroll">
        <table class="matrix-table min-w-[980px]">
          <thead>
            <tr>
              <th class="select-col">
                <label class="matrix-check">
                  <input :checked="allAccountsSelected" type="checkbox" @change="toggleAll('account')" />
                </label>
              </th>
              <th>手机号码</th>
              <th>头像</th>
              <th>账户昵称</th>
              <th>接入方式</th>
              <th>出口 IP</th>
              <th>监听群组</th>
              <th>账号状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in pagedAccounts" :key="item.id" :class="{ selected: selectedAccountIDs.includes(item.id) }">
              <td class="select-col">
                <label class="matrix-check">
                  <input v-model="selectedAccountIDs" :value="item.id" type="checkbox" />
                </label>
              </td>
              <td class="font-semibold">{{ item.phone_display || formatPhone(item.phone) || '-' }}</td>
              <td>
                <div class="listener-avatar">
                  <img v-if="item.avatar_url && !brokenAvatarIDs.includes(item.id)" :src="item.avatar_url" :alt="item.nickname || item.phone" @error="markAvatarBroken(item.id)" />
                  <span v-else>{{ avatarInitial(item) }}</span>
                </div>
              </td>
              <td>{{ item.nickname || '未设置' }}</td>
              <td>{{ accessTypeText(item.access_type) }}</td>
              <td>{{ accountExitIPText(item) }} <span class="text-steel">{{ item.exit_flag }}</span></td>
              <td>
                <div class="count-cell">
                  <strong>{{ item.joined_target_count ?? item.joined_targets ?? 0 }}/{{ item.target_total_count ?? targets.length }}</strong>
                  <span>已加入 / 总数</span>
                </div>
              </td>
              <td><span class="status-pill" :data-tone="accountTone(item)">{{ item.status_text || statusText(item.status) }}</span></td>
              <td><GlassButton variant="danger" size="sm" @click="deleteItem('account', item.id)">删除</GlassButton></td>
            </tr>
            <tr v-if="!filteredAccounts.length"><td colspan="9" class="empty-cell">暂无监听账号</td></tr>
          </tbody>
        </table>
      </div>
      <div v-if="accountPageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
        <span>第 {{ accountPage }} / {{ accountPageCount }} 页</span>
        <GlassButton variant="secondary" size="sm" :disabled="accountPage <= 1" @click="accountPage--">上一页</GlassButton>
        <GlassButton variant="secondary" size="sm" :disabled="accountPage >= accountPageCount" @click="accountPage++">下一页</GlassButton>
      </div>
    </GlassCard>

    <div class="grid flex-1 gap-4 xl:grid-cols-2">
      <GlassCard class="list-card">
        <div class="list-toolbar">
          <div>
            <h2>监听群列表</h2>
            <p>群组和频道也使用监听矩阵独立库。</p>
          </div>
          <div class="toolbar-actions">
            <input v-model="targetKeyword" class="min-h-10 rounded-lg px-3 text-sm" placeholder="搜索群名 / 链接" />
            <select v-model="targetFilterGroupID" class="min-h-10 rounded-lg px-3 text-sm">
              <option value="">全部监听群分组</option>
              <option v-for="group in targetGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
            <span class="status-pill" data-tone="info">显示 {{ pagedTargets.length }} / {{ filteredTargets.length }}</span>
            <GlassButton variant="secondary" :loading="refreshingTargets" @click="refreshTargets">一键刷新群资料</GlassButton>
            <GlassButton variant="danger" :disabled="!selectedTargetIDs.length" @click="deleteSelected('target')">删除已选 {{ selectedTargetIDs.length }}</GlassButton>
          </div>
        </div>
        <div class="table-scroll">
          <table class="matrix-table min-w-[820px]">
            <thead>
              <tr>
                <th class="select-col">
                  <label class="matrix-check">
                    <input :checked="allTargetsSelected" type="checkbox" @change="toggleAll('target')" />
                  </label>
                </th>
                <th>监听群真实名称</th>
                <th>监听群链接</th>
                <th>类型</th>
                <th>成员</th>
                <th>监听群组类别</th>
                <th>操作</th>
            </tr>
          </thead>
          <tbody>
              <tr v-for="item in pagedTargets" :key="item.id" :class="{ selected: selectedTargetIDs.includes(item.id) }">
                <td class="select-col">
                  <label class="matrix-check">
                    <input v-model="selectedTargetIDs" :value="item.id" type="checkbox" />
                  </label>
                </td>
                <td class="font-semibold">{{ item.name || item.identifier }}</td>
                <td class="text-ice">{{ item.identifier }}</td>
                <td>{{ item.type_text || targetTypeText(item.type) }}</td>
                <td>{{ item.size || '-' }}</td>
                <td><span class="status-pill" data-tone="info">{{ item.group_name || groupName(targetGroups, item.group_id) }}</span></td>
                <td><GlassButton variant="danger" size="sm" @click="deleteItem('target', item.id)">删除</GlassButton></td>
              </tr>
              <tr v-if="!filteredTargets.length"><td colspan="7" class="empty-cell">暂无监听群</td></tr>
            </tbody>
          </table>
        </div>
        <div v-if="targetPageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
          <span>第 {{ targetPage }} / {{ targetPageCount }} 页</span>
          <GlassButton variant="secondary" size="sm" :disabled="targetPage <= 1" @click="targetPage--">上一页</GlassButton>
          <GlassButton variant="secondary" size="sm" :disabled="targetPage >= targetPageCount" @click="targetPage++">下一页</GlassButton>
        </div>
      </GlassCard>

      <GlassCard class="list-card">
        <div class="list-toolbar">
          <div>
            <h2>代理列表</h2>
            <p>展示监听矩阵专用代理池，不读取网络节点。</p>
          </div>
          <div class="toolbar-actions">
            <input v-model="proxyKeyword" class="min-h-10 rounded-lg px-3 text-sm" placeholder="搜索 IP / 用户名 / 地区" />
            <select v-model="proxyFilterGroupID" class="min-h-10 rounded-lg px-3 text-sm">
              <option value="">全部代理分组</option>
              <option v-for="group in proxyGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
            <span class="status-pill" data-tone="info">显示 {{ pagedProxies.length }} / {{ filteredProxies.length }}</span>
            <GlassButton variant="ghost" :loading="healthSettingsLoading && healthSettingsMode === 'proxy'" @click="openHealthSettings('proxy')">定时设置</GlassButton>
            <GlassButton variant="secondary" :loading="checkingProxies" @click="checkProxies">一键检测代理延迟</GlassButton>
            <GlassButton variant="danger" :loading="deletingProxyGroup" :disabled="!proxyFilterGroupID" @click="deleteCurrentProxyGroup">删除当前分组</GlassButton>
            <GlassButton variant="danger" :disabled="!selectedProxyIDs.length" @click="deleteSelected('proxy')">删除已选 {{ selectedProxyIDs.length }}</GlassButton>
          </div>
        </div>
        <div class="table-scroll">
          <table class="matrix-table min-w-[1160px]">
            <thead>
              <tr>
                <th class="select-col">
                  <label class="matrix-check">
                    <input :checked="allProxiesSelected" type="checkbox" @change="toggleAll('proxy')" />
                  </label>
                </th>
                <th>IP</th>
                <th>端口</th>
                <th>协议</th>
                <th>出口 IP</th>
                <th>延迟</th>
                <th>出口状态</th>
                <th>Web</th>
                <th>客户端</th>
                <th>地理位置</th>
                <th>绑定终端数</th>
                <th>操作</th>
            </tr>
          </thead>
          <tbody>
              <tr v-for="item in pagedProxies" :key="item.id" :class="{ selected: selectedProxyIDs.includes(item.id) }">
                <td class="select-col">
                  <label class="matrix-check">
                    <input v-model="selectedProxyIDs" :value="item.id" type="checkbox" />
                  </label>
                </td>
                <td class="font-semibold">{{ item.ip }}</td>
                <td>{{ item.port }}</td>
                <td>{{ item.protocol_display || item.protocol }}</td>
                <td>{{ proxyExitIPText(item) }}</td>
                <td>
                  <span class="latency-badge" :data-tone="proxyLatencyTone(item)">
                    <i>{{ proxyLatencyIcon(item) }}</i>
                    {{ proxyLatencyText(item) }}
                  </span>
                </td>
                <td>
                  <span class="latency-badge" :data-tone="proxyExitTone(item)">
                    <i>{{ proxyExitIcon(item) }}</i>
                    {{ proxyExitStatusText(item) }}
                  </span>
                </td>
                <td>
                  <span class="latency-badge" :data-tone="proxyWebTone(item)" :title="item.web_error || '检测 web.telegram.org/k/'">
                    <i>{{ proxyWebIcon(item) }}</i>
                    {{ proxyWebText(item) }}
                  </span>
                </td>
                <td>
                  <span class="latency-badge" :data-tone="proxyTelegramTone(item)" :title="item.telegram_error || '检测 Telegram 客户端 DC 连接'">
                    <i>{{ proxyTelegramIcon(item) }}</i>
                    {{ proxyTelegramText(item) }}
                  </span>
                </td>
                <td>
                  <span class="country-badge">
                    <i>{{ item.flag || '◇' }}</i>
                    {{ proxyCountryText(item) }}
                  </span>
                </td>
                <td>
                  <div class="proxy-bind">
                    <span>{{ item.bound_display || proxyBoundDisplay(item) }}</span>
                    <i :style="{ width: `${proxyAssignmentPercent(item)}%` }"></i>
                  </div>
                </td>
                <td><GlassButton variant="danger" size="sm" @click="deleteItem('proxy', item.id)">删除</GlassButton></td>
              </tr>
              <tr v-if="!filteredProxies.length"><td colspan="12" class="empty-cell">暂无监听代理</td></tr>
            </tbody>
          </table>
        </div>
        <div v-if="proxyPageCount > 1" class="mt-4 flex flex-wrap items-center justify-end gap-2 text-sm text-steel">
          <span>第 {{ proxyPage }} / {{ proxyPageCount }} 页</span>
          <GlassButton variant="secondary" size="sm" :disabled="proxyPage <= 1" @click="proxyPage--">上一页</GlassButton>
          <GlassButton variant="secondary" size="sm" :disabled="proxyPage >= proxyPageCount" @click="proxyPage++">下一页</GlassButton>
        </div>
      </GlassCard>
    </div>

    <Teleport to="body">
      <Transition name="dialog">
        <div v-if="pendingImport" class="modal-backdrop" @click.self="pendingImport = null">
          <div class="modal-card p-6 lg:p-7">
            <div class="panel-title-row mb-5">
              <span class="panel-icon">{{ pendingImport.icon }}</span>
              <div>
                <h2 class="text-xl font-black text-white">{{ pendingImport.title }}</h2>
                <p class="text-sm text-steel">识别到 {{ pendingImport.countText || `${pendingImport.count} 条内容` }}，选择导入分组后直接入库。</p>
              </div>
            </div>
            <div class="space-y-3">
              <select v-model="pendingGroupID" class="min-h-11 w-full rounded-lg px-3 text-sm" @change="handlePendingGroupChange">
                <option value="">选择已有分组</option>
                <option v-for="group in pendingGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
              </select>
              <input v-model="pendingGroupName" class="min-h-11 w-full rounded-lg px-3 text-sm" :disabled="!!pendingGroupID" :placeholder="`或新建${pendingImport.groupLabel}`" />
              <p class="rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-steel">来源：{{ pendingImport.source }}</p>
            </div>
            <div class="mt-7 flex flex-wrap justify-end gap-3">
              <GlassButton variant="ghost" @click="pendingImport = null">取消</GlassButton>
              <GlassButton variant="primary" :loading="pendingSaving" @click="confirmPendingImport">确认导入</GlassButton>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="dialog">
        <div v-if="healthSettingsOpen" class="modal-backdrop" @click.self="closeHealthSettings">
          <div class="modal-card health-settings-dialog p-6 lg:p-7">
            <div class="panel-title-row health-settings-header">
              <span class="panel-icon">⌁</span>
              <div>
                <h2 class="text-xl font-black text-white">{{ healthSettingsTitle }}</h2>
                <p class="text-sm text-steel">保存后调度器会自动创建对应检测任务，任务中心和日志中心会显示进度与结果。</p>
              </div>
            </div>
            <div class="health-settings-form">
              <label v-if="healthSettingsMode === 'account'" class="health-toggle-row">
                <div class="min-w-0">
                  <div class="font-semibold">自动刷新监听账号状态</div>
                  <div class="mt-1 text-sm text-steel">周期性创建“一键检测监听账号状态”任务</div>
                </div>
                <input v-model="healthSettingsForm.autoAccountCheckEnabled" type="checkbox" class="h-5 w-5" />
              </label>
              <label v-if="healthSettingsMode === 'account'" class="health-field">
                <span>账号状态检测周期（分钟）</span>
                <input v-model.number="healthSettingsForm.accountCheckIntervalMinutes" type="number" min="5" max="1440" />
              </label>

              <label v-if="healthSettingsMode === 'proxy'" class="health-toggle-row">
                <div class="min-w-0">
                  <div class="font-semibold">自动刷新代理列表延迟</div>
                  <div class="mt-1 text-sm text-steel">周期性检测代理入口、真实出口、Web 和客户端连通性</div>
                </div>
                <input v-model="healthSettingsForm.autoProxyCheckEnabled" type="checkbox" class="h-5 w-5" />
              </label>
              <label v-if="healthSettingsMode === 'proxy'" class="health-field">
                <span>代理列表检测周期（分钟）</span>
                <input v-model.number="healthSettingsForm.proxyCheckIntervalMinutes" type="number" min="5" max="1440" />
              </label>

              <label class="health-field">
                <span>无消息提醒阈值（分钟）</span>
                <input v-model.number="healthSettingsForm.silenceAlertMinutes" type="number" min="1" max="1440" />
              </label>
            </div>
            <div class="health-settings-actions">
              <GlassButton variant="ghost" @click="closeHealthSettings">取消</GlassButton>
              <GlassButton variant="primary" :loading="healthSettingsSaving" @click="saveHealthSettings">保存定时设置</GlassButton>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <p v-if="message" class="rounded-lg border border-neon/30 bg-neon/10 px-4 py-3 text-sm text-neon">{{ message }}</p>
    <p v-if="error" class="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import GlassButton from '../components/GlassButton.vue'
import GlassCard from '../components/GlassCard.vue'
import { useUiStore } from '../stores/ui'
import { api, type Group, type ListenerAccount, type ListenerAdminOverview, type ListenerProxy, type ListenerProxyAssignment, type ListenerTarget, type SystemSettings, type Task } from '../api/client'

type ImportKind = 'account' | 'target' | 'proxy'
type HealthSettingsMode = 'account' | 'proxy'
type PendingImport = {
  kind: ImportKind
  title: string
  icon: string
  groupLabel: string
  content: string
  count: number
  countText?: string
  source: string
  files?: File[]
}
type FileSystemFileHandleLike = {
  kind: 'file'
  name: string
  getFile: () => Promise<File>
}
type FileSystemDirectoryHandleLike = {
  kind: 'directory'
  name: string
  entries: () => AsyncIterableIterator<[string, FileSystemFileHandleLike | FileSystemDirectoryHandleLike]>
}

const ui = useUiStore()
const overview = ref<ListenerAdminOverview | null>(null)
const accountGroups = ref<Group[]>([])
const targetGroups = ref<Group[]>([])
const proxyGroups = ref<Group[]>([])
const accounts = ref<ListenerAccount[]>([])
const brokenAvatarIDs = ref<string[]>([])
const targets = ref<ListenerTarget[]>([])
const proxies = ref<ListenerProxy[]>([])
const targetTextContent = ref('')
const proxyTextContent = ref('')
const assignProxyGroupID = ref('')
const accountGroupID = ref('')
const joinAccountGroupID = ref('')
const joinTargetGroupID = ref('')
const accountFilterGroupID = ref('')
const targetFilterGroupID = ref('')
const proxyFilterGroupID = ref('')
const accountKeyword = ref('')
const targetKeyword = ref('')
const proxyKeyword = ref('')
const accountPage = ref(1)
const targetPage = ref(1)
const proxyPage = ref(1)
const listPageSize = 10
const proxyProtocol = ref('socks5')
const autoAssign = ref(true)
const joinDailyLimit = ref(5)
const joinIntervalMinutes = ref(30)
const joinMaxTargets = ref(0)
const assignment = ref<ListenerProxyAssignment | null>(null)
const loading = ref(false)
const pendingSaving = ref(false)
const checkingAccounts = ref(false)
const checkingProxies = ref(false)
const refreshingTargets = ref(false)
const refreshingMemberships = ref(false)
const deletingAbnormal = ref(false)
const deletingProxyGroup = ref(false)
const assigning = ref(false)
const joiningTargets = ref(false)
const error = ref('')
const message = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const activePickKind = ref<ImportKind>('account')
const pendingImport = ref<PendingImport | null>(null)
const pendingGroupID = ref('')
const pendingGroupName = ref('')
const selectedAccountIDs = ref<string[]>([])
const selectedTargetIDs = ref<string[]>([])
const selectedProxyIDs = ref<string[]>([])
const healthSettingsOpen = ref(false)
const healthSettingsMode = ref<HealthSettingsMode>('account')
const healthSettingsLoading = ref(false)
const healthSettingsSaving = ref(false)
const healthSettingsPayload = ref<SystemSettings | null>(null)
const healthSettingsForm = reactive({
  autoAccountCheckEnabled: true,
  accountCheckIntervalMinutes: 60,
  autoProxyCheckEnabled: true,
  proxyCheckIntervalMinutes: 60,
  silenceAlertMinutes: 15
})
const activeAccountTask = ref<Task | null>(null)
const activeMembershipTask = ref<Task | null>(null)
const activeJoinTask = ref<Task | null>(null)
const activeProxyTask = ref<Task | null>(null)
let accountTaskTimer: ReturnType<typeof window.setInterval> | null = null
let membershipTaskTimer: ReturnType<typeof window.setInterval> | null = null
let joinTaskTimer: ReturnType<typeof window.setInterval> | null = null
let proxyTaskTimer: ReturnType<typeof window.setInterval> | null = null
const batchDeleteConcurrency = 8
const defaultProxyAssignmentLimit = 5

const cards = computed(() => [
  { label: '监听号', value: overview.value?.account_count || 0, tone: 'info' },
  { label: '监听群', value: overview.value?.target_count || 0, tone: 'success' },
  { label: '监听代理', value: overview.value?.proxy_count || 0, tone: 'warning' },
  { label: '已分配出口', value: overview.value?.assigned_count || 0, tone: 'danger' }
])
const accountTaskSummaryCards = computed(() => {
  const summary = activeAccountTask.value?.summary || {}
  return [
    { label: '总数', value: numericSummary(summary, 'total') },
    { label: '正常', value: numericSummary(summary, 'normal') },
    { label: '会话有效', value: numericSummary(summary, 'offline') },
    { label: '异常', value: numericSummary(summary, 'abnormal') }
  ]
})
const membershipTaskSummaryCards = computed(() => {
  const summary = activeMembershipTask.value?.summary || {}
  return [
    { label: '总记录', value: numericSummary(summary, 'total') },
    { label: '仍有效', value: numericSummary(summary, 'active') },
    { label: '已移除', value: numericSummary(summary, 'removed') },
    { label: '待复查', value: numericSummary(summary, 'skipped') + numericSummary(summary, 'failed') }
  ]
})
const joinTaskSummaryCards = computed(() => {
  const summary = activeJoinTask.value?.summary || {}
  return [
    { label: '目标数', value: numericSummary(summary, 'total') },
    { label: '成功', value: numericSummary(summary, 'success') },
    { label: '失败', value: numericSummary(summary, 'failed') },
    { label: '跳过', value: numericSummary(summary, 'skipped') }
  ]
})
const proxyTaskSummaryCards = computed(() => {
  const summary = activeProxyTask.value?.summary || {}
  return [
    { label: '总数', value: numericSummary(summary, 'total') },
    { label: '正常', value: numericSummary(summary, 'normal') },
    { label: '失败', value: numericSummary(summary, 'failed') },
    { label: '超时', value: numericSummary(summary, 'timeout') }
  ]
})

const healthSettingsTitle = computed(() => healthSettingsMode.value === 'account' ? '监听账号定时检测设置' : '代理列表定时检测设置')

const filteredAccounts = computed(() => {
  const keyword = normalizeKeyword(accountKeyword.value)
  return accounts.value.filter((item) => {
    if (accountFilterGroupID.value && item.group_id !== accountFilterGroupID.value) return false
    if (!keyword) return true
    return [
      item.phone,
      item.phone_display,
      item.nickname,
      item.exit_ip,
      item.exit_flag,
      item.status_text,
      item.risk_status
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const filteredTargets = computed(() => {
  const keyword = normalizeKeyword(targetKeyword.value)
  return targets.value.filter((item) => {
    if (targetFilterGroupID.value && item.group_id !== targetFilterGroupID.value) return false
    if (!keyword) return true
    return [
      item.name,
      item.identifier,
      item.type_text,
      targetTypeText(item.type),
      groupName(targetGroups.value, item.group_id)
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const filteredProxies = computed(() => {
  const keyword = normalizeKeyword(proxyKeyword.value)
  return proxies.value.filter((item) => {
    if (proxyFilterGroupID.value && item.group_id !== proxyFilterGroupID.value) return false
    if (!keyword) return true
    return [
      item.ip,
      item.port,
      item.protocol_display,
      item.protocol,
      item.username,
      item.country,
      item.flag,
      item.location_display
    ].some((value) => normalizeKeyword(value).includes(keyword))
  })
})
const accountPageCount = computed(() => Math.max(1, Math.ceil(filteredAccounts.value.length / listPageSize)))
const targetPageCount = computed(() => Math.max(1, Math.ceil(filteredTargets.value.length / listPageSize)))
const proxyPageCount = computed(() => Math.max(1, Math.ceil(filteredProxies.value.length / listPageSize)))
const pagedAccounts = computed(() => pageSlice(filteredAccounts.value, accountPage.value))
const pagedTargets = computed(() => pageSlice(filteredTargets.value, targetPage.value))
const pagedProxies = computed(() => pageSlice(filteredProxies.value, proxyPage.value))
const allAccountsSelected = computed(() => pagedAccounts.value.length > 0 && pagedAccounts.value.every((item) => selectedAccountIDs.value.includes(item.id)))
const allTargetsSelected = computed(() => pagedTargets.value.length > 0 && pagedTargets.value.every((item) => selectedTargetIDs.value.includes(item.id)))
const allProxiesSelected = computed(() => pagedProxies.value.length > 0 && pagedProxies.value.every((item) => selectedProxyIDs.value.includes(item.id)))
const pendingGroups = computed(() => {
  if (!pendingImport.value) return []
  if (pendingImport.value.kind === 'account') return accountGroups.value
  if (pendingImport.value.kind === 'target') return targetGroups.value
  return proxyGroups.value
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [overviewData, accountGroupData, targetGroupData, proxyGroupData, accountData, targetData, proxyData] = await Promise.all([
      api.listenerAdminOverview(),
      api.groups('listener_account'),
      api.groups('listener_target'),
      api.groups('listener_proxy'),
      api.listenerAccounts(),
      api.listenerTargets(),
      api.listenerProxies()
    ])
    overview.value = overviewData
    accountGroups.value = accountGroupData
    targetGroups.value = targetGroupData
    proxyGroups.value = proxyGroupData
    accounts.value = accountData
    targets.value = targetData
    proxies.value = proxyData
    pruneSelection()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function pruneSelection() {
  const accountIDs = new Set(accounts.value.map((item) => item.id))
  const targetIDs = new Set(targets.value.map((item) => item.id))
  const proxyIDs = new Set(proxies.value.map((item) => item.id))
  selectedAccountIDs.value = selectedAccountIDs.value.filter((id) => accountIDs.has(id))
  brokenAvatarIDs.value = brokenAvatarIDs.value.filter((id) => accountIDs.has(id))
  selectedTargetIDs.value = selectedTargetIDs.value.filter((id) => targetIDs.has(id))
  selectedProxyIDs.value = selectedProxyIDs.value.filter((id) => proxyIDs.has(id))
}

async function pickUpload(kind: ImportKind) {
  activePickKind.value = kind
  const picker = window as Window & { showDirectoryPicker?: () => Promise<FileSystemDirectoryHandleLike> }
  if (picker.showDirectoryPicker) {
    try {
      const directory = await picker.showDirectoryPicker()
      const selected = await readDirectoryHandle(directory, directory.name)
      await prepareImport(selected, kind, directory.name)
      return
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      ui.toast({ title: '读取文件夹失败', message: err instanceof Error ? err.message : '请改用文件选择或拖拽上传。', tone: 'error' })
    }
  }
  fileInput.value?.click()
}

async function pickFiles(event: Event) {
  const input = event.target as HTMLInputElement
  await prepareImport(Array.from(input.files || []), activePickKind.value, '本地选择')
  input.value = ''
}

async function dropUpload(event: DragEvent, kind: ImportKind) {
  await prepareImport(Array.from(event.dataTransfer?.files || []), kind, '拖拽内容')
}

async function readDirectoryHandle(directory: FileSystemDirectoryHandleLike, rootPath: string): Promise<File[]> {
  const collected: File[] = []
  for await (const [, handle] of directory.entries()) {
    const path = `${rootPath}/${handle.name}`
    if (handle.kind === 'file') {
      const file = await handle.getFile()
      collected.push(attachRelativePath(file, path))
    } else {
      collected.push(...(await readDirectoryHandle(handle, path)))
    }
  }
  return collected
}

function attachRelativePath(file: File, path: string) {
  Object.defineProperty(file, 'webkitRelativePath', {
    configurable: true,
    value: path
  })
  return file
}

async function prepareImport(files: File[], kind: ImportKind, source: string) {
  const unique = dedupeFiles(files)
  if (!unique.length) {
    ui.toast({ title: '未识别到内容', message: '请选择文件、文件夹内容或 zip。', tone: 'warning' })
    return
  }
  if (kind === 'account') {
    const names = extractAccountFolderNames(unique)
    const hasZip = unique.some((file) => /\.zip$/i.test(filePath(file)))
    const meta = importMeta(kind)
    pendingImport.value = {
      ...meta,
      kind,
      content: names.join('\n'),
      count: names.length || unique.length,
      countText: hasZip ? `待解压解析，${unique.length} 个文件项` : `${names.length || unique.length} 个账号线索`,
      source: `${source} · ${unique.length} 个文件项`,
      files: unique
    }
    pendingGroupID.value = ''
    pendingGroupName.value = ''
    return
  }
  const content = await buildImportContent(unique, kind)
  const lines = content.split('\n').map((line) => line.trim()).filter(Boolean)
  if (!lines.length) {
    ui.toast({ title: '内容为空', message: '没有读到可导入的监听数据。', tone: 'warning' })
    return
  }
  const meta = importMeta(kind)
  pendingImport.value = { ...meta, kind, content: lines.join('\n'), count: lines.length, source: `${source} · ${unique.length} 个文件项` }
  pendingGroupID.value = ''
  pendingGroupName.value = ''
}

function openTextImport(kind: 'target' | 'proxy') {
  const content = kind === 'target' ? targetTextContent.value : proxyTextContent.value
  const lines = content.split('\n').map((line) => line.trim()).filter(Boolean)
  if (!lines.length) {
    ui.toast({ title: '内容为空', message: kind === 'target' ? '请输入监听群链接。' : '请输入代理。', tone: 'warning' })
    return
  }
  const meta = importMeta(kind)
  pendingImport.value = { ...meta, kind, content: lines.join('\n'), count: lines.length, source: '手动输入' }
  pendingGroupID.value = ''
  pendingGroupName.value = ''
}

function handlePendingGroupChange() {
  if (pendingGroupID.value) pendingGroupName.value = ''
}

async function buildImportContent(files: File[], kind: ImportKind) {
  if (kind === 'account') {
    return extractAccountFolderNames(files).join('\n')
  }
  const lines: string[] = []
  for (const file of files) {
    const text = await file.text().catch(() => '')
    if (text.trim()) lines.push(...text.split('\n'))
  }
  return lines.map((line) => line.trim()).filter(Boolean).join('\n')
}

function extractAccountFolderNames(files: File[]) {
  const names = new Set<string>()
  for (const file of files) {
    const path = filePath(file)
    const parts = path.split('/').filter(Boolean)
    let candidate = ''
    if (parts.length >= 3) {
      candidate = parts[1]
    } else if (parts.length === 2) {
      candidate = parts[0]
    } else {
      candidate = inferAccountName(path)
    }
    candidate = cleanupAccountFolderName(candidate)
    if (candidate) names.add(candidate)
  }
  return [...names]
}

function cleanupAccountFolderName(value: string) {
  return value.replace(/\.(zip|session|json|txt|csv)$/i, '').replace(/_/g, ' ').trim()
}

function inferAccountName(path: string) {
  const parts = path.split('/').filter(Boolean)
  const candidate = parts.length > 1 ? parts[parts.length - 2] : parts[0]
  return cleanupAccountFolderName(candidate)
}

function filePath(file: File) {
  return (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
}

function dedupeFiles(files: File[]) {
  const seen = new Set<string>()
  return files.filter((file) => {
    const key = `${filePath(file)}:${file.size}:${file.lastModified}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

function importMeta(kind: ImportKind) {
  if (kind === 'account') return { title: '确认导入监听号', icon: '☎', groupLabel: '监听号分组' }
  if (kind === 'target') return { title: '确认导入监听群', icon: '◎', groupLabel: '监听群分组' }
  return { title: '确认导入代理', icon: '↯', groupLabel: '监听代理分组' }
}

async function confirmPendingImport() {
  if (!pendingImport.value) return
  if (pendingGroupID.value && pendingGroupName.value.trim()) {
    ui.toast({ title: '分组冲突', message: '请选择已有分组或填写新分组，不能同时使用。', tone: 'warning' })
    return
  }
  pendingSaving.value = true
  message.value = ''
  error.value = ''
  try {
    const groupPayload = {
      group_id: pendingGroupID.value,
      new_group_name: pendingGroupID.value ? '' : pendingGroupName.value.trim() || pendingImport.value.groupLabel.replace('分组', '')
    }
    if (pendingImport.value.kind === 'account') {
      const result = pendingImport.value.files?.length
        ? await api.importListenerAccountFiles({ files: pendingImport.value.files, ...groupPayload })
        : await api.importListenerAccounts({ content: pendingImport.value.content, ...groupPayload })
      message.value = `监听号导入完成：成功 ${result.success}，重复 ${result.duplicate}，失败 ${result.failed}`
      if (result.assignment) {
        assignment.value = result.assignment
        message.value += `；自动分配代理 ${result.assignment.assigned} 个`
      } else if (result.assignment_error) {
        message.value += `；自动分配代理失败：${result.assignment_error}`
      }
      showImportToast('监听号', result.success, result.duplicate, result.failed)
    } else if (pendingImport.value.kind === 'target') {
      const result = await api.importListenerTargets({ content: pendingImport.value.content, ...groupPayload })
      message.value = `监听群导入完成：成功 ${result.success}，重复 ${result.duplicate}，失败 ${result.failed}`
      targetTextContent.value = ''
      showImportToast('监听群', result.success, result.duplicate, result.failed)
    } else {
      const result = await api.importListenerProxies({
        content: pendingImport.value.content,
        default_protocol: proxyProtocol.value,
        ...groupPayload,
        account_group_id: accountGroupID.value,
        assign_to_accounts: autoAssign.value
      })
      assignment.value = result.assignment
      message.value = `监听代理导入完成：成功 ${result.import.success}，重复 ${result.import.duplicate}，失败 ${result.import.failed}`
      if (result.assignment_error) {
        message.value += `；自动分配失败：${result.assignment_error}`
      }
      proxyTextContent.value = ''
      showImportToast('监听代理', result.import.success, result.import.duplicate, result.import.failed, result.assignment_error)
    }
    pendingImport.value = null
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '导入失败'
    ui.toast({ title: '导入失败', message: error.value, tone: 'error', duration: 5200 })
  } finally {
    pendingSaving.value = false
  }
}

function showImportToast(label: string, success: number, duplicate: number, failed: number, warning = '') {
  ui.toast({
    title: `${label}导入完成`,
    message: warning || `成功 ${success}，重复 ${duplicate}，失败 ${failed}，实际导入 ${success}`,
    tone: failed > 0 || warning ? 'warning' : 'success',
    duration: 5200
  })
}

async function openHealthSettings(mode: HealthSettingsMode) {
  healthSettingsMode.value = mode
  healthSettingsLoading.value = true
  try {
    const settings = await api.systemSettings()
    healthSettingsPayload.value = settings
    applyHealthSettings(settings)
    healthSettingsOpen.value = true
  } catch (err) {
    ui.toast({
      title: '读取定时设置失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    healthSettingsLoading.value = false
  }
}

function closeHealthSettings() {
  if (healthSettingsSaving.value) return
  healthSettingsOpen.value = false
}

function applyHealthSettings(settings: SystemSettings) {
  healthSettingsForm.autoAccountCheckEnabled = settings.listener_health?.auto_account_check_enabled ?? true
  healthSettingsForm.accountCheckIntervalMinutes = settings.listener_health?.account_check_interval_minutes ?? 60
  healthSettingsForm.autoProxyCheckEnabled = settings.listener_health?.auto_proxy_check_enabled ?? true
  healthSettingsForm.proxyCheckIntervalMinutes = settings.listener_health?.proxy_check_interval_minutes ?? 60
  healthSettingsForm.silenceAlertMinutes = settings.listener_health?.silence_alert_minutes ?? 15
}

async function saveHealthSettings() {
  const settings = healthSettingsPayload.value
  if (!settings) return
  healthSettingsSaving.value = true
  try {
    const payload = {
      security: { ...settings.security },
      frequency: { ...settings.frequency },
      listener_health: {
        auto_account_check_enabled: healthSettingsForm.autoAccountCheckEnabled,
        account_check_interval_minutes: boundedNumber(healthSettingsForm.accountCheckIntervalMinutes, 5, 1440, 60),
        auto_proxy_check_enabled: healthSettingsForm.autoProxyCheckEnabled,
        proxy_check_interval_minutes: boundedNumber(healthSettingsForm.proxyCheckIntervalMinutes, 5, 1440, 60),
        silence_alert_minutes: boundedNumber(healthSettingsForm.silenceAlertMinutes, 1, 1440, 15)
      },
      audit: { ...settings.audit },
      adapter: { ...settings.adapter },
      risk_control: { ...settings.risk_control }
    }
    const saved = await api.updateSystemSettings(payload)
    healthSettingsPayload.value = saved
    applyHealthSettings(saved)
    healthSettingsOpen.value = false
    ui.toast({
      title: '定时设置已保存',
      message: `监听账号 ${payload.listener_health.account_check_interval_minutes} 分钟，代理列表 ${payload.listener_health.proxy_check_interval_minutes} 分钟`,
      tone: 'success'
    })
  } catch (err) {
    ui.toast({
      title: '保存定时设置失败',
      message: err instanceof Error ? err.message : '请求失败',
      tone: 'error'
    })
  } finally {
    healthSettingsSaving.value = false
  }
}

async function assignProxies() {
  assigning.value = true
  assignment.value = null
  message.value = ''
  error.value = ''
  try {
    assignment.value = await api.assignListenerProxies({ proxy_group_id: assignProxyGroupID.value, account_group_id: accountGroupID.value })
    message.value = `代理分配完成：成功 ${assignment.value.assigned}，跳过 ${assignment.value.skipped}`
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '代理分配失败'
  } finally {
    assigning.value = false
  }
}

async function createListenerJoinTask() {
  joiningTargets.value = true
  message.value = ''
  error.value = ''
  try {
    const res = await api.createListenerJoinTargetsTask({
      account_scope: joinAccountGroupID.value ? 'group' : 'all',
      account_group_id: joinAccountGroupID.value,
      target_scope: joinTargetGroupID.value ? 'group' : 'all',
      target_group_id: joinTargetGroupID.value,
      daily_limit: boundedNumber(joinDailyLimit.value, 1, 200, 5),
      interval_minutes: boundedNumber(joinIntervalMinutes.value, 1, 1440, 30),
      max_joins: Math.max(0, Number(joinMaxTargets.value) || 0),
      prefer_uncovered: true
    })
    trackJoinTask(res.task)
    ui.toast({
      title: '自动加群已启动',
      message: `任务 ${res.task.id} 会优先加入还没有监听号覆盖的监听群。`,
      tone: 'success'
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '启动自动加群失败'
  } finally {
    joiningTargets.value = false
  }
}

async function checkAccounts() {
  const accepted = await ui.confirm({
    title: '开始检测监听账号',
    message: '检测会在后台任务中执行，过程会写入任务模块和日志中心。任务完成后无论你在哪个界面都会弹出提示。',
    confirmText: '开始检测',
    cancelText: '取消',
    tone: 'info'
  })
  if (!accepted) return
  checkingAccounts.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await api.checkListenerAccounts({ group_id: accountFilterGroupID.value })
    trackAccountTask(result.task)
    message.value = `监听账号检测任务已创建，正在后台运行。`
    ui.toast({
      title: '监听账号检测已启动',
      message: `任务 ${result.task.id} 已进入任务模块，日志中心会记录每个监听号的检测结果。`,
      tone: 'success',
      duration: 6200
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '检测监听账号失败'
    checkingAccounts.value = false
    ui.toast({ title: '监听账号检测启动失败', message: error.value, tone: 'error', duration: 5200 })
  }
}

async function refreshTargets() {
  refreshingTargets.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await api.refreshListenerTargets({ group_id: targetFilterGroupID.value })
    message.value = `监听群资料刷新任务已启动：总数 ${result.summary.total}，请到任务模块查看进度、日志和失败明细`
    ui.toast({ title: '监听群资料刷新已启动', message: '任务模块和日志中心会持续显示进度。', tone: 'success' })
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '刷新监听群资料失败'
  } finally {
    refreshingTargets.value = false
  }
}

async function refreshListenerMemberships() {
  refreshingMemberships.value = true
  message.value = ''
  error.value = ''
  try {
    const res = await api.refreshTargetMemberships({
      account_kind: 'listener',
      target_scope: 'all'
    })
    trackMembershipTask(res.task)
    ui.toast({
      title: '监听群状态刷新已启动',
      message: `任务 ${res.task.id} 正在校验监听账号是否仍在群内。`,
      tone: 'success'
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '刷新监听群内状态失败'
    refreshingMemberships.value = false
  }
}

function trackMembershipTask(task: Task) {
  activeMembershipTask.value = task
  refreshingMemberships.value = true
  if (membershipTaskTimer) window.clearInterval(membershipTaskTimer)
  membershipTaskTimer = window.setInterval(pollMembershipTask, 2500)
}

function trackAccountTask(task: Task) {
  activeAccountTask.value = task
  checkingAccounts.value = true
  ui.trackTask({ id: task.id, title: '监听账号状态检测', startedAt: Date.now() })
  if (accountTaskTimer) window.clearInterval(accountTaskTimer)
  accountTaskTimer = window.setInterval(pollAccountTask, 2500)
}

function trackJoinTask(task: Task) {
  activeJoinTask.value = task
  if (joinTaskTimer) window.clearInterval(joinTaskTimer)
  joinTaskTimer = window.setInterval(pollJoinTask, 2500)
}

function trackProxyTask(task: Task) {
  activeProxyTask.value = task
  checkingProxies.value = true
  ui.trackTask({ id: task.id, title: '代理延迟检测', startedAt: Date.now() })
  if (proxyTaskTimer) window.clearInterval(proxyTaskTimer)
  proxyTaskTimer = window.setInterval(pollProxyTask, 2500)
}

async function pollAccountTask() {
  const taskID = activeAccountTask.value?.id
  if (!taskID) {
    stopAccountTaskPolling()
    return
  }
  try {
    const tasks = await api.tasks({ type: 'listener_account_check', limit: 50 })
    const next = tasks.find((task) => task.id === taskID)
    if (next) activeAccountTask.value = next
    if (next && isTaskFinished(next.status)) {
      stopAccountTaskPolling()
      await load()
      if (ui.trackedTasks.some((task) => task.id === next.id)) {
        ui.untrackTask(next.id)
        ui.toast({
          title: '监听账号检测完成',
          message: accountTaskDoneMessage(next),
          tone: next.status === 'failed' ? 'error' : next.status === 'partial_success' ? 'warning' : 'success',
          duration: 7000
        })
      }
    }
  } catch (err) {
    stopAccountTaskPolling()
    error.value = err instanceof Error ? err.message : '读取监听账号检测进度失败'
  }
}

function stopAccountTaskPolling() {
  if (accountTaskTimer) {
    window.clearInterval(accountTaskTimer)
    accountTaskTimer = null
  }
  checkingAccounts.value = false
}

async function pollMembershipTask() {
  const taskID = activeMembershipTask.value?.id
  if (!taskID) {
    stopMembershipTaskPolling()
    return
  }
  try {
    const tasks = await api.tasks({ type: 'target_membership_refresh', limit: 50 })
    const next = tasks.find((task) => task.id === taskID)
    if (next) activeMembershipTask.value = next
    if (next && isTaskFinished(next.status)) {
      stopMembershipTaskPolling()
      await load()
      ui.toast({
        title: '监听群状态刷新完成',
        message: membershipTaskDoneMessage(next),
        tone: next.status === 'failed' ? 'error' : 'success'
      })
    }
  } catch (err) {
    stopMembershipTaskPolling()
    error.value = err instanceof Error ? err.message : '读取监听群状态刷新进度失败'
  }
}

function stopMembershipTaskPolling() {
  if (membershipTaskTimer) {
    window.clearInterval(membershipTaskTimer)
    membershipTaskTimer = null
  }
  refreshingMemberships.value = false
}

async function pollJoinTask() {
  const taskID = activeJoinTask.value?.id
  if (!taskID) {
    stopJoinTaskPolling()
    return
  }
  try {
    const tasks = await api.tasks({ type: 'listener_join_targets', limit: 50 })
    const next = tasks.find((task) => task.id === taskID)
    if (next) activeJoinTask.value = next
    if (next && isTaskFinished(next.status)) {
      stopJoinTaskPolling()
      await load()
      ui.toast({
        title: '自动加群完成',
        message: joinTaskDoneMessage(next),
        tone: next.status === 'failed' ? 'error' : 'success'
      })
    }
  } catch (err) {
    stopJoinTaskPolling()
    error.value = err instanceof Error ? err.message : '读取自动加群进度失败'
  }
}

function stopJoinTaskPolling() {
  if (joinTaskTimer) {
    window.clearInterval(joinTaskTimer)
    joinTaskTimer = null
  }
  joiningTargets.value = false
}

async function pollProxyTask() {
  const taskID = activeProxyTask.value?.id
  if (!taskID) {
    stopProxyTaskPolling()
    return
  }
  try {
    const tasks = await api.tasks({ type: 'listener_proxy_check', limit: 50 })
    const next = tasks.find((task) => task.id === taskID)
    if (next) activeProxyTask.value = next
    if (next && isTaskFinished(next.status)) {
      stopProxyTaskPolling()
      await load()
      if (ui.trackedTasks.some((task) => task.id === next.id)) {
        ui.untrackTask(next.id)
        ui.toast({
          title: '代理延迟检测完成',
          message: proxyTaskDoneMessage(next),
          tone: next.status === 'failed' ? 'error' : next.status === 'partial_success' ? 'warning' : 'success',
          duration: 7000
        })
      }
    }
  } catch (err) {
    stopProxyTaskPolling()
    error.value = err instanceof Error ? err.message : '读取代理检测进度失败'
  }
}

function stopProxyTaskPolling() {
  if (proxyTaskTimer) {
    window.clearInterval(proxyTaskTimer)
    proxyTaskTimer = null
  }
  checkingProxies.value = false
}

async function checkProxies() {
  const accepted = await ui.confirm({
    title: '开始检测代理延迟',
    message: '检测会在后台任务中执行，过程会写入任务模块和日志中心。任务完成后无论你在哪个界面都会弹出提示。',
    confirmText: '开始检测',
    cancelText: '取消',
    tone: 'info'
  })
  if (!accepted) return
  checkingProxies.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await api.checkListenerProxies({ group_id: proxyFilterGroupID.value })
    trackProxyTask(result.task)
    message.value = `代理延迟检测任务已创建：共 ${result.summary.total} 个代理，正在后台运行。`
    ui.toast({
      title: '代理检测已启动',
      message: `任务 ${result.task.id} 已进入任务模块，日志中心会实时记录每个代理的检测结果。`,
      tone: 'success',
      duration: 6200
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '检测代理延迟失败'
    checkingProxies.value = false
    ui.toast({ title: '代理检测启动失败', message: error.value, tone: 'error', duration: 5200 })
  }
}

async function deleteAbnormalAccounts() {
  const accepted = await ui.confirm({
    title: '删除异常监听账号',
    message: '将删除状态异常、检测失败或需要重新导入的监听账号，正常与未检测账号会保留。',
    confirmText: '确认删除',
    cancelText: '取消',
    tone: 'error'
  })
  if (!accepted) return
  deletingAbnormal.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await api.deleteAbnormalListenerAccounts()
    message.value = `已删除 ${result.deleted} 个异常监听账号`
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除异常账号失败'
  } finally {
    deletingAbnormal.value = false
  }
}

async function deleteItem(kind: ImportKind, id: string) {
  const label = kind === 'account' ? '监听账号' : kind === 'target' ? '监听群' : '监听代理'
  const accepted = await ui.confirm({
    title: `删除${label}`,
    message: `确认删除这个${label}？删除后不会影响其他独立库数据。`,
    confirmText: '删除',
    cancelText: '取消',
    tone: 'error'
  })
  if (!accepted) return
  try {
    if (kind === 'account') await api.deleteListenerAccount(id)
    if (kind === 'target') await api.deleteListenerTarget(id)
    if (kind === 'proxy') await api.deleteListenerProxy(id)
    ui.toast({ title: `${label}已删除`, message: '列表已刷新。', tone: 'success' })
    await load()
  } catch (err) {
    ui.toast({ title: `删除${label}失败`, message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  }
}

function toggleAll(kind: ImportKind) {
  if (kind === 'account') {
    const ids = pagedAccounts.value.map((item) => item.id)
    selectedAccountIDs.value = allAccountsSelected.value ? selectedAccountIDs.value.filter((id) => !ids.includes(id)) : [...new Set([...selectedAccountIDs.value, ...ids])]
    return
  }
  if (kind === 'target') {
    const ids = pagedTargets.value.map((item) => item.id)
    selectedTargetIDs.value = allTargetsSelected.value ? selectedTargetIDs.value.filter((id) => !ids.includes(id)) : [...new Set([...selectedTargetIDs.value, ...ids])]
    return
  }
  const ids = pagedProxies.value.map((item) => item.id)
  selectedProxyIDs.value = allProxiesSelected.value ? selectedProxyIDs.value.filter((id) => !ids.includes(id)) : [...new Set([...selectedProxyIDs.value, ...ids])]
}

async function deleteSelected(kind: ImportKind) {
  const label = kind === 'account' ? '监听账号' : kind === 'target' ? '监听群' : '监听代理'
  const ids = kind === 'account' ? selectedAccountIDs.value : kind === 'target' ? selectedTargetIDs.value : selectedProxyIDs.value
  if (!ids.length) return
  const accepted = await ui.confirm({
    title: `批量删除${label}`,
    message: `确认删除已选的 ${ids.length} 个${label}？`,
    confirmText: '批量删除',
    cancelText: '取消',
    tone: 'error'
  })
  if (!accepted) return
  try {
    if (kind === 'account') {
      await runWithConcurrency(ids, batchDeleteConcurrency, (id) => api.deleteListenerAccount(id))
      selectedAccountIDs.value = []
    } else if (kind === 'target') {
      await runWithConcurrency(ids, batchDeleteConcurrency, (id) => api.deleteListenerTarget(id))
      selectedTargetIDs.value = []
    } else {
      await runWithConcurrency(ids, batchDeleteConcurrency, (id) => api.deleteListenerProxy(id))
      selectedProxyIDs.value = []
    }
    ui.toast({ title: `${label}已批量删除`, message: `已删除 ${ids.length} 条记录。`, tone: 'success' })
    await load()
  } catch (err) {
    ui.toast({ title: `批量删除${label}失败`, message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  }
}

async function deleteCurrentProxyGroup() {
  const groupID = proxyFilterGroupID.value
  if (!groupID) return
  const group = proxyGroups.value.find((item) => item.id === groupID)
  const count = proxies.value.filter((item) => item.group_id === groupID).length
  const accepted = await ui.confirm({
    title: '删除代理分组',
    message: `确认删除代理分组「${group?.name || '未命名'}」？该分组下 ${count} 个代理会一起删除，已绑定这些代理的监听号会被取消代理绑定。`,
    confirmText: '删除分组',
    cancelText: '取消',
    tone: 'error'
  })
  if (!accepted) return
  deletingProxyGroup.value = true
  try {
    await api.deleteGroup('listener_proxy', groupID)
    proxyFilterGroupID.value = ''
    if (assignProxyGroupID.value === groupID) assignProxyGroupID.value = ''
    selectedProxyIDs.value = []
    ui.toast({ title: '代理分组已删除', message: '分组及该分组下代理已删除，列表已刷新。', tone: 'success' })
    await load()
  } catch (err) {
    ui.toast({ title: '删除代理分组失败', message: err instanceof Error ? err.message : '请求失败', tone: 'error' })
  } finally {
    deletingProxyGroup.value = false
  }
}

function statusText(status: string) {
  const map: Record<string, string> = { normal: '正常', online: '正常', offline: '会话有效', unchecked: '未检测', abnormal: '异常', unknown: '未知' }
  return map[status] || status || '-'
}

function accountTone(item: ListenerAccount) {
  const text = `${item.status || ''} ${item.risk_status || ''}`.toLowerCase()
  if (text.includes('abnormal') || text.includes('failed') || text.includes('需重新') || text.includes('受限') || text.includes('检测失败')) return 'danger'
  if (text.includes('offline')) return 'success'
  if (text.includes('normal') || text.includes('online') || item.risk_status === '正常') return 'success'
  return 'info'
}

function accessTypeText(value: string) {
  if (!value) return '-'
  if (['tdata', 'data'].includes(value.toLowerCase())) return 'TData'
  if (value.toLowerCase() === 'session') return 'Session'
  return value
}

function accountExitIPText(item: ListenerAccount) {
  if (item.exit_ip) return item.exit_ip
  if (item.proxy_id) return '待检测'
  return '未分配'
}

function avatarInitial(item: ListenerAccount) {
  const source = item.nickname || item.phone_display || item.phone || '?'
  return source.trim().slice(0, 1).toUpperCase()
}

function targetTypeText(value: string) {
  if (value === 'channel') return '频道'
  if (value === 'group' || value === 'supergroup') return '群组'
  return value || '未知'
}

function groupName(groups: Group[], id?: string | null) {
  if (!id) return '未分组'
  return groups.find((group) => group.id === id)?.name || '未分组'
}

function normalizeKeyword(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

function pageSlice<T>(items: T[], page: number) {
  const start = (page - 1) * listPageSize
  return items.slice(start, start + listPageSize)
}

async function runWithConcurrency<T>(items: T[], concurrency: number, worker: (item: T) => Promise<unknown>) {
  const queue = [...items]
  const workers = Array.from({ length: Math.min(concurrency, queue.length) }, async () => {
    while (queue.length) {
      const item = queue.shift()
      if (item !== undefined) await worker(item)
    }
  })
  await Promise.all(workers)
}

function formatPhone(value: string) {
  const digits = (value || '').replace(/\D/g, '')
  if (!digits) return ''
  if (digits.startsWith('86') && digits.length > 2) return `+86 ${digits.slice(2)}`
  if (digits.startsWith('1') && digits.length > 1) return `+1 ${digits.slice(1)}`
  return value
}

function markAvatarBroken(id: string) {
  if (!brokenAvatarIDs.value.includes(id)) {
    brokenAvatarIDs.value = [...brokenAvatarIDs.value, id]
  }
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

function membershipTaskDoneMessage(task: Task) {
  const summary = task.summary || {}
  return `有效 ${numericSummary(summary, 'active')}，移除 ${numericSummary(summary, 'removed')}，待复查 ${numericSummary(summary, 'skipped') + numericSummary(summary, 'failed')}。`
}

function accountTaskDoneMessage(task: Task) {
  const summary = task.summary || {}
  return `总数 ${numericSummary(summary, 'total')}，正常 ${numericSummary(summary, 'normal')}，会话有效 ${numericSummary(summary, 'offline')}，异常 ${numericSummary(summary, 'abnormal')}。`
}

function joinTaskDoneMessage(task: Task) {
  const summary = task.summary || {}
  return `成功 ${numericSummary(summary, 'success')}，失败 ${numericSummary(summary, 'failed')}，跳过 ${numericSummary(summary, 'skipped')}。`
}

function proxyTaskDoneMessage(task: Task) {
  const summary = task.summary || {}
  return `总数 ${numericSummary(summary, 'total')}，正常 ${numericSummary(summary, 'normal')}，失败 ${numericSummary(summary, 'failed')}，超时 ${numericSummary(summary, 'timeout')}。`
}

async function resumeAccountTask() {
  try {
    const tasks = await api.tasks({ type: 'listener_account_check', limit: 50 })
    const running = tasks.find((task) => !isTaskFinished(task.status))
    if (running) trackAccountTask(running)
  } catch {
    // 页面恢复任务失败不阻塞主列表加载。
  }
}

async function resumeMembershipTask() {
  try {
    const tasks = await api.tasks({ type: 'target_membership_refresh', limit: 50 })
    const running = tasks.find((task) => {
      const kind = String(task.payload?.account_kind || '')
      return !isTaskFinished(task.status) && (!kind || kind === 'listener' || kind === 'all')
    })
    if (running) trackMembershipTask(running)
  } catch {
    // 页面恢复任务失败不阻塞主列表加载。
  }
}

async function resumeJoinTask() {
  try {
    const tasks = await api.tasks({ type: 'listener_join_targets', limit: 50 })
    const running = tasks.find((task) => !isTaskFinished(task.status))
    if (running) trackJoinTask(running)
  } catch {
    // 页面恢复任务失败不阻塞主列表加载。
  }
}

async function resumeProxyTask() {
  try {
    const tasks = await api.tasks({ type: 'listener_proxy_check', limit: 50 })
    const running = tasks.find((task) => !isTaskFinished(task.status))
    if (running) trackProxyTask(running)
  } catch {
    // 页面恢复任务失败不阻塞主列表加载。
  }
}

function boundedNumber(value: number, min: number, max: number, fallback: number) {
  const next = Number(value)
  if (!Number.isFinite(next)) return fallback
  return Math.min(max, Math.max(min, Math.trunc(next)))
}

function proxyLatencyTone(item: ListenerProxy) {
  const status = (item.status || '').toLowerCase()
  const latency = Number(item.latency_ms || 0)
  if (status === 'failed' || status === 'timeout') return 'bad'
  if (!latency) return 'unknown'
  if (latency <= 300) return 'good'
  if (latency <= 1000) return 'warn'
  return 'bad'
}

function proxyLatencyIcon(item: ListenerProxy) {
  const tone = proxyLatencyTone(item)
  if (tone === 'good') return '●'
  if (tone === 'warn') return '▲'
  if (tone === 'bad') return '×'
  return '○'
}

function proxyLatencyText(item: ListenerProxy) {
  const status = (item.status || '').toLowerCase()
  if (status === 'timeout') return '超时'
  if (status === 'failed') return '失败'
  return item.latency_ms ? `${item.latency_ms} ms` : '未检测'
}

function proxyExitTone(item: ListenerProxy) {
  const status = (item.status || '').toLowerCase()
  if (!status || status === 'untested') return 'unknown'
  if (status === 'failed' || status === 'timeout') return 'bad'
  return (item.exit_ip || '').trim() ? 'good' : 'bad'
}

function proxyExitIcon(item: ListenerProxy) {
  const tone = proxyExitTone(item)
  if (tone === 'good') return '●'
  if (tone === 'bad') return '×'
  return '○'
}

function proxyExitStatusText(item: ListenerProxy) {
  const tone = proxyExitTone(item)
  if (tone === 'good') return '通'
  if (tone === 'bad') return '不通'
  return '待检测'
}

function proxyWebTone(item: ListenerProxy) {
  const status = (item.web_status || '').toLowerCase()
  if (status === 'normal') return 'good'
  if (status === 'failed' || status === 'timeout') return 'bad'
  return 'unknown'
}

function proxyWebIcon(item: ListenerProxy) {
  const tone = proxyWebTone(item)
  if (tone === 'good') return '●'
  if (tone === 'bad') return '×'
  return '○'
}

function proxyWebText(item: ListenerProxy) {
  const status = (item.web_status || '').toLowerCase()
  if (status === 'normal') return '通'
  if (status === 'timeout') return '超时'
  if (status === 'failed') return '不通'
  return '待检测'
}

function proxyTelegramTone(item: ListenerProxy) {
  const status = (item.telegram_status || '').toLowerCase()
  if (status === 'normal') return 'good'
  if (status === 'failed' || status === 'timeout') return 'bad'
  return 'unknown'
}

function proxyTelegramIcon(item: ListenerProxy) {
  const tone = proxyTelegramTone(item)
  if (tone === 'good') return '●'
  if (tone === 'bad') return '×'
  return '○'
}

function proxyTelegramText(item: ListenerProxy) {
  const status = (item.telegram_status || '').toLowerCase()
  if (status === 'normal') return '可用'
  if (status === 'timeout') return '超时'
  if (status === 'failed') return '不通'
  return '待检测'
}

function proxyExitIPText(item: ListenerProxy) {
  return item.exit_ip || '待检测'
}

function proxyCountryText(item: ListenerProxy) {
  const country = (item.country || '').trim()
  if (!country || country === '未知') return '未知国家'
  return country
}

function proxyAssignmentLimit(item: ListenerProxy) {
  const limit = Number(item.assignment_limit || defaultProxyAssignmentLimit)
  return Number.isFinite(limit) && limit > 0 ? limit : defaultProxyAssignmentLimit
}

function proxyBoundDisplay(item: ListenerProxy) {
  const limit = proxyAssignmentLimit(item)
  return `${Math.min(item.bound_accounts || 0, limit)}/${limit}`
}

function proxyAssignmentPercent(item: ListenerProxy) {
  const fallback = (item.bound_accounts || 0) / proxyAssignmentLimit(item) * 100
  return Math.min(item.assignment_percent || fallback, 100)
}

function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

watch([accountFilterGroupID, accountKeyword], () => {
  accountPage.value = 1
})
watch([targetFilterGroupID, targetKeyword], () => {
  targetPage.value = 1
})
watch([proxyFilterGroupID, proxyKeyword], () => {
  proxyPage.value = 1
})
watch(accountPageCount, (count) => {
  if (accountPage.value > count) accountPage.value = count
})
watch(targetPageCount, (count) => {
  if (targetPage.value > count) targetPage.value = count
})
watch(proxyPageCount, (count) => {
  if (proxyPage.value > count) proxyPage.value = count
})

onMounted(async () => {
  await load()
  await resumeAccountTask()
  await resumeMembershipTask()
  await resumeJoinTask()
  await resumeProxyTask()
})
onUnmounted(() => {
  stopAccountTaskPolling()
  stopMembershipTaskPolling()
  stopJoinTaskPolling()
  stopProxyTaskPolling()
})
</script>

<style scoped>
.listener-matrix-shell { gap: 18px; }
.listener-action-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 16px;
}
.upload-panel { min-height: 100%; }
.listener-action-grid > * { min-width: 0; }
.action-card {
  position: relative;
  overflow: hidden;
}
.action-card::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: .42;
  background: radial-gradient(circle at 0 0, rgba(34, 211, 238, .22), transparent 34%);
}
.action-card[data-kind='target']::before {
  background: radial-gradient(circle at 0 0, rgba(244, 114, 182, .22), transparent 34%);
}
.action-card[data-kind='proxy']::before {
  background: radial-gradient(circle at 0 0, rgba(245, 158, 11, .2), transparent 34%);
}
.action-card[data-kind='assign']::before {
  background: radial-gradient(circle at 0 0, rgba(52, 211, 153, .22), transparent 34%);
}
.action-card[data-kind='join']::before {
  background: radial-gradient(circle at 0 0, rgba(56, 189, 248, .22), transparent 34%);
}
.panel-title-row { display: flex; align-items: flex-start; gap: 12px; }
.panel-title-row h2, .list-toolbar h2 { margin: 0; font-weight: 900; }
.panel-title-row p, .list-toolbar p { margin: 4px 0 0; color: var(--app-text-muted); font-size: 13px; line-height: 1.6; }
.panel-icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 8px; background: rgba(255,255,255,.08); color: var(--accent-cyan); box-shadow: inset 0 1px rgba(255,255,255,.12); }
.action-card[data-kind='target'] .panel-icon { color: var(--accent-pink); }
.action-card[data-kind='proxy'] .panel-icon { color: var(--accent-warning); }
.action-card[data-kind='join'] .panel-icon { color: var(--accent-cyan); }
.limit-field {
  display: grid;
  gap: 6px;
  min-width: 0;
  color: var(--app-text-muted);
  font-size: 12px;
  font-weight: 800;
}
.limit-field input {
  min-height: 44px;
  width: 100%;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,.12);
  background: rgba(8, 15, 30, .72);
  color: #fff;
  padding: 0 12px;
  font-size: 14px;
}
.action-card[data-kind='assign'] .panel-icon { color: var(--accent-success); }
.drop-zone { margin: 16px 0; display: grid; min-height: 132px; place-items: center; gap: 7px; text-align: center; border: 1px dashed rgba(79,172,254,.45); border-radius: 8px; background: linear-gradient(135deg, rgba(34,197,94,.1), rgba(79,172,254,.08)); cursor: pointer; }
.drop-zone:hover { transform: translateY(-2px); border-color: rgba(0,242,254,.8); box-shadow: 0 14px 34px rgba(34,211,238,.14); }
.drop-zone strong { color: #fff; }
.drop-zone span, .panel-mini-grid span { color: var(--app-text-muted); font-size: 12px; }
.manual-import-input {
  width: 100%;
  min-height: 104px;
  margin: 16px 0 12px;
  resize: vertical;
  border-radius: 8px;
  padding: 12px;
  font-size: 13px;
  line-height: 1.55;
}
.panel-mini-grid { display: grid; grid-template-columns: 1fr auto; gap: 8px 12px; padding: 12px; border-radius: 8px; border: 1px solid rgba(255,255,255,.08); background: rgba(255,255,255,.05); }
.auto-assign-box { display: flex; min-height: 44px; align-items: center; justify-content: center; gap: 8px; border-radius: 8px; border: 1px solid rgba(255,255,255,.08); background: rgba(255,255,255,.05); color: var(--app-text-soft); }
.mini-result { border: 1px solid rgba(52,211,153,.25); background: rgba(52,211,153,.1); color: #86efac; border-radius: 8px; padding: 10px 12px; font-size: 13px; }
.list-card { min-height: 0; }
.list-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.toolbar-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 10px; }
.table-scroll { overflow: auto; border-radius: 8px; border: 1px solid rgba(255,255,255,.08); }
.matrix-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.matrix-table th { padding: 13px 14px; color: var(--app-text-muted); font-weight: 800; background: rgba(255,255,255,.045); text-align: left; }
.matrix-table td { padding: 14px; border-top: 1px solid rgba(255,255,255,.07); vertical-align: middle; }
.matrix-table tr:hover td { background: rgba(255,255,255,.045); }
.matrix-table tr.selected td {
  background: rgba(0, 242, 254, .075);
}
.select-col {
  width: 48px;
  text-align: center !important;
}
.matrix-check {
  display: inline-grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,.08);
  background: rgba(255,255,255,.05);
}
.matrix-check input {
  width: 15px;
  height: 15px;
  accent-color: #22d3ee;
}
.listener-avatar {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,.1);
  background: linear-gradient(135deg, rgba(34,211,238,.2), rgba(244,114,182,.18));
  color: #fff;
  font-weight: 900;
  box-shadow: inset 0 1px rgba(255,255,255,.14);
}
.listener-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.count-cell { display: grid; gap: 3px; }
.count-cell span { color: var(--app-text-muted); font-size: 12px; }
.proxy-bind { position: relative; overflow: hidden; height: 28px; min-width: 90px; border-radius: 8px; border: 1px solid rgba(255,255,255,.08); background: rgba(255,255,255,.06); }
.proxy-bind i { position: absolute; inset: 0 auto 0 0; background: linear-gradient(135deg, rgba(34,197,94,.8), rgba(34,211,238,.8)); }
.proxy-bind span { position: relative; z-index: 1; display: grid; height: 100%; place-items: center; font-weight: 800; }
.latency-badge,
.country-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  border-radius: 8px;
  padding: 0 10px;
  border: 1px solid rgba(255,255,255,.1);
  background: rgba(255,255,255,.06);
  font-weight: 800;
  white-space: nowrap;
}
.latency-badge i,
.country-badge i {
  font-style: normal;
}
.latency-badge[data-tone='good'] {
  color: #86efac;
  border-color: rgba(34,197,94,.32);
  background: rgba(34,197,94,.12);
}
.latency-badge[data-tone='warn'] {
  color: #fcd34d;
  border-color: rgba(245,158,11,.35);
  background: rgba(245,158,11,.12);
}
.latency-badge[data-tone='bad'] {
  color: #fda4af;
  border-color: rgba(244,63,94,.35);
  background: rgba(244,63,94,.12);
}
.latency-badge[data-tone='unknown'] {
  color: var(--app-text-muted);
}
.country-badge {
  color: #dbeafe;
}
.health-settings-dialog {
  width: min(34rem, calc(100vw - 2rem));
}
.health-settings-header {
  margin-bottom: 22px;
}
.health-settings-form {
  display: grid;
  gap: 14px;
}
.health-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  min-height: 74px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.42);
  padding: 16px 18px;
}
.health-toggle-row input {
  flex: 0 0 auto;
  accent-color: var(--accent-cyan);
}
.health-field {
  display: grid;
  gap: 8px;
}
.health-field span {
  color: var(--app-text-muted);
  font-size: 0.84rem;
}
.health-field input {
  width: 100%;
  min-height: 46px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(15, 23, 42, 0.72);
  padding: 0 14px;
  color: var(--app-text);
  font-size: 0.92rem;
  outline: none;
}
.health-field input:focus {
  border-color: rgba(34, 211, 238, 0.58);
  box-shadow: 0 0 0 3px rgba(34, 211, 238, 0.12);
}
.health-settings-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 26px;
}
.empty-cell { text-align: center; color: var(--app-text-muted); padding: 30px !important; }
@media (max-width: 1280px) { .listener-action-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 900px) {
  .listener-action-grid { grid-template-columns: 1fr; }
  .list-toolbar { align-items: stretch; flex-direction: column; }
  .toolbar-actions { justify-content: stretch; }
  .toolbar-actions > * { width: 100%; }
}
@media (max-width: 640px) {
  .health-settings-dialog {
    width: min(100%, calc(100vw - 1.25rem));
  }
  .health-toggle-row {
    align-items: flex-start;
    padding: 14px;
  }
  .health-settings-actions > * {
    flex: 1 1 0;
  }
}
</style>

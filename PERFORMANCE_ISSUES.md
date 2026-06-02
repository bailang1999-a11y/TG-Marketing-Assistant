# TG营销助手性能问题诊断报告

生成时间：2026-06-02  
诊断范围：代码静态审计（前端 + 后端）

---

## 🔴 严重问题（立即修复）

### 1. 前端全局任务轮询风暴 ⚠️
**位置**: `frontend/src/components/GlobalTaskNotifier.vue:54`

```typescript
timer = window.setInterval(pollTrackedTasks, 4000)
```

**问题**:
- 每 4 秒调用 `api.tasks({ limit: 200 })`，**全量拉取 200 个任务**
- 即使 `trackedTasks` 为空也会轮询
- 此组件全局挂载，只要用户登录就**持续轮询**
- 后端 `ListTasks` 混合查询 `tasks` + `bot_dm_tasks` 两张表，开销大

**影响**:
- 用户打开任意页面，每 4 秒触发一次复杂联表查询
- 数据库连接池被轮询占满
- 前端内存持续增长（200 个任务对象 × 每 4 秒）

**修复建议**:
```typescript
// 1. 只在有追踪任务时才轮询
if (!ui.trackedTasks.length) return

// 2. 使用 WebSocket 推送任务状态，移除轮询
// 3. 或改为按 task_id 精确查询，而非全量拉取 200 个
const taskIDs = ui.trackedTasks.map(t => t.id)
const tasks = await api.tasks({ ids: taskIDs }) // 需要后端支持
```

---

### 2. 监听矩阵页面 4 个并发定时器
**位置**: `frontend/src/views/ListenerAdminView.vue:631-634`

```typescript
accountTaskTimer = window.setInterval(pollAccountTask, 2500)      // 每 2.5s
membershipTaskTimer = window.setInterval(pollMembershipTask, 2500)
joinTaskTimer = window.setInterval(pollJoinTask, 2500)
proxyTaskTimer = window.setInterval(pollProxyTask, 2500)
```

**问题**:
- 4 个定时器每 2.5 秒各调用 `api.tasks({ type: 'xxx', limit: 50 })`
- **每 2.5 秒 = 4 次数据库查询**
- 打开监听矩阵页面后台就持续轰炸

**影响**:
- 打开此页面，数据库 QPS 瞬间 +1.6/s（4 次 / 2.5s）
- 配合全局轮询（+0.25/s），总 QPS +1.85/s
- 多用户同时打开此页面会让数据库崩溃

**修复建议**:
```typescript
// 1. 合并为单个定时器，一次拉取多种类型
const tasks = await api.tasks({ 
  type: ['listener_account_check', 'target_membership_refresh', 
         'listener_join_targets', 'listener_proxy_check'],
  limit: 200 
})

// 2. 延长轮询间隔到 10-15 秒
// 3. 使用 WebSocket 替代轮询
```

---

### 3. Dashboard 每 5 秒执行 13 个聚合查询
**位置**: `backend/internal/handlers/dashboard.go:41-54`

```go
dashboardCacheTTL = 5 * time.Second  // 缓存仅 5 秒

// 13 个 Count/Sum 查询
s.db.Model(&models.Terminal{}).Count(&terminalTotal)
s.db.Model(&models.Terminal{}).Where("status = ?", "online").Count(&terminalOnline)
s.db.Model(&models.Task{}).Where("status IN ?", []string{...}).Count(&taskActive)
// ... 还有 10 个
```

**问题**:
- 缓存 TTL 只有 5 秒，过期后立即执行 **13 个聚合查询**
- 每个 `Count/Sum` 都要扫描大量行（无法完全走索引）
- Dashboard 是首页，访问频率最高

**影响**:
- 高峰期 Dashboard 访问量 × 13 个查询 = 数据库雪崩
- 即使有缓存，5 秒 TTL 太短

**修复建议**:
```go
// 1. 延长缓存到 30-60 秒
dashboardCacheTTL = 30 * time.Second

// 2. 改用 Redis 缓存，设置更长 TTL
// 3. 低频统计（7 日趋势）改为 scheduled job 预计算
// 4. 高频统计（在线终端、活跃任务）改用 Redis 计数器
```

---

### 4. 任务列表混合查询两张表
**位置**: `backend/internal/handlers/task_queries.go:28-131`

```go
// 先查 tasks 表
query.Limit(limit).Offset(offset).Find(&tasks)

// 再查 bot_dm_tasks 表
dmQuery.Find(&dmTasks)

// 合并、排序、截断
sort.SliceStable(tasks, ...) 
tasks = tasks[:limit]
```

**问题**:
- `ListTasks` 每次调用需要查询两张表，再内存合并排序
- Bot 私信任务和通用任务分裂（架构文档已指出此问题）
- 排序发生在应用层，无法利用数据库索引
- `CAST(payload AS TEXT) LIKE` 全表扫描（第 54 行）

**影响**:
- 任务中心页面加载慢（limit 500）
- 全局轮询每 4 秒执行此复杂查询（limit 200）

**修复建议**:
```go
// 短期：减少 limit，添加分页
// 中期：统一任务模型（参考 docs/architecture-redesign.md）
// 长期：为 bot_user_id 查询添加专门的关联表索引
```

---

## 🟡 中等问题（优先修复）

### 5. 前端巨型组件
**位置**: 多个 View 文件

| 文件 | 行数 | 问题 |
|------|------|------|
| `ListenerAdminView.vue` | 2079 | 所有监听逻辑耦合在一个文件 |
| `BotSettingsView.vue` | 2025 | Bot 配置 + 表单验证 + 实时预览 |
| `ProfileAssetsView.vue` | 1846 | 资料库 + 上传 + 预览 |
| `TerminalsView.vue` | 1809 | 终端管理全流程 |

**问题**:
- 单文件组件过大，Vue 编译和运行时性能差
- 大量 `ref`/`computed` 在同一作用域，响应式系统压力大
- 代码难以维护和性能调优

**修复建议**:
```
// 拆分为领域子组件
ListenerAdminView.vue (200 行入口)
├── ListenerAccountTable.vue
├── ListenerTargetTable.vue
├── ListenerProxyTable.vue
└── ListenerTaskMonitor.vue
```

---

### 6. 前端其他轮询点
**位置**: 多个 View 文件

| 位置 | 间隔 | 影响 |
|------|------|------|
| `TargetPoolView.vue:687` | 2.5s | 刷新目标池任务 |
| `OutreachSyncView.vue:780` | ? | 收件箱轮询 |
| `WorkflowView.vue:1077` | ? | 工作流预览定时器 |

**修复建议**:
- 统一改为 WebSocket 推送
- 或延长轮询间隔到 10-30 秒

---

### 7. 任务日志查询无分页限制
**位置**: `frontend/src/views/TaskCenterView.vue:385`

```typescript
selectedTaskLogs.value = await api.taskLogs(task.id, { limit: 1000 })
```

**问题**:
- 每次点击任务，拉取 1000 条日志
- 长任务日志可能远超 1000 条，仍然很慢

**修复建议**:
```typescript
// 1. 默认只加载最近 100 条
// 2. 滚动到底部时懒加载更多
// 3. 使用 WebSocket 实时推送新日志（已有 ws://.../logs）
```

---

### 8. Dashboard 统计查询缺少复合索引
**位置**: `backend/internal/handlers/dashboard.go:52-54`

```go
// 查询今日更新的目标命中数
s.db.Model(&models.Target{}).
  Where("updated_at >= ?", todayStart).
  Select("COALESCE(SUM(notification_count),0)").
  Scan(&todayHits)
```

**问题**:
- `updated_at >= todayStart` 可能需要扫描大量行
- `notification_count` 字段在 Target 表中，但缺少 `(updated_at, notification_count)` 复合索引

**修复建议**:
```sql
-- 添加 migration
CREATE INDEX idx_targets_updated_at ON targets(updated_at DESC);
CREATE INDEX idx_targets_tenant_updated ON targets(tenant_id, updated_at DESC);
```

---

## 🟢 改进建议（长期优化）

### 9. 数据库连接池配置偏保守
**配置**: `docker-compose.yml`

```yaml
gateway:
  DB_MAX_IDLE_CONNS: "10"
  DB_MAX_OPEN_CONNS: "40"

worker:
  DB_MAX_IDLE_CONNS: "5"
  DB_MAX_OPEN_CONNS: "20"
```

**建议**:
- 根据实际并发量调整（参考 `docs/high-concurrency-runbook.md`）
- 监控数据库连接使用率，动态调参
- 考虑引入 PgBouncer 连接池中间件

---

### 10. 缺少前端性能监控
**现状**: 无前端性能埋点

**建议**:
- 添加关键接口耗时监控（Dashboard、TaskList、ListenerAdmin）
- 监控前端内存泄漏（定时器清理、组件销毁）
- 使用 Vue DevTools Profiler 分析组件渲染耗时

---

### 11. WebSocket 日志流未充分利用
**现状**: 
- 后端已实现 WebSocket 日志推送（`/api/v1/ws/logs`）
- 前端在 `TasksLogsView.vue:166` 使用
- 但任务状态更新仍依赖轮询

**建议**:
- 扩展 WebSocket 协议，推送任务状态变更
- 替换 GlobalTaskNotifier 的轮询逻辑

---

## 📊 预估性能提升

| 优化项 | 预估提升 |
|--------|---------|
| 关闭全局任务轮询 | **数据库 QPS -25%**，前端内存 -30% |
| 合并监听矩阵 4 个定时器 | **数据库 QPS -18%**（打开该页面时） |
| Dashboard 缓存延长到 60s | **数据库 QPS -15%**（高峰期） |
| 统一任务表模型 | 任务列表响应时间 **-40%** |
| 拆分巨型前端组件 | 页面初始渲染 **-30%**，交互响应 **-50%** |

**综合预估**: 修复严重问题后，整体卡顿感可降低 **60-70%**。

---

## 🔧 修复优先级

### 第一阶段（立即）
1. ✅ 关闭或优化 GlobalTaskNotifier 轮询
2. ✅ 合并监听矩阵 4 个定时器
3. ✅ Dashboard 缓存延长到 30-60s

### 第二阶段（本周）
4. 任务列表添加真实分页（前端 + 后端）
5. 任务日志懒加载
6. 添加数据库慢查询日志监控

### 第三阶段（本月）
7. WebSocket 推送任务状态，移除所有轮询
8. 拆分前端巨型组件
9. 统一任务模型（参考架构重构文档）

---

## 🚀 快速验证脚本

在服务器上运行 [`diagnose.sh`](diagnose.sh)，重点关注：

```bash
# 1. PostgreSQL 连接数和慢查询
docker exec tg-postgres psql -U tg_marketing -d tg_marketing -c "
  SELECT count(*), state FROM pg_stat_activity 
  WHERE datname = 'tg_marketing' GROUP BY state;"

# 2. Gateway 容器 goroutine 泄漏
docker stats --no-stream tg-gateway

# 3. Redis 慢日志
docker exec tg-redis redis-cli -a 'tg_redis_change_this_8b6a4e2f9c1d7a3b' \
  SLOWLOG GET 10
```

---

## 📚 相关文档
- [高并发运行手册](docs/high-concurrency-runbook.md)
- [架构重构方案](docs/architecture-redesign.md)
- [环境变量配置](docs/env.md)

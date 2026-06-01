# Codex3 架构与数据库重构方案

## 1. 当前项目现状

### 1.1 技术栈
- 前端：Vue 3 + Vite + TypeScript
- 后端：Go + Gin + GORM + PostgreSQL
- 运行方式：`gateway` + `worker` + `scheduler`
- 任务与日志：当前以 `tasks` / `task_logs` 为主，Bot 私信任务单独使用 `bot_dm_tasks`

### 1.2 当前结构特点
- 前端以页面为中心，核心页面文件体积较大：
  - `frontend/src/views/BotSettingsView.vue`
  - `frontend/src/views/ListenerAdminView.vue`
  - `frontend/src/views/WorkflowView.vue`
  - `frontend/src/views/ProfileAssetsView.vue`
- 后端以 handler 为中心，核心文件明显过载：
  - `backend/internal/handlers/bot.go`
  - `backend/internal/handlers/listener_admin.go`
  - `backend/internal/handlers/tasks.go`
  - `backend/internal/handlers/imports.go`
- 数据模型集中在单文件：
  - `backend/internal/models/models.go`
- 数据库依赖：
  - `backend/internal/database/database.go` 中 `AutoMigrate`
  - `backend/migrations/000001_init.sql` 只覆盖早期基础表，后续大量表已经偏向 GORM 自动维护

### 1.3 当前主要问题

#### 后端架构问题
- `bot.go` 超过 4000 行，混合了：
  - Bot 配置管理
  - Telegram 指令路由
  - 回调按钮处理
  - 试用/会员逻辑
  - 监听任务启停
  - 私信任务创建与停止
  - 推送文案拼装
  - 黑名单逻辑
  - 推送执行逻辑
- `tasks.go` 同时承担：
  - 任务列表
  - 任务日志
  - 任务动作控制
  - WebSocket 日志推送
  - 群发任务执行
  - Bot 私信任务兼容映射
- 领域边界不清晰，监听矩阵、Bot 用户、终端池、目标池、任务中心之间耦合偏高。

#### 数据库问题
- `models.go` 是“大一统模型文件”，业务域混杂。
- 过多重要业务状态放进 JSON 字段，不利于：
  - 查询
  - 过滤
  - 审计
  - 统计
  - 任务归因
- `BotDMTask` 与通用 `Task` 分裂，导致任务中心需要“伪任务映射”。
- 群组设计不统一：
  - 普通资源使用 `groups(resource_type)`
  - Bot 私信号池有独立分组表
  - 监听矩阵部分资源又有独立库/逻辑
- 缺少统一“任务运行态 / 进程态 / 停止原因 / 审计事件”模型。

#### 前端问题
- 页面太大，状态与展示逻辑耦合。
- Bot 配置、Bot 用户看板、监听矩阵都已经具备“平台级控制台”复杂度，但仍主要放在单页组件里。
- 缺少领域组件层和页面编排层。

---

## 2. 推荐目标架构

## 2.1 总体原则
- 按“业务域”拆，而不是按“handler 文件大小”拆。
- 任务统一归口到任务引擎。
- Bot 用户、监听矩阵、终端池、目标池、资源池全部明确分域。
- 可查询字段结构化，非核心配置才使用 JSON。
- 先做“兼容式重构”，不要一步推翻现有功能。

---

## 3. 后端目标目录

建议把后端逐步整理成下面的结构：

```text
backend/
  cmd/
    gateway/
    worker/
    scheduler/

  internal/
    api/
      http/
        router.go
        auth_handler.go
        dashboard_handler.go
        task_handler.go
        log_handler.go
        bot_config_handler.go
        bot_user_handler.go
        listener_admin_handler.go
        import_handler.go
        terminal_handler.go
        target_handler.go
        network_handler.go

    application/
      auth/
      tasks/
      imports/
      terminals/
      targets/
      listener/
      bot/
      scrm/

    domain/
      identity/
      groups/
      terminals/
      targets/
      assets/
      workflows/
      listener_matrix/
      bot_platform/
      tasks/
      logs/
      scrm/

    infrastructure/
      db/
      repository/
      telegram/
      storage/
      runtime/
      queue/

    models/        # 兼容过渡期保留，最终拆散
    middleware/
    config/
```

---

## 4. 业务域拆分建议

## 4.1 身份与租户域
负责：
- Web 用户
- 管理员 / 普通用户权限
- 租户隔离
- Web 用户与 Bot 用户绑定

核心实体：
- `tenant`
- `web_user`
- `web_user_binding`

## 4.2 终端池域
负责：
- Session / TData 导入
- 账号状态同步
- 资料修改
- 账号检测
- 目标加入
- 群发、资料修改等任务的执行端

核心实体：
- `terminal_account`
- `terminal_group`
- `terminal_group_member`
- `terminal_status_snapshot`
- `terminal_profile_snapshot`

## 4.3 监听矩阵域
必须和终端池彻底分库/分表/分逻辑。

负责：
- 监听账号导入与检测
- 监听群导入与分组
- 监听代理导入与分配
- 监听账号加入监听群

核心实体：
- `listener_account`
- `listener_account_group`
- `listener_account_group_member`
- `listener_target`
- `listener_target_group`
- `listener_target_group_member`
- `listener_proxy`
- `listener_proxy_group`
- `listener_proxy_assignment`

## 4.4 Bot 平台域
负责：
- 管理机器人配置
- 指令菜单
- 欢迎语与提示词
- Bot 用户订阅/试用/会员
- Bot 用户关键词与开关
- Bot 用户独享黑名单
- Bot 私信任务

核心实体：
- `bot_config`
- `bot_command`
- `bot_template`
- `bot_menu_layout`
- `bot_subscriber`
- `bot_license`
- `bot_user_settings`
- `bot_user_keyword`
- `bot_user_blacklist`
- `bot_source_blacklist`
- `bot_dm_task`

## 4.5 SCRM / 监听命中域
负责：
- 命中关键词监听
- 线索汇聚
- 搜索历史
- 命中推送
- 私信联动

核心实体：
- `scrm_rule`
- `scrm_rule_keyword`
- `scrm_lead`
- `scrm_hit_event`
- `scrm_recent_message`
- `scrm_push_record`

## 4.6 任务与日志域
负责：
- 所有任务统一编排
- 任务运行状态
- 任务停止原因
- 任务进程管理
- 日志 / 审计 / 事件流

核心实体：
- `task`
- `task_runtime`
- `task_event`
- `audit_log`

---

## 5. 数据库重构建议

## 5.1 当前数据库设计的核心问题

### 问题一：模型文件过于集中
`backend/internal/models/models.go` 同时承载所有业务域，维护成本会越来越高。

### 问题二：JSON 字段过多
以下内容现在大量在 JSON 里：
- Bot 启用命令
- 按钮文案
- 回复模板
- 默认关键词
- Bot 用户关键词
- 任务 payload / summary
- SCRM 关键词与监听号

建议原则：
- **要过滤、统计、联查、排序的字段必须结构化。**
- **纯展示模板、低频配置项才留 JSON。**

### 问题三：任务模型不统一
现在：
- 通用任务走 `tasks`
- Bot 私信走 `bot_dm_tasks`

这会导致：
- 任务中心展示不统一
- 停止/恢复/删除逻辑重复
- 日志归口困难
- 前端需要做“兼容映射”

建议：
- 中长期统一成一个主任务表
- 短期保留 `bot_dm_tasks`，但必须通过 `task_runtime` / `task_event` 统一接入任务中心

### 问题四：群组体系不统一
同样是“分组”，现在至少有三种模式：
- 通用 `groups(resource_type)`
- Bot 私信号池独立分组
- 监听矩阵分组逻辑独立

建议：
- 终端池、目标池、网络节点、资料素材可以继续用通用 `resource_groups`
- Bot 私信号池和监听矩阵建议独立分组表，不要再混入通用组

---

## 6. 推荐目标表设计

## 6.1 基础身份层

### `tenants`
- `id`
- `name`
- `status`
- `created_at`
- `updated_at`

### `web_users`
- `id`
- `tenant_id`
- `username`
- `password_hash`
- `email`
- `role`
- `status`
- `telegram_user_id`
- `telegram_username`
- `trial_ends_at`
- `created_by`
- `created_at`
- `updated_at`

---

## 6.2 终端池

### `terminal_accounts`
- `id`
- `tenant_id`
- `phone`
- `nickname`
- `avatar_url`
- `bio`
- `homepage`
- `status`
- `risk_status`
- `ban_status`
- `access_type`
- `origin_country`
- `origin_flag`
- `exit_ip`
- `exit_country`
- `exit_flag`
- `file_path`
- `session_hash`
- `last_online_at`
- `last_message_at`
- `sleep_until`
- `created_at`
- `updated_at`

### `terminal_groups`
- `id`
- `tenant_id`
- `name`
- `description`
- `created_at`
- `updated_at`

### `terminal_group_members`
- `id`
- `tenant_id`
- `group_id`
- `terminal_id`
- `created_at`

唯一索引：
- `(tenant_id, group_id, terminal_id)`

### `terminal_status_snapshots`
- `id`
- `tenant_id`
- `terminal_id`
- `status`
- `risk_status`
- `ban_status`
- `avatar_url`
- `nickname`
- `bio`
- `homepage`
- `exit_ip`
- `exit_country`
- `captured_at`

用途：
- 账号检测结果留痕
- 方便追溯“最后一次真实拉取”

---

## 6.3 监听矩阵

### `listener_accounts`
- `id`
- `tenant_id`
- `phone`
- `nickname`
- `avatar_url`
- `status`
- `risk_status`
- `access_type`
- `file_path`
- `session_hash`
- `proxy_id`
- `exit_ip`
- `exit_country`
- `exit_flag`
- `joined_targets`
- `last_online_at`
- `last_message_at`
- `created_at`
- `updated_at`

### `listener_account_groups`
- `id`
- `tenant_id`
- `name`
- `description`
- `created_at`
- `updated_at`

### `listener_account_group_members`
- `id`
- `tenant_id`
- `group_id`
- `listener_account_id`
- `created_at`

### `listener_targets`
- `id`
- `tenant_id`
- `identifier`
- `name`
- `type`
- `size`
- `status`
- `source_url`
- `created_at`
- `updated_at`

### `listener_target_groups`
- `id`
- `tenant_id`
- `name`
- `description`
- `created_at`
- `updated_at`

### `listener_target_group_members`
- `id`
- `tenant_id`
- `group_id`
- `listener_target_id`
- `created_at`

### `listener_proxies`
- `id`
- `tenant_id`
- `ip`
- `port`
- `protocol`
- `username`
- `password`
- `latency_ms`
- `country`
- `flag`
- `status`
- `created_at`
- `updated_at`

### `listener_proxy_groups`
- `id`
- `tenant_id`
- `name`
- `description`
- `created_at`
- `updated_at`

### `listener_proxy_group_members`
- `id`
- `tenant_id`
- `group_id`
- `proxy_id`
- `created_at`

### `listener_proxy_assignments`
- `id`
- `tenant_id`
- `proxy_id`
- `listener_account_id`
- `created_at`

约束：
- 每条代理最多绑定 5 个账号，放在服务层强校验，并为 `(tenant_id, proxy_id, listener_account_id)` 建唯一索引。

---

## 6.4 Bot 平台

### `bot_configs`
保留一租户一条主配置，但建议把以下内容拆出去：
- 命令
- 文案模板
- 菜单布局

### `bot_commands`
- `id`
- `tenant_id`
- `key`
- `parent_key`
- `command`
- `label`
- `visible_in_menu`
- `enabled`
- `sort_order`
- `scope`
- `created_at`
- `updated_at`

用途：
- 所有 bot 功能命令可配置
- 支持一级/二级命令
- 支持命令名和展示名同步修改

### `bot_templates`
- `id`
- `tenant_id`
- `template_key`
- `target_command_key`
- `content`
- `content_type`
- `version`
- `created_at`
- `updated_at`

用途：
- 欢迎语
- 帮助文案
- 设置中心提示
- 开启私信提示
- 联系管理员提示
- FAQ
- 功能成功/失败提示

### `bot_menu_buttons`
- `id`
- `tenant_id`
- `menu_key`
- `button_key`
- `label`
- `command_key`
- `sort_order`
- `enabled`
- `created_at`
- `updated_at`

### `bot_subscribers`
- `id`
- `tenant_id`
- `telegram_user_id`
- `user_id`
- `username`
- `nickname`
- `invite_code`
- `invited_by_id`
- `force_joined`
- `push_enabled`
- `push_chat_id`
- `keyword_limit`
- `match_mode`
- `user_blacklist_enabled`
- `ai_filter_enabled`
- `message_dedup_minutes`
- `dm_quota_total`
- `dm_quota_used`
- `status`
- `plan`
- `trial_started_at`
- `trial_ends_at`
- `license_id`
- `authorized_at`
- `expires_at`
- `last_seen_at`
- `created_at`
- `updated_at`

### `bot_user_keywords`
- `id`
- `tenant_id`
- `subscriber_id`
- `keyword`
- `sort_order`
- `created_at`

### `bot_user_terminal_group_bindings`
- `id`
- `tenant_id`
- `subscriber_id`
- `terminal_group_id`
- `created_at`

### `bot_user_blacklists`
- `id`
- `tenant_id`
- `subscriber_id`
- `user_account`
- `user_nickname`
- `reason`
- `created_at`

### `bot_source_blacklists`
- `id`
- `tenant_id`
- `subscriber_id`
- `source_key`
- `source_chat_id`
- `source_chat_name`
- `target_id`
- `reason`
- `created_at`

---

## 6.5 监听 / 命中 / 推送

### `scrm_rules`
- `id`
- `tenant_id`
- `name`
- `listen_group_id`
- `strike_group_id`
- `match_mode`
- `push_to_bot`
- `strike_enabled`
- `status`
- `created_at`
- `updated_at`

### `scrm_rule_monitor_accounts`
- `id`
- `tenant_id`
- `rule_id`
- `listener_account_id`
- `created_at`

### `scrm_rule_keywords`
- `id`
- `tenant_id`
- `rule_id`
- `keyword`
- `sort_order`
- `created_at`

### `scrm_leads`
- `id`
- `tenant_id`
- `target_id`
- `user_nickname`
- `user_account`
- `source_chat_id`
- `source_chat_name`
- `trigger_word`
- `trigger_message`
- `message_id`
- `assigned_worker`
- `status`
- `hit_at`
- `bot_pushed_at`
- `created_at`
- `updated_at`

### `scrm_recent_searches`
- `id`
- `tenant_id`
- `lead_id`
- `source_name`
- `source_url`
- `keyword`
- `message_text`
- `message_id`
- `message_time`
- `created_at`

这样可以替代 `RecentSearches JSON`，后面做“最近 10 条搜索记录”就会轻松很多。

---

## 6.6 统一任务模型

### 目标
监听任务和私信任务都进统一任务中心，而不是一部分在 `tasks`，一部分在 `bot_dm_tasks`。

### 推荐表

### `tasks`
- `id`
- `tenant_id`
- `task_no`
- `name`
- `task_type`  
  值建议：
  - `terminal_check`
  - `terminal_import`
  - `target_import`
  - `workflow_run`
  - `profile_apply`
  - `mass_message`
  - `listener_runtime`
  - `bot_dm`
- `owner_type`  
  取值：
  - `web_user`
  - `bot_subscriber`
  - `system`
- `owner_id`
- `status`
- `status_reason`
- `progress_current`
- `progress_total`
- `process_key`
- `process_pid`
- `created_by`
- `started_at`
- `ended_at`
- `created_at`
- `updated_at`

### `task_payloads`
- `id`
- `task_id`
- `payload_json`
- `summary_json`
- `created_at`
- `updated_at`

### `task_events`
- `id`
- `tenant_id`
- `task_id`
- `event_type`
- `level`
- `title`
- `message`
- `details_json`
- `created_at`

### `audit_logs`
- `id`
- `tenant_id`
- `actor_type`
- `actor_id`
- `action`
- `resource_type`
- `resource_id`
- `result`
- `message`
- `details_json`
- `created_at`

这样区分后：
- **任务日志** 看 `task_events`
- **操作审计** 看 `audit_logs`
- **页面实时输出** 从 `task_events` 和运行时状态拼

---

## 7. 关键索引建议

### 高频索引
- `web_users(tenant_id, role, status)`
- `terminal_accounts(tenant_id, status, group_id)`
- `listener_accounts(tenant_id, status)`
- `listener_targets(tenant_id, type, status)`
- `listener_proxies(tenant_id, status, latency_ms)`
- `bot_subscribers(tenant_id, telegram_user_id)`
- `bot_subscribers(tenant_id, status, expires_at)`
- `tasks(tenant_id, task_type, status, created_at desc)`
- `tasks(tenant_id, owner_type, owner_id, created_at desc)`
- `task_events(tenant_id, task_id, created_at desc)`
- `audit_logs(tenant_id, created_at desc)`
- `scrm_leads(tenant_id, hit_at desc)`
- `scrm_recent_searches(tenant_id, lead_id, message_time desc)`

### 唯一约束
- `bot_user_keywords(tenant_id, subscriber_id, keyword)`
- `listener_account_group_members(tenant_id, group_id, listener_account_id)`
- `listener_target_group_members(tenant_id, group_id, listener_target_id)`
- `listener_proxy_assignments(tenant_id, proxy_id, listener_account_id)`
- `bot_user_terminal_group_bindings(tenant_id, subscriber_id, terminal_group_id)`

---

## 8. 建议保留 JSON 的范围

以下内容可以继续保留 JSON：
- 复杂欢迎页预览布局
- Telegram 菜单渲染快照
- 任务执行过程中的中间摘要
- 可视化机器人右侧预览快照
- 部分低频配置备份

以下内容不建议继续放 JSON：
- 用户关键词
- 任务归属
- 任务状态原因
- 私信号池分组绑定
- 监听号与监听群关系
- 代理绑定关系
- 最近搜索记录
- 功能命令定义

---

## 9. 重构优先级

## 第一阶段：先稳住结构，不动核心功能
- 拆 `models.go`
- 拆 `bot.go`
- 拆 `tasks.go`
- 前端拆 `BotSettingsView.vue`
- 前端拆 `BotUsersDashboardView.vue`
- 增加 `docs/` 设计文档

### 推荐先拆的后端文件
- `bot.go` 拆为：
  - `bot_config_handlers.go`
  - `bot_command_handlers.go`
  - `bot_webhook_handlers.go`
  - `bot_render_text.go`
  - `bot_keyboard_render.go`
  - `bot_subscriber_service.go`
  - `bot_dm_service.go`
  - `bot_runtime_service.go`
- `tasks.go` 拆为：
  - `task_query_handlers.go`
  - `task_action_handlers.go`
  - `task_log_handlers.go`
  - `task_runtime_service.go`
- `listener_admin.go` 拆为：
  - `listener_account_handlers.go`
  - `listener_target_handlers.go`
  - `listener_proxy_handlers.go`
  - `listener_assign_service.go`

## 第二阶段：统一任务中心
- 新增统一 `tasks` / `task_events`
- Bot 监听任务和 Bot 私信任务统一入任务中心
- 所有开始/暂停/停止/恢复动作统一写审计日志

## 第三阶段：重构 Bot 平台数据层
- `bot_commands`
- `bot_templates`
- `bot_menu_buttons`
- `bot_user_keywords`
- `bot_user_terminal_group_bindings`

## 第四阶段：重构监听矩阵数据层
- 监听号 / 监听群 / 代理彻底独立
- 分组成员关系表独立
- 导入去重策略标准化

---

## 10. 前端重构建议

## 10.1 页面拆分策略

### Bot 配置页
拆成：
- `BotOverviewPanel`
- `BotIntegrationPanel`
- `BotTrialPolicyPanel`
- `BotCommandManager`
- `BotTemplateEditor`
- `BotVisualPreview`
- `BotDMComposer`
- `BotManagerBindingPanel`

### Bot 用户看板
拆成：
- `BotUserListPanel`
- `BotUserSummaryCards`
- `BotUserConfigForm`
- `BotUserTaskPanel`
- `BotLicensePanel`

### 监听矩阵
拆成：
- `ListenerImportPanel`
- `ListenerAccountTable`
- `ListenerTargetTable`
- `ListenerProxyTable`
- `ListenerAssignPanel`

## 10.2 页面状态管理建议
- 把页面状态从视图里抽到 `stores/` 或 `composables/`
- 例如：
  - `useBotSettingsState`
  - `useBotUserDashboardState`
  - `useListenerAdminState`
  - `useTaskCenterState`

---

## 11. 迁移实施方案

## 阶段 A：只加不删
- 新增新表
- 保留旧表
- 读旧写新，或双写

## 阶段 B：后台切读
- 前端接口逐步切换到新结构
- 旧表只做兼容

## 阶段 C：停止旧写入
- 新功能只写新表
- 旧表只读

## 阶段 D：清理旧结构
- 删除旧 JSON 冗余字段
- 合并临时兼容逻辑

---

## 12. 我对这个项目的最终建议

如果以“继续长期做产品”的标准看，这个项目现在最该优先做的不是继续堆功能，而是这 4 件事：

1. **统一任务中心**
   - 监听任务、私信任务、资料任务、导入任务全部纳入一个任务体系。

2. **拆 Bot 平台**
   - `bot.go` 必须拆，不然后面任何 Bot 功能都会越来越危险。

3. **把可查询业务数据结构化**
   - 尤其是关键词、任务、分组绑定、最近搜索记录、命令菜单。

4. **监听矩阵彻底独立**
   - 监听号、监听群、监听代理和总账号池不要再互相污染。

---

## 13. 推荐的下一步落地顺序

如果你下一步让我直接动手，我建议按这个顺序来：

1. 先拆后端 `models.go` 和 `bot.go`
2. 再把 Bot 私信任务并入统一任务中心
3. 然后重做 Bot 配置页的数据结构
4. 最后重做监听矩阵的数据层和分组关系

这样风险最低，收益最大。

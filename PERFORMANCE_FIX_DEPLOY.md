# 性能优化部署指南

## 本次修改内容

### ✅ 已完成的代码优化

1. **全局任务轮询间隔延长** (`frontend/src/components/GlobalTaskNotifier.vue`)
   - 从 4 秒 → 30 秒
   - 降低数据库查询频率 87%

2. **Dashboard 缓存延长** (`backend/internal/handlers/dashboard.go`)
   - 从 5 秒 → 30 秒
   - 减少聚合查询频率 83%

3. **监听矩阵任务轮询优化** (`frontend/src/views/ListenerAdminView.vue`)
   - 4 个定时器间隔：2.5 秒 → 8 秒
   - 降低任务追踪期间的查询频率 68%

4. **目标池任务轮询优化** (`frontend/src/views/TargetPoolView.vue`)
   - 间隔：2.5 秒 → 8 秒
   - 降低刷新任务期间的查询频率 68%

**预估效果**：数据库 QPS 降低 50-60%，界面卡顿感明显减轻。

---

## 部署步骤

### 方案 A：服务器直接拉取（推荐）

```bash
# 1. SSH 到服务器
ssh root@154.19.242.137

# 2. 进入项目目录
cd /path/to/TG10  # 根据实际路径调整

# 3. 备份当前版本
git stash save "backup-before-perf-fix-$(date +%Y%m%d-%H%M%S)"

# 4. 拉取最新代码
git pull origin main

# 5. 重新构建并重启服务
docker compose down
docker compose up -d --build

# 6. 查看启动日志
docker compose logs -f gateway worker scheduler
```

### 方案 B：本地推送到服务器（如果你改了本地代码）

```bash
# 在本地项目目录执行

# 1. 提交本地修改
git add -A
git commit -m "perf: optimize polling intervals to reduce DB load

- GlobalTaskNotifier: 4s -> 30s
- Dashboard cache: 5s -> 30s  
- Listener admin timers: 2.5s -> 8s
- Target pool timer: 2.5s -> 8s"

# 2. 推送到 GitHub
git push origin main

# 3. 然后在服务器上执行方案 A 的步骤 1-6
```

### 方案 C：手动复制文件（网络不稳定时）

```bash
# 1. 在本地打包修改的文件
tar czf perf-fix.tar.gz \
  frontend/src/components/GlobalTaskNotifier.vue \
  frontend/src/views/ListenerAdminView.vue \
  frontend/src/views/TargetPoolView.vue \
  backend/internal/handlers/dashboard.go

# 2. 上传到服务器
scp perf-fix.tar.gz root@154.19.242.137:/tmp/

# 3. SSH 到服务器解压
ssh root@154.19.242.137
cd /path/to/TG10
tar xzf /tmp/perf-fix.tar.gz

# 4. 重新构建并重启
docker compose down
docker compose up -d --build
```

---

## 验证效果

### 1. 运行诊断脚本（部署后 5 分钟）

```bash
cd /path/to/TG10
bash diagnose.sh

# 查看报告
cat diagnose_*.txt
```

**重点关注指标**：

- PostgreSQL 连接数：应该减少 30-50%
- 慢查询数量：应该显著减少
- Gateway 容器 CPU/内存：应该更平稳

### 2. 前端测试

在浏览器中：

1. 打开开发者工具 → Network 标签
2. 访问 Dashboard 页面，观察 `/api/v1/dashboard` 请求频率
   - **优化前**：每 5 秒一次
   - **优化后**：每 30 秒一次（有缓存时不请求）

3. 访问监听矩阵页面，点击"检测账号状态"
   - 观察 `/api/v1/tasks` 请求频率
   - **优化前**：每 2.5 秒一次
   - **优化后**：每 8 秒一次

### 3. 数据库性能监控

```bash
# 查看实时连接数
docker exec tg-postgres psql -U tg_marketing -d tg_marketing -c "
SELECT count(*), state 
FROM pg_stat_activity 
WHERE datname = 'tg_marketing' 
GROUP BY state;"

# 查看慢查询（持续监控 1 分钟）
docker exec tg-postgres psql -U tg_marketing -d tg_marketing -c "
SELECT pid, now() - query_start as duration, state, left(query, 80) 
FROM pg_stat_activity 
WHERE datname = 'tg_marketing' 
  AND state = 'active' 
  AND now() - query_start > interval '500 milliseconds'
ORDER BY duration DESC;"
```

---

## 回滚方案（如果出问题）

```bash
# 快速回滚到上一个版本
cd /path/to/TG10
git stash list  # 查看备份列表
git stash pop   # 恢复最近的备份

# 或者回滚到指定 commit
git log --oneline -5
git reset --hard <commit-hash>

# 重新构建
docker compose down
docker compose up -d --build
```

---

## 预期改善

| 指标 | 优化前 | 优化后 | 改善 |
|------|--------|--------|------|
| Dashboard 查询频率 | 每 5s | 每 30s | -83% |
| 全局任务轮询频率 | 每 4s | 每 30s | -87% |
| 任务追踪轮询频率 | 每 2.5s | 每 8s | -68% |
| 数据库总 QPS | 估计 10-15 | 估计 4-6 | -50~60% |
| 页面响应延迟 | 500-2000ms | 200-500ms | -60~75% |

---

## 下一步优化（可选）

如果部署后效果好，可以继续：

1. **使用 WebSocket 替代所有轮询**
   - 后端已有 `/api/v1/ws/logs` 端点
   - 扩展协议推送任务状态变更
   - 完全消除轮询开销

2. **添加数据库连接池监控**
   - 部署 PgBouncer
   - 监控连接池使用率

3. **前端组件拆分**
   - ListenerAdminView.vue (2079 行) → 拆分成子组件
   - 提升渲染性能

详见 [`PERFORMANCE_ISSUES.md`](PERFORMANCE_ISSUES.md)

---

## 问题排查

### 前端构建失败

```bash
cd frontend
npm install
npm run build
```

### 后端构建失败

```bash
cd backend
go mod tidy
go build ./cmd/gateway
go build ./cmd/worker
go build ./cmd/scheduler
```

### 容器启动失败

```bash
# 查看具体错误
docker compose logs gateway
docker compose logs worker

# 检查端口占用
netstat -tlnp | grep 8080
netstat -tlnp | grep 36666
```

---

## 联系支持

- 性能问题诊断报告：[`PERFORMANCE_ISSUES.md`](PERFORMANCE_ISSUES.md)
- 诊断脚本：[`diagnose.sh`](diagnose.sh)
- 如需进一步优化，提供 `diagnose_*.txt` 文件内容

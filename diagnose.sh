#!/bin/bash
# TG营销助手性能诊断脚本
# 在服务器上以 root 身份运行：bash diagnose.sh

set -e
REPORT="diagnose_$(date +%Y%m%d_%H%M%S).txt"

echo "========================================" | tee -a "$REPORT"
echo "TG营销助手性能诊断报告" | tee -a "$REPORT"
echo "时间: $(date)" | tee -a "$REPORT"
echo "========================================" | tee -a "$REPORT"

echo -e "\n[1] 系统资源" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
uptime | tee -a "$REPORT"
free -h | tee -a "$REPORT"
df -h / | tee -a "$REPORT"
echo "磁盘 I/O:" | tee -a "$REPORT"
iostat -x 1 3 2>/dev/null | tail -20 | tee -a "$REPORT" || echo "iostat 未安装" | tee -a "$REPORT"

echo -e "\n[2] Docker 容器状态" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker ps -a --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | tee -a "$REPORT"

echo -e "\n[3] 容器资源占用（前10）" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}" | head -11 | tee -a "$REPORT"

echo -e "\n[4] Gateway 最近 100 行日志" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker logs --tail 100 tg-gateway 2>&1 | tee -a "$REPORT"

echo -e "\n[5] Worker 最近 100 行日志" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker logs --tail 100 tg-worker 2>&1 | tee -a "$REPORT"

echo -e "\n[6] Scheduler 最近 50 行日志" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker logs --tail 50 tg-scheduler 2>&1 | tee -a "$REPORT"

echo -e "\n[7] PostgreSQL 状态" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker exec tg-postgres psql -U tg_marketing -d tg_marketing -c "
SELECT
  count(*) as total_connections,
  count(*) FILTER (WHERE state = 'active') as active,
  count(*) FILTER (WHERE state = 'idle') as idle,
  count(*) FILTER (WHERE wait_event_type IS NOT NULL) as waiting
FROM pg_stat_activity
WHERE datname = 'tg_marketing';" 2>&1 | tee -a "$REPORT"

echo -e "\n慢查询（超过 1 秒）:" | tee -a "$REPORT"
docker exec tg-postgres psql -U tg_marketing -d tg_marketing -c "
SELECT
  pid,
  now() - query_start as duration,
  state,
  left(query, 100) as query_preview
FROM pg_stat_activity
WHERE datname = 'tg_marketing'
  AND state = 'active'
  AND now() - query_start > interval '1 second'
ORDER BY duration DESC;" 2>&1 | tee -a "$REPORT"

echo -e "\n表大小 TOP 10:" | tee -a "$REPORT"
docker exec tg-postgres psql -U tg_marketing -d tg_marketing -c "
SELECT
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;" 2>&1 | tee -a "$REPORT"

echo -e "\n[8] Redis 状态" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker exec tg-redis redis-cli -a 'tg_redis_change_this_8b6a4e2f9c1d7a3b' INFO stats 2>&1 | grep -E "total_commands_processed|instantaneous_ops_per_sec|keyspace" | tee -a "$REPORT"
docker exec tg-redis redis-cli -a 'tg_redis_change_this_8b6a4e2f9c1d7a3b' INFO memory 2>&1 | grep -E "used_memory_human|maxmemory" | tee -a "$REPORT"
echo "慢日志（最近 10 条）:" | tee -a "$REPORT"
docker exec tg-redis redis-cli -a 'tg_redis_change_this_8b6a4e2f9c1d7a3b' SLOWLOG GET 10 2>&1 | tee -a "$REPORT"

echo -e "\n[9] NATS JetStream 状态" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker exec tg-nats nats stream list 2>&1 | tee -a "$REPORT" || echo "nats CLI 未安装" | tee -a "$REPORT"
docker exec tg-nats nats stream info TASKS 2>&1 | tee -a "$REPORT" || echo "TASKS stream 未找到或 nats CLI 不可用" | tee -a "$REPORT"

echo -e "\n[10] 网络连接数统计" | tee -a "$REPORT"
echo "---" | tee -a "$REPORT"
docker exec tg-gateway netstat -an 2>/dev/null | awk '/^tcp/ {print $6}' | sort | uniq -c | sort -rn | tee -a "$REPORT" || \
  docker exec tg-gateway ss -tan 2>/dev/null | awk 'NR>1 {print $1}' | sort | uniq -c | sort -rn | tee -a "$REPORT" || \
  echo "netstat/ss 未安装" | tee -a "$REPORT"

echo -e "\n========================================" | tee -a "$REPORT"
echo "诊断完成，报告已保存到: $REPORT" | tee -a "$REPORT"
echo "========================================" | tee -a "$REPORT"
echo ""
echo "请把 $REPORT 文件内容发给我分析"

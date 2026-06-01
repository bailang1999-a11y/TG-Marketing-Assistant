import type { Task, TaskLog } from './task'

export type SCRMKeywordRule = {
  id: string
  tenant_id?: string
  owner_user_id?: string | null
  name: string
  listen_group_id?: string | null
  strike_group_id?: string | null
  monitor_group_id?: string | null
  monitor_terminal_ids?: string[]
  keywords: {
    list?: string[]
    text?: string
    [key: string]: unknown
  }
  match_mode: 'fuzzy' | 'exact'
  push_to_bot: boolean
  strike_enabled: boolean
  status: string
  created_at: string
  updated_at?: string
  creator?: Task['creator'] | null
  bot_user?: Task['bot_user'] | null
}

export type SCRMLead = {
  id: string
  tenant_id?: string
  owner_user_id?: string | null
  source_task_id?: string | null
  target_id: string
  user_nickname?: string
  user_account?: string
  source_chat_id: string
  source_chat_name: string
  trigger_word: string
  trigger_message: string
  message_id: string
  assigned_worker?: string
  status: string
  updated_at?: string
  hit_at?: string | null
  hit_time?: string
  recent_history?: Array<{
    keyword: string
    message: string
    source_chat_name: string
    hit_at: string
  }>
  created_at: string
}

export type SCRMMessage = {
  id: string
  lead_id: string
  sender_type: string
  terminal_id?: string
  content: string
  is_read: boolean
  message_time: string
  created_at: string
}

export type SCRMListenerStatus = {
  running: boolean
  task_id?: string
  rule_id?: string
  started_at?: string
  target_count: number
  covered_target_count?: number
  uncovered_target_count?: number
  terminal_count: number
  match_count: number
  last_event_at?: string
  last_heartbeat_at?: string
  health_status?: string
  health_text?: string
  silence_seconds?: number
  strike_enabled: boolean
  monitor_terminal_labels?: string[]
}

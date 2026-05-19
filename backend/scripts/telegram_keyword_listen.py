#!/usr/bin/env python3
import argparse
import asyncio
import json
import os
import shutil
import signal
import sys
import tempfile
import zipfile
from datetime import datetime, timezone

from opentele.api import API, UseCurrentSession
from opentele.td import TDesktop
from telethon import TelegramClient, events

from telegram_proxy import telethon_proxy_from_json, telethon_use_ipv6_from_json


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False), flush=True)


def resolve_tdata_root(base_dir: str) -> str:
    for root, _, files in os.walk(base_dir):
        if "key_data" in files:
            return root
        if "key_datas" in files:
            legacy_path = os.path.join(root, "key_datas")
            normalized_path = os.path.join(root, "key_data")
            try:
                with open(legacy_path, "rb") as src, open(normalized_path, "wb") as dst:
                    dst.write(src.read())
            except OSError:
                pass
            return root
    return base_dir


def normalize_access_type(access_type: str) -> str:
    value = (access_type or "").strip().lower()
    if value in ("data", "tdata", "tdesktop"):
        return "data"
    return "session"


async def open_session(file_path: str, proxy_config=None, use_ipv6: bool = False) -> TelegramClient:
    client = TelegramClient(
        file_path,
        API.TelegramDesktop.api_id,
        API.TelegramDesktop.api_hash,
        device_model=API.TelegramDesktop.device_model,
        system_version=API.TelegramDesktop.system_version,
        app_version=API.TelegramDesktop.app_version,
        lang_code=API.TelegramDesktop.lang_code,
        system_lang_code=API.TelegramDesktop.system_lang_code,
        receive_updates=True,
        proxy=proxy_config,
        use_ipv6=use_ipv6,
    )
    await client.connect()
    return client


async def open_tdata(file_path: str, proxy_config=None, use_ipv6: bool = False):
    if not zipfile.is_zipfile(file_path):
        raise ValueError("当前 tdata 文件不完整，请重新导入完整 tdata 文件夹或 zip")

    temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_listen_")
    with zipfile.ZipFile(file_path) as archive:
        archive.extractall(temp_dir)

    tdata_root = resolve_tdata_root(temp_dir)
    desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
    if not desktop.isLoaded() or desktop.accountsCount < 1:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise ValueError("未从 tdata 中识别到账户，请重新导入完整目录")

    session_path = os.path.join(temp_dir, "telethon_listen")
    client = await desktop.ToTelethon(
        session=session_path,
        flag=UseCurrentSession,
        api=API.TelegramDesktop,
        receive_updates=True,
        use_ipv6=use_ipv6,
    )
    if proxy_config:
        client.set_proxy(proxy_config)
    await client.connect()
    return client, temp_dir


def match_keyword(text: str, keywords: list[str], mode: str) -> str:
    message = (text or "").strip()
    if not message:
        return ""
    lowered_message = message.lower()
    normalized_mode = (mode or "").strip().lower()
    for keyword in keywords:
        item = (keyword or "").strip()
        if not item:
            continue
        if normalized_mode == "exact":
            if item == message:
                return item
            continue
        if item.lower() in lowered_message:
            return item
    return ""


def format_account(sender) -> str:
    if isinstance(sender, dict):
        username = (sender.get("username") or "").strip()
        if username:
            return f"@{username.lstrip('@')}"
        return str(sender.get("id") or "")
    username = getattr(sender, "username", None)
    if username:
        return f"@{username}"
    sender_id = getattr(sender, "id", None)
    return str(sender_id or "")


def format_nickname(sender) -> str:
    if isinstance(sender, dict):
        return (sender.get("nickname") or sender.get("title") or sender.get("username") or "").strip()
    title = getattr(sender, "title", None) or getattr(sender, "name", None)
    if title:
        return str(title).strip()
    first_name = getattr(sender, "first_name", None) or ""
    last_name = getattr(sender, "last_name", None) or ""
    nickname = " ".join(part.strip() for part in [first_name, last_name] if part and part.strip()).strip()
    if nickname:
        return nickname
    username = getattr(sender, "username", None)
    if username:
        return f"@{username}"
    return ""


def sender_is_bot(sender) -> bool:
    if isinstance(sender, dict):
        return bool(sender.get("bot") or sender.get("is_bot"))
    return bool(getattr(sender, "bot", False))


def sender_is_real_user(sender) -> bool:
    if sender is None or sender_is_bot(sender):
        return False
    if isinstance(sender, dict):
        return bool(sender.get("id") or sender.get("username"))
    # Telethon User objects expose first_name/last_name/bot. Channels and chats
    # expose title/name instead, and must not become lead sources.
    class_name = sender.__class__.__name__.lower()
    if class_name == "user":
        return True
    return hasattr(sender, "first_name") or hasattr(sender, "last_name")


def should_process_sender(event_or_message, sender) -> bool:
    message = getattr(event_or_message, "message", event_or_message)
    if bool(getattr(event_or_message, "out", False)) or bool(getattr(message, "out", False)):
        return False
    return sender_is_real_user(sender)


def sender_id_from_message(message) -> str:
    sender_id = getattr(message, "sender_id", None)
    if sender_id:
        return str(sender_id)
    from_id = getattr(message, "from_id", None)
    user_id = getattr(from_id, "user_id", None)
    if user_id:
        return str(user_id)
    channel_id = getattr(from_id, "channel_id", None)
    if channel_id:
        return str(channel_id)
    return ""


async def resolve_sender(client: TelegramClient, event_or_message):
    message = getattr(event_or_message, "message", event_or_message)
    sender = None
    sender = getattr(message, "sender", None)
    try:
        if sender is None and hasattr(event_or_message, "get_sender"):
            sender = await event_or_message.get_sender()
    except Exception:
        sender = None
    if sender is None:
        try:
            if hasattr(message, "get_sender"):
                sender = await message.get_sender()
        except Exception:
            sender = None
    if sender is None:
        from_id = getattr(message, "from_id", None)
        if from_id is not None:
            try:
                sender = await client.get_entity(from_id)
            except Exception:
                sender = None
    if sender is None:
        sender_id = sender_id_from_message(message)
        if sender_id:
            try:
                sender = await client.get_entity(int(sender_id))
            except Exception:
                sender = {"id": sender_id, "nickname": "", "title": "", "username": ""}
    return sender


def emit_message_event(event_type: str, args, meta: dict, chat, sender, event, message_text: str, trigger_word: str = "") -> None:
    emit(
        {
            "type": event_type,
            "terminal": args.terminal_label,
            "self_sent": bool(getattr(event, "out", False)),
            "sender_is_bot": sender_is_bot(sender),
            "sender_type": "user" if sender_is_real_user(sender) else "non_user",
            "target_id": meta.get("id", ""),
            "source_chat_id": str(getattr(chat, "id", "") or ""),
            "source_chat_name": getattr(chat, "title", None) or meta.get("name", "") or "",
            "message_id": str(getattr(event.message, "id", "") or ""),
            "user_nickname": format_nickname(sender),
            "user_account": format_account(sender),
            "trigger_word": trigger_word,
            "trigger_message": message_text,
            "hit_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat(),
        }
    )


async def resolve_targets(client: TelegramClient, targets: list[dict]) -> list[tuple]:
    resolved = []
    for target in targets:
        identifier = (target.get("identifier") or "").strip()
        target_type = (target.get("type") or "").strip().lower()
        if target_type != "channel":
          emit({"type": "warning", "reason": f"目标 {identifier or target.get('name') or '-'} 暂不支持监听，仅支持公开群组或频道"})
          continue
        try:
            entity = await client.get_entity(identifier)
        except Exception as exc:
            emit({"type": "warning", "reason": f"目标 {identifier or target.get('name') or '-'} 无法挂载监听：{exc}"})
            continue
        resolved.append((entity, target))
    return resolved


async def listen(args) -> int:
    file_path = os.path.abspath(args.file)
    if not os.path.exists(file_path):
        emit({"type": "error", "reason": "本地会话文件不存在"})
        return 0

    try:
        targets = json.loads(args.targets_json or "[]")
    except Exception:
        emit({"type": "error", "reason": "监听目标格式无效"})
        return 0
    try:
        keywords = json.loads(args.keywords_json or "[]")
    except Exception:
        emit({"type": "error", "reason": "关键词格式无效"})
        return 0

    access_type = normalize_access_type(args.access_type)
    client = None
    temp_dir = None
    stop_event = asyncio.Event()

    def request_stop(*_):
        stop_event.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        try:
            loop.add_signal_handler(sig, request_stop)
        except NotImplementedError:
            signal.signal(sig, lambda *_: stop_event.set())

    try:
        proxy_config = telethon_proxy_from_json(args.proxy_json)
        use_ipv6 = telethon_use_ipv6_from_json(args.proxy_json)
        if access_type == "data":
            client, temp_dir = await open_tdata(file_path, proxy_config, use_ipv6)
        else:
            client = await open_session(file_path, proxy_config, use_ipv6)

        if not await client.is_user_authorized():
            emit({"type": "error", "reason": "监听号未授权，需要重新登录"})
            return 0

        resolved = await resolve_targets(client, targets)
        if not resolved:
            emit({"type": "error", "reason": "没有可监听的公开群组或频道"})
            return 0

        entity_map = {getattr(entity, "id", None): meta for entity, meta in resolved}
        chat_entities = [entity for entity, _ in resolved]
        last_seen_message_ids = {}
        for entity in chat_entities:
            try:
                latest = await client.get_messages(entity, limit=1)
                if latest:
                    last_seen_message_ids[getattr(entity, "id", None)] = latest[0].id
            except Exception as exc:
                emit({"type": "warning", "reason": f"初始化消息游标失败：{exc}"})

        @client.on(events.NewMessage(chats=chat_entities))
        async def handle_message(event):
            try:
                chat = await event.get_chat()
                sender = await resolve_sender(client, event)
                if not should_process_sender(event, sender):
                    return
                meta = entity_map.get(getattr(chat, "id", None), {})
                message_text = getattr(event, "raw_text", "") or ""
                trigger_word = match_keyword(message_text, keywords, args.match_mode)
                emit_message_event("message", args, meta, chat, sender, event, message_text, trigger_word)
                chat_id = getattr(chat, "id", None)
                message_id = getattr(event.message, "id", None)
                if chat_id is not None and message_id is not None:
                    previous = last_seen_message_ids.get(chat_id, 0)
                    if message_id > previous:
                        last_seen_message_ids[chat_id] = message_id
                if not trigger_word:
                    return
                emit_message_event("match", args, meta, chat, sender, event, message_text, trigger_word)
            except Exception as exc:
                emit({"type": "warning", "reason": f"处理消息事件失败：{exc}"})

        async def poll_latest_messages():
            while not stop_event.is_set():
                try:
                    for entity in chat_entities:
                        chat_id = getattr(entity, "id", None)
                        if chat_id is None:
                            continue
                        latest_messages = await client.get_messages(entity, limit=5)
                        if not latest_messages:
                            continue
                        latest_messages = sorted(latest_messages, key=lambda item: item.id)
                        previous = last_seen_message_ids.get(chat_id, 0)
                        fresh_messages = [item for item in latest_messages if getattr(item, "id", 0) > previous]
                        if not fresh_messages:
                            continue

                        chat = await client.get_entity(entity)
                        for message in fresh_messages:
                            sender = await resolve_sender(client, message)
                            if not should_process_sender(message, sender):
                                last_seen_message_ids[chat_id] = max(last_seen_message_ids.get(chat_id, 0), getattr(message, "id", 0))
                                continue
                            message_text = getattr(message, "message", "") or ""
                            trigger_word = match_keyword(message_text, keywords, args.match_mode)
                            meta = entity_map.get(chat_id, {})
                            class PollEvent:
                                def __init__(self, msg):
                                    self.message = msg
                                    self.out = getattr(msg, "out", False)
                            event_wrapper = PollEvent(message)
                            emit_message_event("message", args, meta, chat, sender, event_wrapper, message_text, trigger_word)
                            if trigger_word:
                                emit_message_event("match", args, meta, chat, sender, event_wrapper, message_text, trigger_word)
                            last_seen_message_ids[chat_id] = max(last_seen_message_ids.get(chat_id, 0), getattr(message, "id", 0))
                except Exception as exc:
                    emit({"type": "warning", "reason": f"轮询消息失败：{exc}"})

                try:
                    await asyncio.wait_for(stop_event.wait(), timeout=2.0)
                except asyncio.TimeoutError:
                    continue

        async def emit_heartbeat():
            while not stop_event.is_set():
                emit({"type": "heartbeat", "terminal": args.terminal_label})
                try:
                    await asyncio.wait_for(stop_event.wait(), timeout=30.0)
                except asyncio.TimeoutError:
                    continue

        emit({"type": "ready", "terminal": args.terminal_label, "resolved_count": len(chat_entities)})
        poller = asyncio.create_task(poll_latest_messages())
        heartbeat = asyncio.create_task(emit_heartbeat())
        await stop_event.wait()
        poller.cancel()
        heartbeat.cancel()
        try:
            await poller
        except asyncio.CancelledError:
            pass
        try:
            await heartbeat
        except asyncio.CancelledError:
            pass
        return 0
    except Exception as exc:
        emit({"type": "error", "reason": str(exc)})
        return 0
    finally:
        if client is not None:
            await client.disconnect()
        if temp_dir:
            shutil.rmtree(temp_dir, ignore_errors=True)


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--targets-json", required=True)
    parser.add_argument("--keywords-json", required=True)
    parser.add_argument("--match-mode", default="fuzzy")
    parser.add_argument("--terminal-label", default="")
    parser.add_argument("--proxy-json", default="")
    args = parser.parse_args()
    return await listen(args)


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(line_buffering=True)
    raise SystemExit(asyncio.run(main()))

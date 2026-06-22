<div align="center">

# Agent Notify

<p align="center"> 一个面向 AI Agent 的通知配置工具 </p>

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/hellolib/agent-notify.svg)](https://github.com/hellolib/agent-notify/releases)

</div>

## 项目简介

一个面向 AI Agent 的通知配置工具。支持将 Claude Code、Codex 等 Agent 的事件通知推送到飞书、企业微信、Bark 和系统通知。

## 功能特性

|   通知渠道   | 说明 | 绑定方式    |
|:--------|------|---------|
| 🖥️ 系统通知 | 支持 macOS、Linux、Windows 系统通知 |         |
| <img src="assist/logo/feishu.png" width="24" align="absmiddle"> 飞书   | 支持一键扫码绑定、支持飞书机器人消息推送 | 二维码扫描   |
| <img src="assist/logo/qiyeweixin.png" width="24" align="absmiddle"> 企业微信  | 支持通过企业微信群机器人 Webhook 推送通知消息 | Webhook |
| <img src="assist/logo/dingding.png" width="24" align="absmiddle"> 钉钉  | 支持通过钉钉群机器人 Webhook 推送通知消息 | Webhook |
| <img src="assist/logo/bark.png" width="24" align="absmiddle"> Bark  | 支持通过 Bark Webhook URL 推送到 iOS 设备 | Webhook |


### 支持的事件

| 事件 | 说明 | Claude Code | Codex |
|------|------|:---:|:---:|
| `permission_required` | Agent 需要授权（如执行命令） | ✅ | ✅ |
| `input_required` | Agent 等待用户输入 | ✅ | — |
| `run_completed` | 任务执行完成 | ✅ | ✅ |
| `run_failed` | 任务执行失败 | ✅ | — |

说明：

- Claude Code 通过 `~/.claude/settings.json` 的 hooks 订阅四个事件（`PermissionRequest`、`Notification`、`Stop`、`PostToolUseFailure`）。
- Codex 通过 `~/.codex/hooks.json` 订阅 `PermissionRequest` 与 `Stop`，分别映射到 `permission_required` 与 `run_completed`。`input_required` 与 `run_failed` Codex 目前没有对应 hook，因此暂不支持。

### 支持的平台

| 平台 | 架构 | 状态 |
|:---:|:---:|:---:|
| macOS | amd64 / arm64 | ✅ |
| Linux | amd64 / arm64 | ✅ |
| Windows | amd64 | ✅ |

## 快速开始

```bash
npx agent-notify
```

首次运行会从 GitHub Releases 下载当前 npm 包版本对应平台的二进制文件，并安装到：

- macOS / Linux: `~/.agent-notify/agent-notify`
- Windows: `~/.agent-notify/agent-notify.exe`

之后每次运行都会检查本地二进制版本：不存在则自动下载，版本落后则自动更新，否则直接运行。launcher 不会持久修改 PATH，始终用绝对路径执行。

> **注意**: Codex 通过 `~/.codex/hooks.json` 接入官方 hooks 系统，目前仅订阅 `PermissionRequest`、`Stop` 两个事件。首次安装后请在 codex 内运行 `/hooks` 完成 trust 审核。


## 配置说明

> agent-notify 不需要手动处理配置文件，该章节仅是为了说明配置相关信息。

agent-notify 自身配置位于 `~/.agent-notify/config.yaml`。Agent 集成配置位置：

- Claude Code: `~/.claude/settings.json`（写入 hooks → 命令 `agent-notify handle-claude-hook`）
- Codex: `~/.codex/hooks.json`（写入 hooks → 命令 `agent-notify handle-codex-hook`，需在 codex 内运行 `/hooks` 完成 trust）

### 企业微信机器人绑定小技巧

1. **创建单人通知群**：在企业微信中发起群聊（随便拉几个同事），创建成功后**不要在群里发言**，直接将其他人移出，此时该群将变成你的单人通知群；
2. **添加机器人**：「群设置」->「消息推送」->「添加」-> 「自定义消息推送」，命名并保存；
3. **获取 Webhook 地址**：复制生成的地址，格式类似 `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`；
4. **绑定配置**：运行 `npx agent-notify`，在配置向导中选择启用企业微信渠道，粘贴 Webhook URL 即可；
> 旧版企业微信添加机器人步骤：「群设置」->「群机器人」->「添加机器人」-> 「新建机器人」，命名并保存

### Bark 配置说明

1. **复制 Bark URL**：在 Bark App 内复制测试 URL，例如 `https://api.day.app/<key>/这里改成你自己的推送内容`；
2. **绑定配置**：运行 `npx agent-notify`，进入「消息渠道配置」->「初始化 Bark」，粘贴 Bark URL 即可；
3. **Codex 任务完成通知**：在配置文件 `~/.agent-notify/config.yaml` 中保留 Codex 的 `run_completed` 事件，并启用 `notify.codex.channels.bark`。

Bark URL 会作为本地配置保存，发送时使用 Bark 的 POST JSON 参数 `title` 和 `body`。

## 工作流程

<p align="center">
  <img src="assist/workflow.png" alt="工作流程图" />
</p>

## 效果图

| |                                                              |
|:---:|:------------------------------------------------------------:|
| <img src="assist/launch-setting.png" alt="软件配置" width="75%"> |  <img src="assist/feishu-bind.png" alt="飞书绑定" width="75%">   |
| **软件配置** |                           **飞书绑定**                           |
| <img src="assist/feishu-notify-phone.png" alt="飞书通知" width="50%"> | <img src="assist/wecom-notify.jpg" alt="企业微信通知" width="55%"> |
| **飞书通知** |                          **企业微信通知**                          |
| ![系统通知](assist/system-notify.png) |                                                              |
| **系统通知** |                                                              |

## Friendship Link

Thanks for the support and feedback from the friends at [LINUX DO](https://linux.do/).

## 项目架构

```
外部 Agent (Claude Code / Codex / CodeBuddy)
  │  触发 Hook
  ▼
agent-notify handle-xxx-hook < JSON (stdin)
  │
  ▼
┌─ Adapter 层 ───────────────────────┐
│  claudehooks/  │  codexhooks/     │  原始 Hook JSON → 统一 Event
│  codebuddyhooks/                   │  含大小写兼容、字段兼容
└──────────┬────────────────────────┘
           ▼
┌─ Event Protocol ───────────────────┐
│  internal/event/                   │  spec_version, event_id,
│  Event struct + Adapter 接口      │  status, raw_payload
└──────────┬────────────────────────┘
           ▼
┌─ State Machine ────────────────────┐
│  internal/state/session.go         │  状态推断:
│  Advancer.Advance()               │  NEW → ACTIVE → COMPLETED/FAILED
│                                   │  Stop(Codex) → 默认完成
│                                   │  Stop(Claude idle) → 不通知
└──────────┬────────────────────────┘
           ▼
┌─ Dispatch ─────────────────────────┐
│  agenthooks/dispatch.go           │  buildSenders → 按 Agent+事件筛选
│  DispatchEvent()                  │
└──────────┬────────────────────────┘
           ▼
┌─ Notification ─────────────────────┐
│  notify/dispatcher.go             │  去重(10s窗口) + 并发推送
│  Sender 接口 × 9 种实现          │
│  → bark/feishu/dingtalk/         │
│    wxpusher/serverchan/          │
│    pushplus/wechatwork/          │
│    macos/linux/windows           │
└──────────────────────────────────┘
```

### 核心数据结构

```
event.Event                     notify.Message             SessionRecord
┌─────────────────┐            ┌────────────────┐         ┌────────────────┐
│ spec_version    │            │ Agent          │         │ session_id     │
│ event_id        │            │ Event          │         │ agent          │
│ agent           │ ──→        │ SessionID      │         │ status         │
│ hook_event      │ bridge     │ Workspace      │         │ has_run_event  │
│ status          │            │ Title          │         │ notified       │
│ session_id      │            │ Body           │         │ started_at     │
│ workspace       │            │ RawPayload     │         │ updated_at     │
│ title/body      │            └────────────────┘         └────────────────┘
│ raw_payload     │
│ received_at     │
└─────────────────┘
```

## 性能影响

| 方面 | 说明 |
|------|------|
| **CPU** | Hook 触发时进程运行 ~0.5-3s 即退出。无常驻进程 |
| **内存** | Go 二进制 ~32MB 磁盘，运行时峰值 ~10-15MB RSS |
| **网络** | 每次事件发 1-2 个 HTTPS POST |
| **启动时间** | Go 冷启动 ~50ms |
| **磁盘** | 配置 <1KB，状态 <10KB，日志可增长（建议定期清理） |

**不是常驻服务。** 只在 hook 触发时短暂执行，对电脑性能无感知影响。

## 卸载指南

```bash
# 1. 删除二进制
rm /usr/local/bin/agent-notify

# 2. 删除配置和状态
rm -rf ~/.agent-notify

# 3. 清理各 Agent 的 Hook 配置
rm ~/.claude/settings.json          # Claude Code
rm ~/.codex/hooks.json              # Codex
rm ~/.codebuddy/settings.json       # CodeBuddy

# 4. VS Code 扩展：在扩展面板搜索 "agent-notify" → 卸载
```

> ⚠️ `~/.claude/settings.json` 和 `~/.codebuddy/settings.json` 可能包含手动添加的其他配置。
> 如果还有别的设置，不要直接删文件，而是手动删除里面的 `hooks` 块。
> 更安全的做法：运行 `agent-notify doctor` 查看哪些文件被修改过，选择性地清理。


## ❤️ 赞助

感谢 **[DDS（呆呆兽）](https://www.ddshub.cc/register?aff=E7N6PDYWW4N5)** 赞助本项目！呆呆兽是一家专注 Claude 和 CodeX 的可靠高效 API 中转站，为个人和企业用户提供极具性价比的国内 Claude / CodeX API 直连加速服务。支持 Claude Haiku / Opus / Sonnet 等满血模型。企业客户更可享受定制化分组和技术支持服务。

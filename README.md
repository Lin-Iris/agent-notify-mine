<div align="center">

# Agent Notify

<p align="center"> 一个面向 AI Agent 的通知配置工具 </p>

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/Lin-Iris/agent-notify-mine.svg)](https://github.com/Lin-Iris/agent-notify-mine/releases)

</div>

## 项目简介

一个面向 AI Agent 的通知配置工具。支持将 Claude Code、Codex、CodeBuddy 等 Agent 的事件通知推送到微信（Server酱 / PushPlus / WxPusher）、飞书、企业微信、钉钉、Bark 和系统通知。

## 功能特性

|   通知渠道   | 说明 | 绑定方式    |
|:--------|------|---------|
| 🖥️ 系统通知 | 支持 macOS、Linux、Windows 系统通知 | 无需配置 |
| <img src="assist/logo/feishu.png" width="24" align="absmiddle"> 飞书   | 支持一键扫码绑定、支持飞书机器人消息推送 | 二维码扫描   |
| <img src="assist/logo/qiyeweixin.png" width="24" align="absmiddle"> 企业微信  | 支持通过企业微信群机器人 Webhook 推送通知消息 | Webhook |
| <img src="assist/logo/dingding.png" width="24" align="absmiddle"> 钉钉  | 支持通过钉钉群机器人 Webhook 推送通知消息 | Webhook |
| <img src="assist/logo/bark.png" width="24" align="absmiddle"> Bark  | 支持通过 Bark Webhook URL 推送到 iOS 设备 | Webhook |
| 🔔 Server酱 | 通过微信公众号推送通知到微信 | SendKey |
| 📢 PushPlus | 通过微信公众号推送通知到微信 | Token |
| 📲 WxPusher | 通过微信公众号推送通知到微信 | AppToken + UID |


### 支持的事件

| 事件 | 说明 | Claude Code | Codex | CodeBuddy | Cursor | Hermes |
|------|------|:---:|:---:|:---:|:---:|:---:|
| `permission_required` | Agent 需要授权（如执行命令） | ✅ | ✅ | ✅ | ✅ | ✅ |
| `input_required` | Agent 等待用户输入 | ✅ | — | ✅ | — | — |
| `run_completed` | 任务执行完成 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `run_failed` | 任务执行失败 | ✅ | — | ✅ | ✅ | — |

说明：

- Claude Code 通过 `~/.claude/settings.json` 的 hooks 订阅四个事件（`PermissionRequest`、`Notification`、`Stop`、`PostToolUseFailure`）。
- Codex 通过 `~/.codex/hooks.json` 订阅 `PermissionRequest` 与 `Stop`，分别映射到 `permission_required` 与 `run_completed`。`input_required` 与 `run_failed` Codex 目前没有对应 hook，因此暂不支持。
- CodeBuddy 通过 `~/.codebuddy/settings.json` 的 hooks 订阅六个事件（`PermissionRequest`、`Notification`、`Stop`、`SessionEnd`、`PostToolUseFailure`、`PreToolUse`），全部映射到通知事件。
- Cursor 通过 `~/.cursor/hooks.json` 的 hooks 订阅三个事件（`beforeShellExecution`、`stop`、`postToolUseFailure`），映射到 `permission_required`、`run_completed`/`run_failed`。`input_required` Cursor 目前没有对应 hook。
- Hermes 通过 `~/.hermes/config.yaml` 的 hooks 订阅两个事件（`pre_approval_request`、`post_llm_call`），映射到 `permission_required`、`run_completed`。`input_required` 与 `run_failed` Hermes 目前不支持。

普通消息通知只负责提醒：`PermissionRequest` 会被转成 `permission_required` 通知，不接管 Codex 桌面 App 或 Claude/Codex 自身的授权弹窗。只有开启飞书远程对话后，由 broker 启动的受控 Agent 子进程才支持手机审批。

### 支持的平台

| 平台 | 架构 | 状态 |
|:---:|:---:|:---:|
| macOS | amd64 / arm64 | ✅ |
| Linux | amd64 / arm64 | ✅ |
| Windows | amd64 | ✅ |

## 快速开始

### 安装

#### 方式一：一键安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/Lin-Iris/agent-notify-mine/main/install.sh | bash
```

脚本自动检测系统平台，下载对应二进制，安装到 `~/.local/bin/` 并配置 PATH。

#### 方式二：npx（需 Node.js >= 18）

```bash
npx agent-notify-mine
```

首次运行自动下载 Go 二进制到 `~/.agent-notify/`，之后每次运行自动检查更新。无需手动安装。

> ⚠️ 使用此方式前，需先将包发布到 npm。详见下方 [发布到 npm](#发布到-npm)。

#### 方式三：从源码构建

```bash
git clone https://github.com/Lin-Iris/agent-notify-mine.git
cd agent-notify-mine
go build -o ~/.local/bin/agent-notify ./cmd/agent-notify/
export PATH="$HOME/.local/bin:$PATH"
agent-notify --help
```

#### 方式四：手动下载 Release

从 [Releases](https://github.com/Lin-Iris/agent-notify-mine/releases) 下载对应平台的 `.tar.gz` 文件：

```bash
# macOS ARM（M1/M2/M3/M4/M5）
curl -fsSL https://github.com/Lin-Iris/agent-notify-mine/releases/download/v0.11.0/agent-notify-v0.11.0-darwin-arm64.tar.gz -o agent-notify.tar.gz
tar -xzf agent-notify.tar.gz
mv agent-notify-* ~/.local/bin/agent-notify
chmod +x ~/.local/bin/agent-notify
export PATH="$HOME/.local/bin:$PATH"
```

| 你的电脑 | 下载文件 |
|---------|---------|
| Mac (Apple Silicon) | `*-darwin-arm64.tar.gz` |
| Mac (Intel) | `*-darwin-amd64.tar.gz` |
| Linux (amd64) | `*-linux-amd64.tar.gz` |
| Linux (arm64) | `*-linux-arm64.tar.gz` |
| Windows (amd64) | `*-windows-amd64.tar.gz` |

### 配置

```bash
agent-notify init
```

交互式向导会依次询问：
1. 选择 Agent（Claude Code / Codex / CodeBuddy / Cursor / Hermes）
2. 选择配置类型：
   - **消息通知**：配置任务完成、需要授权、失败等事件提醒。
   - **远程飞书对话**：为这个 Agent 配置手机飞书对话入口。
3. 如果选择消息通知，再选择通知渠道（系统通知 / 微信 / 钉钉 / 飞书 / Bark...）和订阅事件。
4. 如果选择远程飞书对话，按提示扫码绑定飞书机器人，并选择是否立即启动远程对话服务。

配置消息通知后，Agent 每次触发事件都会推送通知到你的手机。配置远程飞书对话后，可以直接在手机飞书里给对应 Agent 发任务、看运行卡片和最终结果。

> `agent-notify init` 会自动检测你安装的 Agent：
> - `claude` 命令 → Claude Code CLI
> - VS Code 扩展中的 Claude Code
> - `codex` 命令 → Codex CLI
> - `/Applications/Codex.app` → Codex.app GUI
> - `codebuddy` 命令 → CodeBuddy CLI
> - `~/.codebuddy/settings.json` → CodeBuddy IDE 扩展
>
> 就算没有命令行版 CLI，也能正确配置 hooks。配置后需在 Agent 内运行 `/hooks` 完成信任审核。

## 在其他设备上使用

新设备上安装同样简单，任选一种：

### 方式一：一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/Lin-Iris/agent-notify-mine/main/install.sh | bash
```

### 方式二：npx（需 Node.js >= 18）

```bash
npx agent-notify-mine
```

> ⚠️ 使用前需先[发布到 npm](#发布到-npm)。

### 方式三：从源码构建

```bash
git clone https://github.com/Lin-Iris/agent-notify-mine.git
cd agent-notify-mine
go build -o ~/.local/bin/agent-notify ./cmd/agent-notify/
export PATH="$HOME/.local/bin:$PATH"
```

### 方式四：手动下载 Release

从 [Releases](https://github.com/Lin-Iris/agent-notify-mine/releases) 下载对应平台压缩包，解压到 `~/.local/bin/` 即可。

安装后运行 `agent-notify init` 完成配置。消息通知可用 `agent-notify test` 测试；远程飞书对话可在手机飞书对应机器人窗口里发送消息验证。

第一次配置远程飞书对话前，建议先阅读 [`docs/first-run-troubleshooting.md`](docs/first-run-troubleshooting.md)。里面整理了 Codex.app / Claude Code VS Code 插件用户如何找到 CLI、workspace 未设置、卡片一直运行中、飞书没有回应等常见问题。

## 支持的 Agent

目前支持 **5 个 Agent**，每个 Agent 可独立配置订阅的事件和通知渠道：

| Agent | Hook 配置位置 | 支持的事件数 | 配置命令 |
|-------|-------------|:----------:|---------|
| **Claude Code** | `~/.claude/settings.json` | 4 | `agent-notify init` 选择 Claude Code |
| **Codex** | `~/.codex/hooks.json` | 2 | `agent-notify init` 选择 Codex |
| **CodeBuddy** | `~/.codebuddy/settings.json` | 4 | `agent-notify init` 选择 CodeBuddy |
| **Cursor** | `~/.cursor/hooks.json` | 3 | `agent-notify init` 选择 Cursor |
| **Hermes** | `~/.hermes/config.yaml` | 2 | `agent-notify init` 选择 Hermes |

### Agent 事件对照

| 统一事件 | 含义 | Claude Code | Codex | CodeBuddy | Cursor | Hermes |
|---------|------|:-----------:|:-----:|:---------:|:------:|:------:|
| `permission_required` | 需要用户授权执行操作 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `input_required` | 等待用户输入 | ✅ | — | ✅ | — | — |
| `run_completed` | 任务执行完成 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `run_failed` | 任务执行失败 | ✅ | — | ✅ | ✅ | — |

### Agent 通知设置

```text
Claude Code 推荐事件:
  ├─ permission_required  ← 执行命令前通知
  ├─ input_required       ← 等待输入时通知
  ├─ run_completed        ← 任务完成时通知
  └─ run_failed           ← 任务失败时通知

Codex 推荐事件:
  ├─ permission_required  ← 需要授权时通知
  └─ run_completed        ← 任务完成时通知

CodeBuddy 推荐事件:
  ├─ permission_required  ← 执行高风险操作前通知
  ├─ input_required       ← 空闲超过 60s 时通知
  ├─ run_completed        ← 每轮对话完成时通知
  └─ run_failed           ← 工具执行失败时通知

Cursor 推荐事件:
  ├─ permission_required  ← shell 命令执行前通知（⚠️ 含所有命令）
  ├─ run_completed        ← 任务完成时通知
  └─ run_failed           ← 工具执行失败时通知

Hermes 推荐事件:
  ├─ permission_required  ← 高危命令需要授权时通知
  └─ run_completed        ← 每轮对话完成时通知
```

### Hooks 配置格式

`agent-notify init` 会自动写入正确的 hooks 配置（包含实际的二进制路径，如 `~/.local/bin/agent-notify`）。以下是配置文件的实际 JSON 格式，供手动排查时参考。

> ⚠️ 以下示例中的 `/usr/local/bin/agent-notify` 仅为示意。`agent-notify init` 会根据实际安装位置自动填入正确路径（如 `/Users/你的用户名/.local/bin/agent-notify`）。手动编写 hooks 时请替换为 `which agent-notify` 输出的实际路径。

#### Claude Code（`~/.claude/settings.json`）

在 VS Code 扩展和命令行版 Claude Code 中格式一致。

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-claude-hook"
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-claude-hook"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-claude-hook"
          }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-claude-hook"
          }
        ]
      }
    ]
  }
}
```

> `Stop` 事件官方不支持 `matcher` 字段，始终触发。其他事件都设 `"matcher": ""` 表示匹配全部。

#### Codex（`~/.codex/hooks.json`）

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codex-hook"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codex-hook"
          }
        ]
      }
    ]
  }
}
```

> Codex 的 `~/.codex/config.toml` 中还需要启用 hooks 功能。`agent-notify init` 会自动写入：
> ```toml
> [features]
> hooks = true
> ```

#### CodeBuddy（`~/.codebuddy/settings.json`）

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codebuddy-hook"
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codebuddy-hook"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codebuddy-hook"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codebuddy-hook"
          }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codebuddy-hook"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "execute_command|write_to_file|delete_file|create_file|ask_followup_question|edit_and_apply|edit",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/agent-notify handle-codebuddy-pretooluse"
          }
        ]
      }
    ]
  }
}
```

#### Cursor（`~/.cursor/hooks.json`）

```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "command": "/usr/local/bin/agent-notify handle-cursor-hook"
      }
    ],
    "stop": [
      {
        "command": "/usr/local/bin/agent-notify handle-cursor-hook"
      }
    ],
    "postToolUseFailure": [
      {
        "command": "/usr/local/bin/agent-notify handle-cursor-hook"
      }
    ]
  }
}
```

> ⚠️ Cursor 的 `beforeShellExecution` 对所有 shell 命令触发（不止授权操作），启用 `permission_required` 通知可能会比较频繁。`input_required` 事件 Cursor 目前不支持。

#### Hermes（`~/.hermes/config.yaml`）

```yaml
hooks:
  pre_approval_request:
    - command: "/usr/local/bin/agent-notify handle-hermes-hook"
      timeout: 10
  post_llm_call:
    - command: "/usr/local/bin/agent-notify handle-hermes-hook"
      timeout: 10
hooks_auto_accept: true
```

> Hermes Shell Hooks 不支持 `run_failed` 和 `input_required`。

---

### VS Code 扩展 / Codex.app 支持

`agent-notify init` 与 `agent-notify doctor` 现在支持检测以下安装方式：

| Agent | 检测方式 |
|-------|---------|
| Claude Code CLI | `claude` 命令在 PATH 中 |
| Claude Code VS Code 扩展 | `~/.claude/settings.json` 存在或 VS Code 扩展目录签名 |
| Codex CLI | `codex` 命令在 PATH 中 |
| Codex.app | `/Applications/Codex.app` 或 `~/Applications/Codex.app` 存在 |
| Codex（已配置） | `~/.codex/hooks.json` 或 `~/.codex/config.toml` 存在 |
| CodeBuddy IDE 扩展 | `~/.codebuddy/settings.json` 存在 |
| CodeBuddy CLI | `codebuddy` 命令在 PATH 中 |
| Cursor IDE | `~/.cursor/` 目录存在 或 `/Applications/Cursor.app` |
| Hermes CLI | `~/.hermes/` 目录存在 或 `hermes` 命令在 PATH 中 |

即使没有命令行版 CLI，你也可以正常配置 hooks：

```bash
agent-notify init
# → 会显示 "Claude Code (VS Code 扩展)" 或 "Codex (Codex.app)"
```

配置完成后，在 Agent 内运行 `/hooks` 确认 hooks 已被加载并信任。

> 注意：这里说的是“消息通知 hooks”。如果要使用“远程飞书对话”，电脑上必须能直接执行对应 CLI：Claude Code 需要 `claude`，Codex 需要 `codex`。只有 VS Code 插件或 Codex.app GUI 但没有可调用 CLI 时，普通通知可以工作，远程任务不能执行。

#### Codex.app 的 CLI 路径

如果只安装了 Codex.app，先检查 app 内附带的 CLI：

```bash
"/Applications/Codex.app/Contents/Resources/codex" --version
```

如果可用，写入用户级 PATH：

```bash
mkdir -p ~/.local/bin
ln -sf "/Applications/Codex.app/Contents/Resources/codex" ~/.local/bin/codex
export PATH="$HOME/.local/bin:$PATH"
codex --version
```

如果 Codex.app 不在 `/Applications`，先定位：

```bash
mdfind 'kMDItemFSName == "Codex.app"'
```

然后把实际路径替换到 `ln -sf` 命令中。

#### Claude Code VS Code 插件的 CLI 路径

如果你只通过 VS Code 插件使用 Claude Code，先检查系统是否已有 CLI：

```bash
which claude
claude --version
```

如果没有，可以在 VS Code / Cursor 扩展目录里查找候选可执行文件：

```bash
find ~/.vscode/extensions ~/.vscode-insiders/extensions ~/.cursor/extensions \
  -type f -name claude 2>/dev/null | head
```

找到候选路径后测试：

```bash
"/实际/找到的/claude" --version
```

如果能输出版本，写入用户级 PATH：

```bash
mkdir -p ~/.local/bin
ln -sf "/实际/找到的/claude" ~/.local/bin/claude
export PATH="$HOME/.local/bin:$PATH"
claude --version
```

如果插件目录里找不到可执行 `claude`，说明当前插件版本不能作为远程飞书对话的执行 CLI；消息通知仍可使用，远程飞书对话需要安装可调用的 Claude Code CLI。

---

### 各 Agent 配置详情

#### Claude Code

| 项目 | 说明 |
|------|------|
| Hook 文件 | `~/.claude/settings.json` |
| 支持事件 | `permission_required` / `input_required` / `run_completed` / `run_failed` |
| 通知渠道示例 | WxPusher（与其他 Agent 共用，配置一次即可） |

<details>
<summary>点击展开 Claude Code 通知配置示例</summary>

```bash
# 运行配置向导
agent-notify init
# → 选择 Agent: Claude Code
# → 选择渠道: WxPusher（或其他渠道）
# → 选择事件: 全选四个事件
```

配置后的 YAML：

```yaml
notify:
  claude_code:
    events:
      - permission_required
      - input_required
      - run_completed
      - run_failed
    channels:
      wxpusher:
        enabled: true
        app_token: AT_xxxxx
        uid: UID_xxxxx
```
</details>

---

#### Codex

| 项目 | 说明 |
|------|------|
| Hook 文件 | `~/.codex/hooks.json` |
| 支持事件 | `permission_required` / `run_completed` |
| 通知渠道示例 | WxPusher |
| 额外步骤 | 安装后需在 Codex 内运行 `/hooks` 信任 |

> 注意：Codex 需要在 `~/.codex/config.toml` 中启用 hooks 功能。`agent-notify init` 会自动写入，无需手动操作。

<details>
<summary>点击展开 Codex 通知配置示例</summary>

```bash
agent-notify init
# → 选择 Agent: Codex
# → 选择渠道: WxPusher
# → 选择事件: permission_required + run_completed
```

配置后的 YAML：

```yaml
notify:
  codex:
    events:
      - permission_required
      - run_completed
    channels:
      wxpusher:
        enabled: true
        app_token: AT_xxxxx
        uid: UID_xxxxx
```
</details>

---

#### CodeBuddy

| 项目 | 说明 |
|------|------|
| Hook 文件 | `~/.codebuddy/settings.json` |
| 支持事件 | `permission_required` / `input_required` / `run_completed` / `run_failed` |
| 通知渠道示例 | WxPusher |

<details>
<summary>点击展开 CodeBuddy 通知配置示例</summary>

```bash
agent-notify init
# → 选择 Agent: CodeBuddy
# → 选择渠道: WxPusher
# → 选择事件: 全选四个事件
```

配置后的 YAML：

```yaml
notify:
  codebuddy:
    events:
      - permission_required
      - input_required
      - run_completed
      - run_failed
    channels:
      wxpusher:
        enabled: true
        app_token: AT_xxxxx
        uid: UID_xxxxx
```
</details>

---

### 通知渠道配置（按渠道）
| 🖥️ 系统通知 | 本地推送 | 无 | macOS / Linux / Windows |
| <img src="assist/logo/feishu.png" width="20" align="absmiddle"> 飞书 | 机器人消息 | 飞书 App 扫码绑定 | — |
| <img src="assist/logo/dingding.png" width="20" align="absmiddle"> 钉钉 | 群机器人 Webhook | Webhook URL | — |
| <img src="assist/logo/qiyeweixin.png" width="20" align="absmiddle"> 企业微信 | 群机器人 Webhook | Webhook URL | — |
| <img src="assist/logo/bark.png" width="20" align="absmiddle"> Bark | HTTP API | 推送 URL | iOS |
| 🔔 Server酱 | 微信服务号推送 | SendKey (SCU...) | 需关注服务号 |
| 📢 PushPlus | 微信服务号推送 | Token | 需关注服务号 |
| 📲 WxPusher | 微信服务号推送 | AppToken + UID | 需关注服务号 |

### 各渠道配置步骤

#### 🖥️ 系统通知

无需配置。`agent-notify init` 时默认启用，Agent 事件触发时会在电脑上弹出系统通知。

- macOS：基于 `osascript` 或 `terminal-notifier`
- Linux：基于 `notify-send`
- Windows：基于 PowerShell `NotifyIcon`

---

#### 📲 WxPusher（微信推送）

通过微信公众号推送消息到微信。

```bash
# 交互式配置
agent-notify
# → 消息渠道配置 → WxPusher
```

**获取凭证：**

1. 打开 [WxPusher 官网](https://wxpusher.zjiecode.com/)，微信扫码登录
2. 创建应用，获取 **AppToken**
3. 扫码关注你的应用公众号
4. 在「用户管理」页面找到你的 **UID**

**手动配置**（`~/.agent-notify/config.yaml`）：

```yaml
notify:
  codex:
    channels:
      wxpusher:
        enabled: true
        app_token: AT_xxxxx
        uid: UID_xxxxx
```

---

#### 🔔 Server酱（微信推送）

通过微信公众号推送消息到微信。需要 SendKey。

```bash
# 交互式配置
agent-notify
# → 消息渠道配置 → Server酱
```

**获取凭证：**

1. 打开 [Server酱官网](https://sct.ftqq.com/)，微信扫码登录
2. 进入「发送消息」页面，复制你的 **SendKey**（以 `SCU` 开头）

---

#### 📢 PushPlus（微信推送）

通过微信公众号推送消息到微信。

```bash
# 交互式配置
agent-notify
# → 消息渠道配置 → PushPlus
```

**获取凭证：**

1. 打开 [PushPlus 官网](https://www.pushplus.plus/)，微信扫码登录
2. 进入「发送消息」→「一对一推送」，复制 **Token**

---

#### <img src="assist/logo/feishu.png" width="20" align="absmiddle"> 飞书

支持一键扫码绑定，无需手动填写 Webhook。

```bash
agent-notify
# → 消息渠道配置 → 飞书
```

首次配置会打开浏览器，扫码授权飞书账号即可。

---

#### <img src="assist/logo/dingding.png" width="20" align="absmiddle"> 钉钉

通过群机器人 Webhook 推送通知。

```bash
# 交互式配置
agent-notify
# → 消息渠道配置 → 钉钉
```

**获取 Webhook：**

1. 打开钉钉，创建或进入一个群
2. 群设置 → 智能群助手 → 添加机器人 → 选择「自定义」
3. 复制机器人的 **Webhook URL**
4. 粘贴到配置向导中

---

#### <img src="assist/logo/qiyeweixin.png" width="20" align="absmiddle"> 企业微信

通过群机器人 Webhook 推送通知。

```bash
# 交互式配置
agent-notify
# → 消息渠道配置 → 企业微信
```

**获取 Webhook：**

1. 企业微信中创建群聊（可拉同事后移出，变成单人通知群）
2. 群设置 → 群机器人 → 添加机器人 → 新建机器人
3. 复制 **Webhook URL**（格式：`https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`）
4. 粘贴到配置向导中

---

#### <img src="assist/logo/bark.png" width="20" align="absmiddle"> Bark（iOS 推送）

推送到 iPhone。需要安装 Bark App。

```bash
# 交互式配置
agent-notify
# → 消息渠道配置 → Bark
```

**获取推送 URL：**

1. iPhone 上安装 [Bark App](https://apps.apple.com/app/bark/id1403753865)
2. 打开 App，复制推送 URL（格式：`https://api.day.app/你的Key`）
3. 粘贴到配置向导中

---

### 验证配置

配置完成后，发送测试通知确认：

```bash
agent-notify test
```

选择对应渠道，手机会收到测试消息。


## 配置说明

> agent-notify 不需要手动处理配置文件，该章节仅是为了说明配置相关信息。

agent-notify 自身配置位于 `~/.agent-notify/config.yaml`。Agent 集成配置位置：

- Claude Code: `~/.claude/settings.json`（hooks 命令: `agent-notify handle-claude-hook`）
- Codex: `~/.codex/hooks.json`（hooks 命令: `agent-notify handle-codex-hook`，需在 Codex 内运行 `/hooks` 完成 trust）
- CodeBuddy: `~/.codebuddy/settings.json`（hooks 命令: `agent-notify handle-codebuddy-hook`，PreToolUse: `agent-notify handle-codebuddy-pretooluse`）
- Cursor: `~/.cursor/hooks.json`（hooks 命令: `agent-notify handle-cursor-hook`）
- Hermes: `~/.hermes/config.yaml`（hooks 命令: `agent-notify handle-hermes-hook`）

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
| **CPU** | 普通通知模式只在 Hook 触发时短暂运行；broker 模式会有一个用户显式启动的常驻长连接进程 |
| **内存** | Go 二进制 ~32MB 磁盘，运行时峰值 ~10-15MB RSS |
| **网络** | 每次事件发 1-2 个 HTTPS POST |
| **启动时间** | Go 冷启动 ~50ms |
| **磁盘** | 配置 <1KB，状态 <10KB，日志可增长（建议定期清理） |

普通通知模式不是常驻服务。启用飞书审批 broker 后，只有 `agent-notify broker run` 这一条长连接进程会常驻；关闭通信或执行 `agent-notify broker stop` 会拒绝待审批并停止受控 Agent 子进程。

## 飞书远程对话 / 审批 Broker

> Broker 控制的是本机 `codex` / `claude` CLI 子进程，不接管当前 Codex 桌面 App 聊天窗口。手机和电脑不需要在同一网络；电脑端 broker 主动连飞书云端，手机通过飞书下发任务。
>
> 飞书远程审批只对 broker 启动的受控任务生效。未开启远程对话时，同一个飞书机器人仍可收到普通通知，但授权类通知只会提示你回电脑处理，不会显示可点击审批按钮。

推荐入口仍然是：

```bash
agent-notify init
# → 选择 Agent
# → 选择配置类型: 远程飞书对话
# → 按提示扫码绑定
```

远程飞书对话初版支持 Claude Code 和 Codex。配置后会自动写入对应 profile：

| Agent | Profile | 手机飞书入口 |
|------|---------|--------------|
| Claude Code | `claude-main` | 该 Agent 绑定的飞书机器人 |
| Codex | `codex-main` | 该 Agent 绑定的飞书机器人 |

如果需要脚本化或高级配置，也可以使用：

```bash
agent-notify profile feishu setup claude-main
agent-notify profile feishu setup codex-main
```

### 开启和控制

```bash
# 开启本机 broker 通信和审批；init 远程飞书对话完成后也可选择自动启动
agent-notify broker start

# 手动前台运行飞书长连接（调试时使用；broker start 会自动后台启动）
agent-notify broker run

# 向飞书发送一张显性控制卡
agent-notify broker card

# 查看历史对话列表卡
agent-notify broker card --view threads
```

控制卡会显示当前 profile、通信状态、workspace、权限模式、待审批数量和运行任务数量，并提供这些按钮：

| 按钮 | 行为 |
|------|------|
| 开启通信 | 启用 broker、启用审批、启用当前 profile |
| 关闭通信 / 断开并清理 | 禁用 broker、关闭当前 profile、拒绝 pending approval、停止该 profile 的受控进程 |
| 查看对话 | 打开当前项目的历史对话列表 |
| 停止任务 | 停止当前 profile 下由 broker 启动的 Agent 子进程 |
| 新建对话 | 在当前项目下新建一个对话窗口 |
| 刷新状态 | 重新发送最新状态控制卡 |

### 飞书命令

飞书里给对应机器人发送普通文本，会进入该机器人绑定 profile 的受控 Codex / Claude Code 子进程。以下命令不会进入 Agent，而是由 broker 处理：

```text
/status
/cd <path>
/ws list
/ws save <name>
/ws use <name>
/ws remove <name>
/new
/connect
/stop
/ps
/exit <id|pid>
/disconnect
/threads
/thread new <title>
/thread use <id|#>
/thread rename <id|#> <title>
/thread archive <id|#>
/tail <task_id|#> [lines]
/log <task_id|#>
/result <task_id|#>
/home
/back
```

审批消息绑定 `approval_id`、一次性 token、workspace、工具/命令摘要、发起时间和操作人 open_id。远程对话优先使用 profile 级 `feishu.owner_open_id` 鉴权；也可以在全局 `broker.admin_open_ids`、`broker.allowed_open_ids` 中显式加入其他操作人。

更多手机端完整体验、跨网络前提、多对话窗口和日志查看说明见 [`docs/feishu-broker.md`](docs/feishu-broker.md)。

首次使用或遇到异常时，见 [`docs/first-run-troubleshooting.md`](docs/first-run-troubleshooting.md)。常见问题包括：Codex.app CLI 路径、Claude Code VS Code 插件 CLI 路径、workspace 未设置、任务卡一直运行中、Codex `readonly database`、飞书没有回应。

> 💡 macOS 用户可以创建快捷指令一键开关远程，不用每次开终端。详见 [`docs/feishu-broker.md#macos-快捷指令`](docs/feishu-broker.md#macos-快捷指令)。

## 卸载指南

### 场景一：只保留消息通知，关闭飞书远程功能

如果配置了飞书远程对话，后来只想用消息通知，不需要卸载整个项目：

```bash
# 1. 断开所有 profile（关闭远程对话功能，拒绝待审批，停止受控进程）
agent-notify broker stop --profile claude-main
agent-notify broker stop --profile codex-main

# 2. (可选) 删除 profile 中的飞书凭证，只保留消息通知配置
# 编辑 ~/.agent-notify/config.yaml，删除或清空对应 profile 的 feishu 字段
```

也可以直接在飞书控制卡里点击「断开并清理」按钮。broker 后台进程会在所有 profile 关闭后自动退出。

此时消息通知 hooks 不受影响，Agent 事件照常推送。

### 场景二：暂时关闭远程对话（之后会重新开启）

如果只是暂时不用手机飞书远程对话，不需要卸载：

```bash
# 关闭当前 active profile 的远程功能
agent-notify broker stop

# 关闭指定 profile
agent-notify broker stop --profile claude-main
agent-notify broker stop --profile codex-main
```

`broker stop` 会：禁用 profile、拒绝待审批、停止受控 Agent 子进程、停止 broker 后台守护进程。

之后恢复使用：

```bash
# 重新开启远程对话
agent-notify broker start

# 确认状态正常
agent-notify broker card --profile claude-main
```

也可以在飞书控制卡里点击「暂停通信」（保留 daemon，只暂停当前 profile）或「断开并清理」（完全断开并停止 daemon）。

### 场景三：彻底卸载整个项目

根据你的安装方式选择：

#### install.sh 安装的

```bash
# 1. 先停止 broker 守护进程
agent-notify broker stop

# 2. 清理所有配置、hooks、状态、日志
agent-notify clean --purge

# 3. 删除二进制
rm ~/.local/bin/agent-notify

# 4. （可选）从 ~/.zshrc 或 ~/.bashrc 中删除 PATH 行
# export PATH=”$HOME/.local/bin:$PATH”
```

#### npx 安装的

```bash
# 1. 停止 broker 守护进程
npx agent-notify-mine broker stop

# 2. 清理所有配置、hooks、状态、日志
npx agent-notify-mine clean --purge

# 3. 确认缓存目录已删除
rm -rf ~/.agent-notify
```

#### 从源码构建的

```bash
agent-notify broker stop
agent-notify clean --purge
rm ~/.local/bin/agent-notify       # 或你 go build 指定的路径
```

> ⚠️ 重要：`clean --purge` 之前必须先 `broker stop`，否则 broker 后台守护进程可能继续运行。
>
> `clean` 只删除 agent-notify 写入的 hooks 条目，Agent 配置文件（`settings.json` 等）中的其他用户配置不受影响。
>
> VS Code 扩展需在扩展面板手动卸载，与上述方式无关。

### `agent-notify clean` 清理内容

| 清理项 | `clean` | `clean --purge` |
|--------|:------:|:-------------:|
| `~/.agent-notify/config.yaml` | ✅ 重置为默认（全部关闭） | ✅ 整个目录删 |
| `~/.agent-notify/state.json`（去重状态） | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/sessions.json`（会话状态） | ❌ 不删 | ✅ 整个目录删 |
| `~/.agent-notify/agent-notify.log`（日志） | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/approvals.json` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/input_requests.json` | ❌ 不删 | ✅ 整个目录删 |
| `~/.agent-notify/processes.json` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/threads.json` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/tasks.json` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/views.json` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/broker.pid` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/audit.log` | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/logs/`（运行日志） | ✅ 删 | ✅ 整个目录删 |
| `~/.agent-notify/` 整个目录 | ❌ 保留 | ✅ 删 |
| pending approvals | ✅ 拒绝 | ✅ 拒绝 |
| 受控 Agent 子进程 | ✅ 停止 | ✅ 停止 |
| 远程飞书对话 profile（飞书凭证） | ✅ 重置为默认 | ✅ 整个目录删 |
| broker 后台守护进程 | ❌ 不杀（需先手动 `broker stop`） | ❌ 不杀（需先手动 `broker stop`） |
| Claude Code hooks | ✅ 移除自身条目 | ✅ 移除自身条目 |
| Codex hooks | ✅ 移除自身条目 | ✅ 移除自身条目 |
| CodeBuddy hooks | ✅ 移除自身条目 | ✅ 移除自身条目 |
| Cursor hooks | ✅ 移除自身条目 | ✅ 移除自身条目 |
| Hermes hooks | ✅ 移除自身条目 | ✅ 移除自身条目 |
| 二进制本身 | ❌ 不删 | ❌ 不删 |
| npm 全局包 | ❌ 不删 | ❌ 不删 |

## 发布到 npm

为了让用户通过 `npx agent-notify-mine` 一键使用，需要将 npx 包装器发布到 npm。

### 首次发布

```bash
# 1. 注册 npm 账号
#    打开 https://www.npmjs.com/signup 注册（免费）

# 2. 在终端登录 npm
cd npx
npm login
#    输入用户名、密码、邮箱（注册时填的）

# 3. 发布
npm publish
```

发布成功后，任何装了 Node.js 的设备都能用：

```bash
npx agent-notify-mine init
```

### 后续更新

每次 Go 二进制发新版后，更新 `npx/package.json` 的 `version` 字段，然后：

```bash
cd npx
npm version patch        # 小版本号 +0.0.1，或手动改 package.json
npm publish
```

> 注：npx 包装器不再维护测试文件，发布时 `package.json` 中 `"files"` 可改为 `["bin", "lib"]`。

## ❤️ 赞助

感谢 **[DDS（呆呆兽）](https://www.ddshub.cc/register?aff=E7N6PDYWW4N5)** 赞助本项目！呆呆兽是一家专注 Claude 和 CodeX 的可靠高效 API 中转站，为个人和企业用户提供极具性价比的国内 Claude / CodeX API 直连加速服务。支持 Claude Haiku / Opus / Sonnet 等满血模型。企业客户更可享受定制化分组和技术支持服务。

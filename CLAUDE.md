# agent-notify-go

基于 [hellolib/agent-notify](https://github.com/hellolib/agent-notify) 的增强版。Go CLI 工具，通过 Agent 自身的 hook 机制捕获"任务完成"/"需要授权"/"等待输入"等事件，推送通知到手机。

**已支持 Agent**：Claude Code、Codex、CodeBuddy
**已支持推送通道**：WxPusher、Server酱、PushPlus、钉钉、飞书、企业微信、Bark、系统通知

---

## 架构速览

```
用户运行 ./agent-notify
  → cli/app.go: 无参数 → runMenu (交互菜单)
  → cli/app.go: 有参数 → cobra 命令路由

Agent hook 被触发时：
  → ~/.claude/settings.json (或 ~/.codebuddy/settings.json, ~/.codex/hooks.json)
  → 执行 agent-notify handle-xxx-hook
  → stdin 传入 JSON
  → xxxhooks.ParseMessage() 解析为 notify.Message
  → agenthooks.Dispatch() 构建 Sender 列表
  → notify.Dispatcher.SendAll() 去重 + 并发发送
```

### 核心 Interface

```go
// 通知通道：实现 Name() + Send()
type Sender interface {
    Name() string
    Send(ctx context.Context, msg Message) error
}

// Agent 集成：实现安装/卸载 hook 到 agent 配置文件中
type Integration interface {
    Name() string
    DetectInstalled() bool
    SettingsPath(scope string) (string, error)
    Install(settingsPath, binaryPath string) error
    Uninstall(settingsPath string) error
    IsHookInstalled(settingsPath string) (bool, error)
}
```

---

## 目录结构

```
cmd/agent-notify/main.go           # 入口
internal/
├── config/config.go               # YAML 配置结构、默认值、读写
├── notify/                        # 推送通道（Sender 接口实现）
│   ├── message.go                 # Message 结构 + Sender 接口定义
│   ├── dispatcher.go              # 去重分发器
│   ├── sender.go                  # 系统通知工厂（macos/linux/windows）
│   ├── format.go                  # 消息标题/正文格式化
│   ├── serverchan.go              # Server酱 (新增)
│   ├── pushplus.go                # PushPlus (新增)
│   ├── wxpusher.go                # WxPusher (新增)
│   ├── dingtalk.go / feishu.go / wechatwork.go / bark.go  # 原有
│   └── macos.go / linux.go / windows.go / unsupported.go
├── agenthooks/dispatch.go         # buildSenders(): 根据 agent+事件 构建 Sender 列表
├── agentintegrations/             # Agent 集成（Integration 接口实现）
│   ├── integration.go             # Integration 接口
│   ├── claude.go                  # Claude Code → ~/.claude/settings.json
│   ├── codex.go                   # Codex → ~/.codex/hooks.json
│   └── codebuddy.go               # CodeBuddy → ~/.codebuddy/settings.json (新增)
├── claudehooks/                   # Claude Code hook JSON 解析 + settings 读写
│   ├── event.go                   # ParseMessage(): Stop/PermissionRequest/Notification/PostToolUseFailure
│   ├── handler.go                 # Handle(): stdin → Parse → Dispatch
│   └── settings.go                # Install/Uninstall/IsInstalled
├── codebuddyhooks/                # CodeBuddy hook JSON 解析 + settings 读写 (新增)
│   ├── event.go                   # ParseMessage(): Stop/Notification(idle_prompt/permission_prompt)/SessionEnd
│   ├── handler.go
│   └── settings.go
├── codexhooks/                    # Codex hook 解析 (原项目自带)
├── cli/                           # TUI 交互界面 (cobra + promptui)
│   ├── app.go                     # Run(): 无参数→菜单 / 有参数→cobra
│   ├── root.go                    # cobra 命令注册
│   ├── menu.go                    # 主菜单 / 渠道菜单 / 测试菜单 / 清理
│   ├── init.go                    # init 命令：安装 hook + 配置通道
│   ├── handler_claude.go          # handle-claude-hook 子命令
│   ├── handler_codebuddy.go       # handle-codebuddy-hook 子命令 (新增)
│   ├── handler_codex.go           # handle-codex-hook 子命令
│   ├── actions.go                 # 系统通知/飞书/企微配置动作
│   ├── actions_serverchan.go      # Server酱 配置+测试 (新增)
│   ├── actions_pushplus.go        # PushPlus 配置+测试 (新增)
│   ├── actions_wxpusher.go        # WxPusher 配置+测试 (新增)
│   ├── actions_bark.go / actions_dingtalk.go
│   ├── test.go / doctor.go / claude.go / version.go / prompt.go
├── state/                         # 去重状态 + 日志文件
│   ├── dedupe.go                  # ReserveSend/MarkSent: 去重窗口内不重复
│   └── logfile.go                 # AppendLog: 追加日志行
└── common/path.go                 # ResolveBinaryPath: 二进制路径解析
```

---

## 关键数据流

### 配置加载

```
config.Load(path)
  → YAML 解析
  → 补默认值（InstallScope, Events, Channels）
  → 返回 Config 结构体

config.Config {
  Agent    AgentConfig      // 各 agent 的 enabled/installScope
  Notify   NotifyConfig     // 每个 agent 的 events + channels
  Behavior BehaviorConfig   // dedupe_seconds/send_timeout/locale
}
```

### Hook 触发 → 通知发送

```
1. Claude Code 触发 Stop hook
2. 执行: /path/to/agent-notify handle-claude-hook < JSON
3. handler_claude.go → claudehooks.Handle()
   → 读 stdin → ParseMessage() → notify.Message{
       Agent: "claude_code", Event: "run_completed", ...
     }
4. agenthooks.Dispatch()
   → buildSenders(cfg, msg)
     → 根据 msg.Agent 选择 notifyCfg (ClaudeCode/Codex/CodeBuddy)
     → 检查事件是否在 notifyCfg.Events 中
     → 遍历所有 enabled 的 channel，创建 Sender
   → dispatcher.SendAll(ctx, msg)
     → 每个 Sender 检查去重窗口（ReserveSend）
     → 发送 (sender.Send)
     → 记录状态 (MarkSent)
```

### 添加新通知通道的步骤

1. `internal/config/config.go`：在 ChannelsConfig 加配置字段 + 结构体
2. `internal/notify/xxx.go`：实现 `Sender` 接口（Name + Send）
3. `internal/agenthooks/dispatch.go`：buildSenders() 加 case
4. `internal/cli/actions_xxx.go`：配置向导 + 测试函数
5. `internal/cli/menu.go`：测试菜单 + 渠道菜单加选项

### 添加新 Agent 的步骤

1. `internal/xxxhooks/`：event.go（ParseMessage）+ settings.go（Install/Uninstall）
2. `internal/agentintegrations/xxx.go`：实现 Integration 接口
3. `internal/config/config.go`：AgentConfig + NotifyConfig + Default + Load 加字段
4. `internal/agenthooks/dispatch.go`：buildSenders() 加 agent 识别
5. `internal/cli/handler_xxx.go`：cobra 子命令
6. `internal/cli/root.go`：注册命令
7. `internal/cli/menu.go`：清理逻辑加通道重置 + Agent 清理

---

## 事件映射

| Agent Hook 事件 | 统一事件名 | 含义 |
|------|------|------|
| Stop | `run_completed` | 任务完成 |
| PermissionRequest / Notification(permission_prompt) | `permission_required` | 需要授权 |
| Notification(idle_prompt) / Notification(waiting for input) | `input_required` | 等待用户输入 |
| PostToolUseFailure | `run_failed` | 任务失败 |

---

## 配置

- 用户配置：`~/.agent-notify/config.yaml`（YAML，TUI 自动生成）
- Claude Code hook：`~/.claude/settings.json`（自动写入）
- CodeBuddy hook：`~/.codebuddy/settings.json`（自动写入）
- Codex hook：`~/.codex/hooks.json`（自动写入）
- 去重状态：`~/.agent-notify/state.json`
- 日志：`~/.agent-notify/agent-notify.log`

## 编译 & 运行

```bash
go build -o agent-notify ./cmd/agent-notify/
./agent-notify              # 交互菜单
./agent-notify init         # 直接进入初始化流程
./agent-notify test         # 测试通知
./agent-notify doctor       # 环境诊断
```

## 本次增强内容

相对于上游 hellolib/agent-notify，新增：
- 3 个微信推送通道：Server酱、PushPlus、WxPusher
- CodeBuddy Agent 集成（hook 解析 + settings 读写 + CLI handler）
- 默认 dedupe_seconds 从 60→10（减少授权通知丢失）

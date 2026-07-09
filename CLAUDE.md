# agent-notify-go

基于 [hellolib/agent-notify](https://github.com/hellolib/agent-notify) 的增强版。Go CLI 工具，通过 Agent 自身的 hook 机制捕获"任务完成"/"需要授权"/"等待输入"/"任务失败"等事件，推送通知到手机。支持远程审批（飞书卡片双向交互）和 Broker 模式（长连接代理）。

**已支持 Agent**：Claude Code、Codex、CodeBuddy、Cursor、Hermes
**已支持推送通道**：WxPusher、Server酱、PushPlus、钉钉、飞书、企业微信、Bark、系统通知

---

## 架构速览

```
用户运行 ./agent-notify
  → cli/app.go: 无参数 → runMenu (交互菜单)
  → cli/app.go: 有参数 → cobra 命令路由

Agent hook 被触发时：
  → ~/.claude/settings.json (或其他 agent 配置文件)
  → 执行 agent-notify handle-xxx-hook
  → stdin 传入 JSON
  → xxxhooks.ParseMessage() 解析为 event.Event（规范事件）
  → 状态机 (state.Advancer) 推进会话状态
  → agenthooks.DispatchEvent() 构建 Sender 列表
  → notify.Dispatcher.SendAll() 去重 + 并发发送

远程审批/输入流程：
  → PermissionRequest/input_required hook 触发
  → agenthooks.MaybeHandleApproval() / MaybeHandleInput()
  → 发送飞书交互卡片 → 阻塞等待用户决策/输入
  → 决策返回给 Agent hook stdout
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

// 事件适配器：将原始 hook JSON 规范化为 event.Event
type Adapter interface {
    AgentName() string
    Parse(raw []byte) (Event, error)
}
```

---

## 目录结构

```
cmd/agent-notify/main.go              # 入口
internal/
├── event/event.go                    # 规范事件协议 (Event + Status + Adapter 接口)
├── config/config.go                  # YAML 配置结构、默认值、读写
├── notify/                           # 推送通道（Sender 接口实现）
│   ├── message.go                    # Message 结构 + Sender 接口定义
│   ├── dispatcher.go                 # 去重分发器
│   ├── sender.go                     # 系统通知工厂（macos/linux/windows）
│   ├── format.go                     # 消息标题/正文格式化
│   ├── feishu.go                     # 飞书通知（含交互卡片）
│   ├── serverchan.go                 # Server酱
│   ├── pushplus.go                   # PushPlus
│   ├── wxpusher.go                   # WxPusher
│   ├── dingtalk.go                   # 钉钉
│   ├── wechatwork.go                 # 企业微信
│   ├── bark.go                       # Bark
│   └── macos.go / linux.go / windows.go / unsupported.go
├── agenthooks/                       # Hook 分发核心
│   ├── dispatch.go                   # buildSenders() + DispatchEvent() + Dispatch()
│   ├── approval.go                   # MaybeHandleApproval(): 远程审批（飞书卡片→阻塞等待）
│   └── input.go                      # MaybeHandleInput(): 远程输入（飞书卡片→阻塞等待）
├── agentintegrations/                # Agent 集成（Integration 接口实现）
│   ├── integration.go                # Integration 接口
│   ├── claude.go                     # Claude Code → ~/.claude/settings.json
│   ├── codex.go                      # Codex → ~/.codex/hooks.json
│   ├── codebuddy.go                  # CodeBuddy → ~/.codebuddy/settings.json
│   ├── cursor.go                     # Cursor → ~/.cursor/settings.json
│   └── hermes.go                     # Hermes → ~/.hermes/config.yaml
├── claudehooks/                      # Claude Code hook JSON 解析 + settings 读写
│   ├── event.go                      # ParseMessage(): Stop/PermissionRequest/Notification/PostToolUseFailure
│   ├── handler.go                    # Handle(): stdin → Parse → DispatchEvent
│   └── settings.go                   # Install/Uninstall/IsInstalled
├── codebuddyhooks/                   # CodeBuddy hook 解析 + settings 读写
│   ├── event.go                      # ParseMessage(): Stop/Notification/SessionEnd/PostToolUseFailure
│   ├── handler.go                    # Handle(): Stop 用文件+后台进程做防抖
│   ├── pretooluse.go                 # PreToolUse handler: 拦截工具调用做安全过滤
│   └── settings.go
├── codexhooks/                       # Codex hook 解析
│   ├── event.go                      # ParseMessage()
│   ├── handler.go                    # Handle()
│   ├── features.go                   # Codex 特性检测
│   └── settings.go
├── cursorhooks/                      # Cursor hook 解析
│   ├── event.go                      # ParseMessage(): beforeShellExecution/Stop/SessionEnd
│   ├── handler.go                    # Handle()
│   └── settings.go
├── hermeshooks/                      # Hermes hook 解析
│   ├── event.go                      # ParseMessage()
│   ├── handler.go                    # Handle()
│   ├── settings.go                   # ~/.hermes/config.yaml 读写
│   └── config_yaml.go                # Hermes 特定配置格式
├── state/                            # 状态持久化
│   ├── dedupe.go                     # ReserveSend/MarkSent: 去重窗口内不重复
│   ├── session.go                    # SessionStore + Advancer: 会话状态机
│   └── logfile.go                    # AppendLog: 追加日志行
├── approval/                         # 远程审批请求存储
│   └── store.go                      # Create/Wait/Update: 审批生命周期
├── inputrequest/                     # 远程输入请求存储
│   └── store.go                      # Create/Wait/Update: 输入请求生命周期
├── feishubridge/                     # 飞书 WebSocket 网关（Broker 长连接）
│   ├── gateway_ws.go                 # WebSocket 客户端 + 消息路由
│   └── types.go                      # 消息类型定义
├── feishucli/                        # 飞书 CLI 客户端
│   └── client.go                     # 飞书 API 调用封装
├── agentprocess/                     # Agent 进程注册表
│   └── registry.go                   # 进程 PID 注册/查询/清理
├── threadstore/                      # 会话线程存储
│   └── store.go                      # 线程/任务持久化
├── app/                              # 应用服务层
│   ├── doctor/service.go             # 环境诊断
│   ├── setup/service.go              # 初始化流程
│   └── tester/service.go             # 通知测试
├── cli/                              # TUI 交互界面 (cobra + promptui)
│   ├── app.go                        # Run(): 无参数→菜单 / 有参数→cobra
│   ├── root.go                       # cobra 命令注册
│   ├── menu.go                       # 主菜单 / 渠道菜单 / 测试菜单 / 清理
│   ├── init.go                       # init 命令：安装 hook + 配置通道
│   ├── broker.go                     # broker 命令：远程代理管理
│   ├── profile.go                    # profile 命令：多 profile 管理
│   ├── thread_task.go                # thread/task/ps/kill 命令：会话管理
│   ├── clean.go                      # clean 命令：清理配置/hook/状态
│   ├── claude.go / codex.go / codebuddy.go / cursor.go / hermes.go  # Agent 子命令
│   ├── handler_claude.go             # handle-claude-hook 子命令
│   ├── handler_codex.go              # handle-codex-hook 子命令
│   ├── handler_codebuddy.go          # handle-codebuddy-hook 子命令
│   ├── handler_codebuddy_pretooluse.go  # handle-codebuddy-pretooluse 子命令
│   ├── handler_cursor.go             # handle-cursor-hook 子命令
│   ├── handler_hermes.go             # handle-hermes-hook 子命令
│   ├── actions.go                    # 系统通知/飞书/企微配置动作
│   ├── actions_serverchan.go         # Server酱 配置+测试
│   ├── actions_pushplus.go           # PushPlus 配置+测试
│   ├── actions_wxpusher.go           # WxPusher 配置+测试
│   ├── actions_bark.go               # Bark 配置+测试
│   ├── actions_dingtalk.go           # 钉钉 配置+测试
│   ├── test.go / doctor.go / version.go / prompt.go
├── common/path.go                    # ResolveBinaryPath: 二进制路径解析
└── project/                          # 项目工具
```

---

## 关键数据流

### 事件协议 (event.Event)

```
原始 hook JSON → Adapter.Parse() → event.Event{
    SpecVersion, EventID, Agent, HookEvent,
    Status (最佳猜测), SessionID, Workspace,
    Title, Body, RawPayload, ReceivedAt
}
→ state.Advancer.Advance() → 状态机推进 → AdvanceDecision{Notify, Status, Reason}
→ DispatchEvent() → 检查事件是否启用 → buildSenders() → SendAll()
```

### 配置加载

```
config.Load(path)
  → YAML 解析
  → 补默认值（InstallScope, Events, Channels, Broker, Approval, Profiles）
  → 返回 Config 结构体

config.Config {
  Version  int
  Agent    AgentConfig      // 各 agent 的 enabled/installScope (ClaudeCode/Codex/CodeBuddy/Cursor/Hermes)
  Notify   NotifyConfig     // 每个 agent 的 events + channels
  Behavior BehaviorConfig   // dedupe_seconds/send_timeout/locale
  Broker   BrokerConfig     // broker 远程代理配置
  Approval ApprovalConfig   // 远程审批配置
  Profiles ProfilesConfig   // 多 profile 配置（飞书凭证 + workspace 映射）
}
```

### Hook 触发 → 通知发送

```
1. Claude Code 触发 Stop hook
2. 执行: /path/to/agent-notify handle-claude-hook < JSON
3. handler_claude.go → claudehooks.Handle()
   → 读 stdin → ParseMessage() → event.Event{
       Agent: "claude_code", Status: StatusPending, HookEvent: "Stop", ...
     }
4. agenthooks.DispatchEvent()
   → state.NewAdvancer(sessionStore).Advance(evt)
     → 状态机: StatusPending + "Stop" → StatusCompleted, Notify=true
   → 检查事件是否在 notifyCfg.Events 中
   → buildSenders(cfg, msg)
     → 根据 msg.Agent 选择 notifyCfg
     → 遍历所有 enabled channel，创建 Sender
     → Profile 飞书凭证优先于全局默认
   → dispatcher.SendAll(ctx, msg)
     → 去重窗口检查 (ReserveSend)
     → 并发发送 (sender.Send)
     → 记录状态 (MarkSent)
```

### 远程审批流程

```
1. Claude Code 触发 PermissionRequest hook
2. handler_claude.go → Handle()
   → ParseMessage() → event.Event{Status: StatusPermissionReq}
   → MaybeHandleApproval()
     → 检查 broker enabled + approval enabled + profile 飞书凭证
     → 创建 approval.Request 存入 approval.Store
     → 发送飞书审批卡片（交互式）
     → 立即向其他渠道推送"等待授权"通知（SkipFeishu=true 避免重复）
     → 阻塞等待用户飞书回复（默认 300s 超时）
     → 返回 allow/deny 决策给 hook stdout
```

### 远程输入流程

```
1. Claude Code 触发 idle_prompt (input_required)
2. handler_claude.go → Handle()
   → ParseMessage() → event.Event{Status: StatusInputRequired}
   → MaybeHandleInput()
     → 发送飞书输入卡片
     → 向其他渠道推送通知（SkipFeishu=true）
     → 阻塞等待用户飞书回复
     → 返回用户输入给 hook stdout
```

---

## 会话状态机

状态机 (`state.Advancer`) 跟踪每个 session 生命周期，决定是否通知及通知状态：

| 输入事件 | 状态转换 | 通知 |
|---------|---------|------|
| Stop / SessionEnd | → Completed | ✅ run_completed |
| PermissionRequest | → Active | ✅ permission_required |
| Notification(idle_prompt) / PreToolUse | → Active | ✅ input_required |
| PostToolUseFailure / StatusFailed | → Failed | ✅ run_failed |
| 未知事件 (StatusPending) | 保持当前 | ❌ |

状态机确保每个 session 只发一次终端通知（Completed/Failed 后 `Notified=true`）。

---

## 事件映射

| Agent Hook 事件 | 规范 Status | 通知事件名 | 含义 |
|------|------|------|------|
| Stop | `completed` | `run_completed` | 任务完成 |
| SessionEnd | `completed` | `run_completed` | 会话结束 |
| PermissionRequest | `permission_required` | `permission_required` | 需要授权 |
| Notification(idle_prompt / waiting for input) | `input_required` | `input_required` | 等待用户输入 |
| PreToolUse | `input_required` | `input_required` | 工具调用前拦截 |
| PostToolUseFailure | `failed` | `run_failed` | 任务失败 |

---

## 配置

- 用户配置：`~/.agent-notify/config.yaml`（YAML，TUI 自动生成）
- 会话状态：`~/.agent-notify/sessions.json`
- 去重状态：`~/.agent-notify/state.json`
- 审批请求：`~/.agent-notify/approvals.json`
- 输入请求：`~/.agent-notify/input_requests.json`
- 进程注册：`~/.agent-notify/processes.json`
- 审计日志：`~/.agent-notify/audit.log`
- 线程/任务：`~/.agent-notify/threads.json` / `tasks.json`
- 日志：`~/.agent-notify/agent-notify.log`
- Broker PID：`~/.agent-notify/broker.pid`

Agent hook 配置：
- Claude Code：`~/.claude/settings.json`
- Codex：`~/.codex/hooks.json`
- CodeBuddy：`~/.codebuddy/settings.json`
- Cursor：`~/.cursor/settings.json`
- Hermes：`~/.hermes/config.yaml`

## 编译 & 运行

```bash
go build -o agent-notify ./cmd/agent-notify/
./agent-notify              # 交互菜单
./agent-notify init         # 直接进入初始化流程
./agent-notify test         # 测试通知
./agent-notify doctor       # 环境诊断
./agent-notify broker       # 启动远程代理（Broker 模式）
./agent-notify profile      # 管理多 profile 配置
./agent-notify thread       # 查看活跃会话线程
./agent-notify task         # 查看后台任务
./agent-notify ps           # 查看 agent 进程
./agent-notify kill         # 终止 agent 进程
./agent-notify clean        # 清理配置/hook/状态
```

## 添加新通知通道的步骤

1. `internal/config/config.go`：在 ChannelsConfig 加配置字段 + 结构体
2. `internal/notify/xxx.go`：实现 `Sender` 接口（Name + Send）
3. `internal/agenthooks/dispatch.go`：buildSenders() 加 case
4. `internal/cli/actions_xxx.go`：配置向导 + 测试函数
5. `internal/cli/menu.go`：测试菜单 + 渠道菜单加选项

## 添加新 Agent 的步骤

1. `internal/xxxhooks/`：event.go（Adapter.Parse）+ settings.go（Install/Uninstall）+ handler.go
2. `internal/agentintegrations/xxx.go`：实现 Integration 接口
3. `internal/config/config.go`：AgentConfig + NotifyConfig + Default + Load 加字段
4. `internal/agenthooks/dispatch.go`：notifyConfigForAgent() + buildSenders() 加 agent 识别
5. `internal/cli/handler_xxx.go`：cobra 子命令
6. `internal/cli/xxx.go`：agent 子命令（配置管理）
7. `internal/cli/root.go`：注册命令
8. `internal/cli/menu.go`：清理逻辑加通道重置 + Agent 清理

---

## 本次增强内容

相对于上游 hellolib/agent-notify，新增：
- 3 个微信推送通道：Server酱、PushPlus、WxPusher
- 3 个 Agent 集成：CodeBuddy、Cursor、Hermes（hook 解析 + settings 读写 + CLI）
- 远程审批系统：飞书交互卡片 + 阻塞等待决策 + 审批存储
- 远程输入系统：飞书卡片收集用户输入 + 回调 Agent
- Broker 模式：飞书 WebSocket 长连接代理，支持远程启动/管理 Agent
- 多 Profile 支持：不同 workspace 可用不同飞书凭证
- 会话状态机：精确跟踪 session 生命周期，避免重复/遗漏通知
- 规范事件协议 (`event.Event`)：统一所有 Agent hook 的事件表示
- 默认 dedupe_seconds 从 60→10（减少授权通知丢失）

# 飞书远程对话 / Broker 手机端控制台

## 运行前提

飞书 Broker 控制的是电脑上的受控 CLI 子进程：

```text
手机飞书 -> 飞书云端 -> 电脑 agent-notify broker -> codex / claude CLI
```

前提：

- 电脑必须开机、联网，并运行远程对话服务；推荐通过 `agent-notify init` 配置后选择立即启动。
- 电脑必须能访问飞书开放平台。
- 电脑必须安装并能调用 `codex` CLI 或 `claude` CLI。
- Codex UI 不是必需；但如果只有不可调用 CLI 的桌面 GUI，无法执行远程任务。Codex.app 用户通常可以把 `/Applications/Codex.app/Contents/Resources/codex` 链接到 `~/.local/bin/codex`。
- Claude Code VS Code 插件用户如果没有 `claude` 命令，需要先在扩展目录里找到可执行 CLI，并链接到 `~/.local/bin/claude`。
- 手机和电脑不需要同一个网络。电脑在深圳、手机在福建也可以，只要电脑 broker 长连接在线。
- 项目文件必须存在于电脑本地。

首次使用和排障详见 [`first-run-troubleshooting.md`](first-run-troubleshooting.md)。

## 首次配置

推荐使用交互式向导：

```bash
agent-notify init
# 选择 Agent
# 选择配置类型: 远程飞书对话
# 按提示扫码绑定
```

配置完成后会自动写入对应 profile：

| Agent | Profile | 说明 |
|------|---------|------|
| Claude Code | `claude-main` | Claude Code 的远程飞书对话入口 |
| Codex | `codex-main` | Codex 的远程飞书对话入口 |

配置完成后，先发控制台卡确认状态：

```bash
agent-notify broker card --profile claude-main
agent-notify broker card --profile codex-main
```

如果控制台卡显示当前项目未设置，请在对应 Agent 的飞书机器人里发送：

```text
/cd /你的/具体/项目路径
```

不要把用户目录、桌面、下载目录或系统目录设为 workspace。

如果需要在脚本里配置，也可以使用高级命令：

```bash
agent-notify profile feishu setup claude-main
agent-notify profile feishu setup codex-main
```

手动传参仍可用：

```bash
agent-notify profile feishu claude-main --app-id ... --app-secret ... --owner-open-id ...
agent-notify profile feishu codex-main --app-id ... --app-secret ... --owner-open-id ...
```

## 手机端完整体验

1. 在电脑端通过 `agent-notify init` 配置远程飞书对话，并选择启动远程对话服务。也可以手动运行：

```bash
agent-notify broker start
agent-notify broker card
```

2. 手机飞书收到控制台卡，能看到：

- 通信状态
- 当前 profile
- 当前项目
- 当前对话窗口
- Agent 类型
- 运行任务数
- 待审批数
- CLI 可用性诊断

3. 控制台卡按钮：

- `开启通信`：启用 broker 通信和当前 profile。
- `查看对话`：打开当前项目的历史对话列表。
- `新建对话`：在当前项目下创建新的对话窗口。
- `停止任务`：停止当前 profile 的受控任务。
- `刷新状态`：重新发送控制台卡。
- `断开并清理`：拒绝待审批、停止受控任务、关闭 broker；下次需要电脑端重新 `broker start`。

## 多对话窗口

模型是：

```text
profile -> workspace/project -> thread/conversation -> task
```

同一个项目可以有多个对话窗口。飞书命令：

```text
/threads
/thread new <title>
/thread use <id|#>
/thread rename <id|#> <title>
/thread archive <id|#>
```

对话列表卡支持：

- 进入对话
- 查看结果
- 新建对话
- 上一页 / 下一页
- 返回控制台

每个子界面都必须有返回入口，避免手机端点进去后无法回到原界面。

## 任务结果与过程

在对应 Agent 的飞书机器人里直接发普通文本，会启动该 profile 的受控任务。任务开始后会发送任务卡，并在运行过程中更新同一张卡：

- 任务状态
- 对话 ID
- 项目路径
- 模型输出预览
- 最终结果
- 模型输出
- 返回对话
- 返回控制台
- 停止任务

飞书默认不刷屏。运行中卡片会节流更新；任务结束后仍更新同一张卡为最终结果。点击“模型输出”会展示模型可见的流式输出过程和上游显式输出的 reasoning/summary 类片段，不读取运行日志，也不会展示或伪造模型隐藏的私有推理链。

命令查看：

```text
/tail <task_id|#> [lines]
/log <task_id|#>
/result <task_id|#>
```

`/result` 查看模型输出；`/tail` 和 `/log` 保留为排查入口，面向开发调试。

如果原卡片更新失败，broker 会补发一张新的失败/完成卡。旧的“任务正在运行”卡可能仍留在飞书里，此时以最新控制台卡和最新任务卡为准。

## Claude / Codex 续聊

Claude Code：

- 新对话优先使用 `--session-id`。
- 继续对话使用 `--resume`。
- 输出使用 `--output-format=stream-json` 捕获。

Codex：

- 新任务使用 `codex exec --json`。
- 继续对话使用 `codex exec resume <session_id> <prompt> --json`。

如果某次无法稳定取得原生 session id，会降级为本地 thread + 摘要续接，并在任务状态里记录 `native_resume=false`。

## 开启、暂停、断开

- `开启通信`：手机端可以继续发任务和审批。
- `暂停通信`：保留 broker listener，只关闭当前 profile；手机端可重新开启。
- `断开并清理`：关闭该 profile、拒绝待审批并停止受控任务；如果没有其他 profile 在线，会停止 broker listener。下次可在电脑上执行：

```bash
agent-notify broker start
agent-notify broker card
```

命令行关闭：

```bash
agent-notify broker stop
agent-notify broker stop --profile claude-main
agent-notify broker stop --profile codex-main
```

`broker stop` 会尝试停止该 profile 的受控任务，并清理后台 broker 长连接。多次从不同终端启动过 broker 时，建议执行 stop 后再 start，避免旧连接继续接收飞书事件。

## 常见问题

### 手机发消息后一直显示任务正在运行

先在电脑端检查：

```bash
agent-notify broker status --profile codex-main
agent-notify broker command --profile codex-main /ps
```

如果 `processes=0` 或任务已经是 `exited_error`，说明任务并没有继续运行，通常是旧卡片没有被更新。新版会在原卡更新失败时补发新的失败/完成卡。

处理：

```bash
agent-notify broker stop --profile codex-main
agent-notify broker start
agent-notify broker card --profile codex-main
```

### Codex 显示 readonly database 或 app-server 初始化失败

常见错误：

```text
failed to initialize in-process app-server client
attempt to write a readonly database
Operation not permitted
```

含义是 broker 启动的 Codex CLI 没有权限读写 `~/.codex`。先在电脑终端直接测试：

```bash
codex --ask-for-approval on-request exec --json --sandbox workspace-write --cd /你的/项目路径 --skip-git-repo-check "你好"
```

如果终端可用但飞书不可用，重启 broker。如果终端也失败，先修复 Codex CLI 或 `~/.codex` 权限。

### 手机发消息没有任何回应

检查：

- 是否在正确的 Agent 飞书机器人窗口发消息。
- `agent-notify broker status --profile <name>` 是否在线。
- 飞书开放平台是否启用了 `im.message.receive_v1` 接收消息事件。
- 修改飞书后台事件后是否重启了 broker。

更多问题见 [`first-run-troubleshooting.md`](first-run-troubleshooting.md)。

## 本地状态与卸载

新增状态文件：

```text
~/.agent-notify/threads.json
~/.agent-notify/tasks.json
~/.agent-notify/views.json
~/.agent-notify/logs/runs/<profile>-<thread>-<task>.log
```

彻底清理：

```bash
agent-notify clean --purge
```

清理会删除 broker 状态、审批、对话窗口、任务索引、飞书导航状态、profile 飞书机器人配置和运行日志，只移除 agent-notify 写入的 hooks，不删除用户自己的 Claude/Codex 其他配置。

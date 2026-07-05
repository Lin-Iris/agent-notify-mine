# 首次使用排障指南

这份文档面向第一次使用 `agent-notify` 的用户。先区分两个功能：

- **消息通知**：Agent 已经在电脑里运行，`agent-notify` 只在需要授权、完成、失败等事件发生时发通知。
- **远程飞书对话**：你在手机飞书里发消息，由电脑上的 `agent-notify broker` 启动本机 `claude` 或 `codex` CLI 执行任务。

远程飞书对话不是云端托管。电脑必须开机、联网、broker 在线，并且本机能直接调用对应 CLI。

## 推荐安装位置

统一使用用户级安装：

```bash
mkdir -p ~/.local/bin
export PATH="$HOME/.local/bin:$PATH"
```

如果你使用 zsh，把 PATH 写入：

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

确认当前使用的是用户级二进制：

```bash
which agent-notify
agent-notify --version
```

期望 `which agent-notify` 输出类似：

```text
/Users/你的用户名/.local/bin/agent-notify
```

如果同时存在 `/usr/local/bin/agent-notify`，容易运行到旧版本。建议删除旧二进制或调整 PATH 顺序。

## Codex App 用户：找到可调用的 Codex CLI

远程飞书对话需要能执行 `codex` 命令。只安装 Codex.app GUI 时，手机端不能直接控制 GUI 聊天窗口；broker 会启动 Codex app 内附带的 CLI。

先查系统是否已经能调用：

```bash
which codex
codex --version
```

如果没有，常见路径是：

```bash
/Applications/Codex.app/Contents/Resources/codex
~/Applications/Codex.app/Contents/Resources/codex
```

检查：

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

然后把实际路径替换到 `ln -sf` 命令里。

### Codex 后台报 readonly database

如果飞书任务卡显示：

```text
failed to initialize in-process app-server client
attempt to write a readonly database
Operation not permitted
```

通常表示 broker 启动的 Codex CLI 没有权限读写 `~/.codex`，尤其是 `~/.codex/state_*.sqlite`。

处理顺序：

1. 在电脑终端直接运行同样的 Codex CLI，确认 CLI 本身可用：

```bash
codex --ask-for-approval on-request exec --json --sandbox workspace-write --cd /你的/项目路径 --skip-git-repo-check "你好"
```

2. 如果终端里也失败，先修复 Codex 本身或 `~/.codex` 权限。
3. 如果终端里成功，但飞书里失败，停止后重新启动 broker：

```bash
agent-notify broker stop --profile codex-main
agent-notify broker start
agent-notify broker card --profile codex-main
```

## Claude Code VS Code 插件用户：找到可调用的 Claude CLI

消息通知可以只依赖 Claude Code 的 hook 配置；但远程飞书对话需要电脑上能执行 `claude` 命令。

先查：

```bash
which claude
claude --version
```

如果没有命令，但你是通过 VS Code 插件使用 Claude Code，可以先找插件目录里的可执行文件：

```bash
find ~/.vscode/extensions ~/.vscode-insiders/extensions ~/.cursor/extensions \
  -type f -name claude 2>/dev/null | head
```

也可以先看 Claude 相关扩展目录：

```bash
find ~/.vscode/extensions ~/.vscode-insiders/extensions ~/.cursor/extensions \
  -maxdepth 2 -iname '*claude*' 2>/dev/null
```

找到候选路径后先测试：

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

如果插件目录里没有可执行的 `claude`，说明该插件版本没有提供可独立调用的 CLI。此时：

- 消息通知仍可继续使用。
- 远程飞书对话需要另行安装 Claude Code CLI，或等插件提供可调用 CLI。

## 远程飞书对话的首次使用顺序

推荐流程：

```bash
agent-notify init
# 选择 Agent
# 选择配置类型：远程飞书对话
# 按提示扫码绑定
# 选择是否现在启动远程对话服务
```

启动后发一张控制台卡：

```bash
agent-notify broker card --profile codex-main
# 或
agent-notify broker card --profile claude-main
```

在飞书控制台卡里确认：

- 通信状态已开启。
- CLI 可用。
- 当前项目不是“未设置”。
- 运行任务数为 0。

## 当前项目未设置

如果卡片显示当前项目未设置，或者任务失败提示 workspace 未设置，请在对应 Agent 的飞书机器人里发送：

```text
/cd /你的/具体/项目路径
```

例如：

```text
/cd /Users/victoria/vibe-coding/agent完成提示/agent-notify-go
```

不要把用户目录、桌面、下载目录或系统目录设置成 workspace，例如：

```text
/Users/你的用户名
/Users/你的用户名/Desktop
/Users/你的用户名/Downloads
/
```

这些目录范围太大，会被拒绝。

## 任务卡一直显示“任务正在运行”

先判断是不是真的还在运行：

```bash
agent-notify broker status --profile codex-main
agent-notify broker command --profile codex-main /ps
```

如果 `processes=0`，或者 `/ps` 里任务已经是 `exited_error`，手机里的“运行中”通常是旧卡片没有成功更新。新版会在更新失败时补发一张失败卡。

如果仍然异常，清理并重启 broker：

```bash
agent-notify broker stop --profile codex-main
agent-notify broker start
agent-notify broker card --profile codex-main
```

多次启动、从不同终端启动、或旧版本 stop 没有清干净时，可能会残留旧飞书长连接。重启后只以最新控制台卡为准。

## 手机飞书没有任何回应

检查顺序：

1. 电脑是否在线，broker 是否在线：

```bash
agent-notify broker status --profile codex-main
```

2. 是否在正确的机器人窗口里发消息。Claude 和 Codex 如果分别绑定了不同飞书机器人，要在对应机器人里发。
3. 飞书开放平台是否启用了“接收消息”事件 `im.message.receive_v1`，并开通了读取单聊/群聊消息权限。
4. 修改飞书后台事件后，重启 broker：

```bash
agent-notify broker stop --profile codex-main
agent-notify broker start
```

5. 发一张控制台卡确认通路：

```bash
agent-notify broker card --profile codex-main
```

## 飞书卡片里出现 `message_read_v1 not found handler`

这是“消息已读”事件，不是用户发来的文本消息。它通常不影响任务执行，但会干扰排查。

真正需要的是：

```text
im.message.receive_v1
```

如果只有 read 事件，没有 receive 事件，手机发消息不会进入 broker。

## 任务编号为什么不是 #1

任务编号按当前 profile + workspace + 对话窗口递增。失败、停止过的任务也会占用编号。

例如你连续发了三次“你好”，前两次失败，第三次仍会显示 `#3`。这不代表有三个任务正在运行，只代表这是这个对话窗口里的第 3 个任务记录。

## 常用排查命令

```bash
# 当前 broker 状态
agent-notify broker status --profile codex-main

# 发送控制台卡
agent-notify broker card --profile codex-main

# 在本机模拟飞书 slash 命令
agent-notify broker command --profile codex-main /status
agent-notify broker command --profile codex-main '/cd /你的/项目路径'
agent-notify broker command --profile codex-main /ps

# 查看 profile 是否绑定飞书机器人
agent-notify profile list

# 关闭远程对话
agent-notify broker stop --profile codex-main
```


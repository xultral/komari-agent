# komari-agent

监控专用 agent。当前仓库版本只保留监控数据采集与上报能力，已移除远程终端、远程命令执行、远程 ping 任务和自动更新能力。

## 安装

Linux/macOS:

```bash
wget -qO- https://raw.githubusercontent.com/xultral/komari-agent/refs/heads/main/install.sh | sudo bash -s -- -e http://192.168.100.3:25774 -t YOUR_TOKEN --disable-web-ssh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/xultral/komari-agent/refs/heads/main/install.ps1 | iex
```

示例中的面板地址已按当前环境写成 `http://192.168.100.3:25774`，`YOUR_TOKEN` 需要替换成你实际生成的 agent token，不建议把真实 token 写进仓库。

说明：

- `--disable-web-ssh` 仍保留为兼容参数，但远程控制已经永久禁用。
- `--disable-auto-update` 仍保留为兼容参数，但自动更新已经永久禁用。
- `--show-warning` 仍保留为兼容参数，但在当前版本中不执行任何操作。
- `install.sh` 与 `install.ps1` 默认从 `xultral/komari-agent` 的 GitHub Releases 下载二进制。
- 如需临时改成别的发布仓库，可追加 `--install-repo owner/repo`。

## 面板接入

如果你的面板还在生成原仓库命令：

```bash
wget -qO- https://raw.githubusercontent.com/komari-monitor/komari-agent/refs/heads/main/install.sh | sudo bash -s -- -e http://192.168.100.3:25774 -t YOUR_TOKEN --disable-web-ssh
```

应改成：

```bash
wget -qO- https://raw.githubusercontent.com/xultral/komari-agent/refs/heads/main/install.sh | sudo bash -s -- -e http://192.168.100.3:25774 -t YOUR_TOKEN --disable-web-ssh
```

如果暂时改不了面板模板，也可以在命令末尾补一个发布源覆盖：

```bash
wget -qO- https://raw.githubusercontent.com/xultral/komari-agent/refs/heads/main/install.sh | sudo bash -s -- -e http://192.168.100.3:25774 -t YOUR_TOKEN --disable-web-ssh --install-repo xultral/komari-agent
```

## 本地构建

```bash
go test ./...
./build_all.sh
```

Windows:

```powershell
go test ./...
.\build_all.ps1
```

## 发布

发布前先打 tag 并创建 GitHub Release。安装脚本会按如下规则下载：

- 最新版：`https://github.com/xultral/komari-agent/releases/latest/download/<binary>`
- 指定版：`https://github.com/xultral/komari-agent/releases/download/<tag>/<binary>`

具体流程见 [RELEASE.md](/D:/Temp/komari/komari-agent/RELEASE.md)。

与官方版的差异及后续同步维护方式见 [FORK_NOTES.md](/D:/Temp/komari/komari-agent/FORK_NOTES.md)。

## 代码入口

配置与启动入口见 `cmd/root.go`，参数定义见 `cmd/flags/flag.go`。

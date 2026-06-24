# Release Guide

本仓库的安装脚本默认从 `xultral/komari-agent` 的 GitHub Releases 下载二进制。

## 发布前检查

发布前至少确认以下事项：

- `install.sh` 默认仓库为 `xultral/komari-agent`
- `install.ps1` 默认仓库为 `xultral/komari-agent`
- README 中的 raw 脚本地址指向 `xultral/komari-agent`
- `go test ./...` 通过
- 需要发布的二进制名称与脚本下载名一致

当前脚本期望的文件名：

- `komari-agent-linux-amd64`
- `komari-agent-linux-arm64`
- `komari-agent-linux-386`
- `komari-agent-linux-arm`
- `komari-agent-darwin-amd64`
- `komari-agent-darwin-arm64`
- `komari-agent-freebsd-amd64`
- `komari-agent-freebsd-arm64`
- `komari-agent-freebsd-386`
- `komari-agent-freebsd-arm`
- `komari-agent-windows-amd64.exe`
- `komari-agent-windows-arm64.exe`
- `komari-agent-windows-386.exe`

## 本地发布流程

1. 运行测试

```bash
go test ./...
```

2. 本地构建检查

```bash
./build_all.sh
```

Windows:

```powershell
.\build_all.ps1
```

3. 提交代码并打 tag

```bash
git tag vX.Y.Z
git push origin main --tags
```

4. 在 GitHub 创建 `vX.Y.Z` Release

- 如果启用了仓库里的 release workflow，GitHub Actions 会自动构建并上传对应二进制。
- `install.sh --install-version vX.Y.Z` 和 `install.ps1 --install-version vX.Y.Z` 会下载这个版本。

## 安装脚本下载规则

Linux/macOS 脚本：

- 最新版：`https://github.com/xultral/komari-agent/releases/latest/download/<binary>`
- 指定版：`https://github.com/xultral/komari-agent/releases/download/<tag>/<binary>`

Windows 脚本：

- 先调用 `https://api.github.com/repos/xultral/komari-agent/releases/latest` 获取最新版 tag
- 再从 `https://github.com/xultral/komari-agent/releases/download/<tag>/<binary>` 下载

## 面板命令替换

如果面板还在输出原仓库：

```bash
wget -qO- https://raw.githubusercontent.com/komari-monitor/komari-agent/refs/heads/main/install.sh | sudo bash -s -- -e http://PANEL -t TOKEN --disable-web-ssh
```

应替换为：

```bash
wget -qO- https://raw.githubusercontent.com/xultral/komari-agent/refs/heads/main/install.sh | sudo bash -s -- -e http://PANEL -t TOKEN --disable-web-ssh
```

如果面板模板暂时不可改，至少确保 raw 脚本地址已经换成你的 fork；安装脚本本身会默认从 `xultral/komari-agent` release 下载。

## 可选覆盖项

安装脚本支持临时覆盖下载仓库：

```bash
--install-repo owner/repo
```

PowerShell:

```powershell
--install-repo owner/repo
```

这个参数适合灰度测试，不建议长期依赖。

# 部署指南

本文档提供 komari-agent (monitoring-only) 的生产环境部署建议。

**重要提示：** 本文档中的权限配置和安全加固步骤应在执行 `install.sh` 安装脚本**之后**进行。安装脚本会将 agent 安装到 `/opt/komari/agent`（Linux）或 `/usr/local/komari/agent`（macOS），并默认以 `root` 用户运行。

## 目录

- [权限配置](#权限配置)
- [systemd 服务](#systemd-服务)
- [Windows 服务](#windows-服务)
- [Docker 部署](#docker-部署)
- [日志管理](#日志管理)
- [网络配置](#网络配置)
- [监控验证](#监控验证)

---

## 权限配置

### 最小权限原则

虽然此版本已移除所有远程控制功能，但仍应遵循最小权限原则运行 agent。

**Agent 需要的权限：**
- ✅ 读取系统监控数据（`/proc`、`/sys`、性能计数器等）
- ✅ 网络访问（连接到 dashboard）
- ✅ 读写自己的配置和状态目录

**Agent 不需要的权限：**
- ❌ Root/Administrator 权限
- ❌ sudo 或权限提升
- ❌ 写入系统目录
- ❌ 访问其他用户的文件

### Linux/macOS 权限配置

**前置条件：** 已运行 `install.sh` 安装脚本，agent 已安装到 `/opt/komari/agent` (Linux) 或 `/usr/local/komari/agent` (macOS)。

#### 1. 创建专用用户

```bash
# 创建系统用户（无登录 shell）
# 使用 mmmmonitor 作为用户名（低调，不易被扫描识别，可以替换成自己喜欢的名字）
sudo useradd -r -s /bin/false -d /opt/komari mmmmonitor

# 设置目录权限
sudo chown -R mmmmonitor:mmmmonitor /opt/komari
```

#### 2. 修改已安装的 systemd 服务配置

安装脚本已创建 `/etc/systemd/system/komari-agent.service`，现在修改它使用专用用户：

```bash
# 停止服务
sudo systemctl stop komari-agent

# 编辑服务文件
sudo vim /etc/systemd/system/komari-agent.service
```

将 `User=root` 改为 `User=mmmmonitor`，并添加安全加固选项：

```ini
[Unit]
Description=Komari Agent Service
After=network.target

[Service]
Type=simple
User=mmmmonitor
Group=mmmmonitor
ExecStart=/opt/komari/agent -e https://your-dashboard.com -t YOUR_TOKEN
WorkingDirectory=/opt/komari
Restart=always

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/komari
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true

# 资源限制
LimitNOFILE=65536
MemoryLimit=256M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

重载并重启：

```bash
sudo systemctl daemon-reload
sudo systemctl start komari-agent
sudo systemctl status komari-agent
```

### Windows 权限配置

#### 1. 创建专用服务账户

```powershell
# 创建本地用户（使用 mmmmonitor 作为用户名）
$password = ConvertTo-SecureString "YOUR_STRONG_PASSWORD" -AsPlainText -Force
New-LocalUser -Name "mmmmonitor" -Password $password -Description "System Monitoring Service Account"

# 禁用交互式登录
Set-LocalUser -Name "mmmmonitor" -PasswordNeverExpires $true -UserMayNotChangePassword $true
```

#### 2. 设置文件权限

```powershell
# 二进制文件权限（只读+执行）
icacls "C:\Program Files\komari-agent\komari-agent.exe" /grant "mmmmonitor:(RX)"

# 配置文件权限（读取）
icacls "C:\ProgramData\komari-agent\config.json" /grant "mmmmonitor:(R)"

# 数据目录权限（读写）
icacls "C:\ProgramData\komari-agent\data" /grant "mmmmonitor:(OI)(CI)M"
```

---

## systemd 服务

### 说明

`install.sh` 脚本已自动创建并启动 systemd 服务，配置文件位于 `/etc/systemd/system/komari-agent.service`。

默认配置：
- 二进制路径：`/opt/komari/agent`
- 工作目录：`/opt/komari`
- 运行用户：`root`（需要手动改为专用用户，见上文权限配置）

### 安全加固的服务配置（推荐）

在执行 `install.sh` 之后，按照上文"权限配置"章节创建专用用户，然后修改 `/etc/systemd/system/komari-agent.service`：

```ini
[Unit]
Description=Komari Agent (Monitoring Only)
Documentation=https://github.com/xultral/komari-agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=mmmmonitor
Group=mmmmonitor

# 工作目录（install.sh 安装的实际路径）
WorkingDirectory=/opt/komari

# 启动命令（使用 install.sh 安装的实际路径和参数）
ExecStart=/opt/komari/agent \
  -e https://your-dashboard.com \
  -t YOUR_TOKEN \
  --interval 1.0

# 自动重启
Restart=always
RestartSec=10

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/komari-agent
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true

# 资源限制
LimitNOFILE=65536
MemoryLimit=256M
CPUQuota=50%

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=komari-agent

[Install]
WantedBy=multi-user.target
```

### 管理服务

```bash
# 查看状态（install.sh 已启动服务）
sudo systemctl status komari-agent

# 重启服务（修改配置后）
sudo systemctl restart komari-agent

# 停止服务
sudo systemctl stop komari-agent

# 查看日志
sudo journalctl -u komari-agent -f

# 重载配置（修改 service 文件后）
sudo systemctl daemon-reload
sudo systemctl restart komari-agent
```

---

## Windows 服务

### 使用 NSSM 安装服务

1. **下载 NSSM**
   - 访问 https://nssm.cc/download
   - 解压到 `C:\Tools\nssm`

2. **安装服务**

```powershell
# 使用 NSSM 安装服务
C:\Tools\nssm\nssm.exe install komari-agent "C:\Program Files\komari-agent\komari-agent.exe"

# 设置参数
C:\Tools\nssm\nssm.exe set komari-agent AppDirectory "C:\ProgramData\komari-agent"
C:\Tools\nssm\nssm.exe set komari-agent AppParameters `
  "--config C:\ProgramData\komari-agent\config.json --interval 1.0"

# 设置运行账户
C:\Tools\nssm\nssm.exe set komari-agent ObjectName ".\mmmmonitor" "YOUR_PASSWORD"

# 设置日志
C:\Tools\nssm\nssm.exe set komari-agent AppStdout "C:\ProgramData\komari-agent\logs\stdout.log"
C:\Tools\nssm\nssm.exe set komari-agent AppStderr "C:\ProgramData\komari-agent\logs\stderr.log"

# 设置自动重启
C:\Tools\nssm\nssm.exe set komari-agent AppRestartDelay 10000

# 启动服务
Start-Service komari-agent
```

### 使用 sc.exe（原生方式）

```powershell
# 创建服务
sc.exe create komari-agent `
  binPath= "C:\Program Files\komari-agent\komari-agent.exe --config C:\ProgramData\komari-agent\config.json" `
  start= auto `
  obj= ".\mmmmonitor" `
  password= "YOUR_PASSWORD"

# 设置描述
sc.exe description komari-agent "Komari Monitoring Agent"

# 启动服务
sc.exe start komari-agent
```

---

## Docker 部署

### Dockerfile 示例

```dockerfile
FROM alpine:3.19

# 安装必要的工具
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户（使用 mmmmonitor）
RUN addgroup -g 1000 mmmmonitor && \
    adduser -D -u 1000 -G mmmmonitor mmmmonitor

# 复制二进制文件
COPY komari-agent /usr/local/bin/komari-agent
RUN chmod 755 /usr/local/bin/komari-agent

# 创建工作目录
RUN mkdir -p /var/lib/komari-agent && \
    chown mmmmonitor:mmmmonitor /var/lib/komari-agent

# 切换到非 root 用户
USER mmmmonitor
WORKDIR /var/lib/komari-agent

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD pgrep komari-agent || exit 1

# 启动命令
ENTRYPOINT ["/usr/local/bin/komari-agent"]
```

### docker-compose.yml 示例

```yaml
version: '3.8'

services:
  komari-agent:
    image: komari-agent:latest
    container_name: komari-agent
    restart: unless-stopped
    
    # 资源限制
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
        reservations:
          memory: 64M
    
    # 网络模式（host 模式以便准确采集网络数据）
    network_mode: host
    
    # 环境变量配置
    environment:
      - AGENT_ENDPOINT=https://your-dashboard.example.com
      - AGENT_TOKEN=your_token_here
      - AGENT_INTERVAL=1.0
      - TZ=Asia/Shanghai
    
    # 挂载配置文件（可选）
    volumes:
      - ./config.json:/etc/komari-agent/config.json:ro
      - agent-data:/var/lib/komari-agent
    
    # 只读根文件系统（安全加固）
    read_only: true
    tmpfs:
      - /tmp
    
    # 安全选项
    security_opt:
      - no-new-privileges:true
    
    # 日志配置
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  agent-data:
```

### 运行 Docker 容器

```bash
# 构建镜像
docker build -t komari-agent:latest .

# 运行容器
docker run -d \
  --name komari-agent \
  --restart unless-stopped \
  --network host \
  --read-only \
  --tmpfs /tmp \
  --security-opt no-new-privileges:true \
  -e AGENT_ENDPOINT=https://your-dashboard.example.com \
  -e AGENT_TOKEN=your_token_here \
  -e AGENT_INTERVAL=1.0 \
  komari-agent:latest

# 查看日志
docker logs -f komari-agent

# 使用 docker-compose
docker-compose up -d
```

---

## 日志管理

### systemd 日志配置

```bash
# 查看实时日志
sudo journalctl -u komari-agent -f

# 查看最近 100 行
sudo journalctl -u komari-agent -n 100

# 搜索关键词
sudo journalctl -u komari-agent | grep "Ignoring"

# 查看今天的日志
sudo journalctl -u komari-agent --since today

# 查看特定时间段
sudo journalctl -u komari-agent --since "2026-06-25 08:00:00" --until "2026-06-25 10:00:00"
```

### 日志轮转（logrotate）

如果使用文件日志，创建 `/etc/logrotate.d/komari-agent`：

```
/var/log/komari-agent/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 komari-agent komari-agent
    sharedscripts
    postrotate
        systemctl reload komari-agent > /dev/null 2>&1 || true
    endscript
}
```

### 监控日志中的异常

建议监控以下日志模式：
- `Ignoring remote exec/terminal/ping` - 确认远程控制被正确拦截
- `Error`、`Failed` - 错误和失败
- `Max retries reached` - 连接问题
- `WebSocket disconnected` - 连接断开

---

## 网络配置

### 防火墙规则

**Agent 只需要出站连接，不需要入站端口：**

#### Linux (iptables)

```bash
# 允许到 dashboard 的出站 HTTPS 连接
sudo iptables -A OUTPUT -p tcp --dport 443 -d YOUR_DASHBOARD_IP -j ACCEPT

# 如果 dashboard 使用 HTTP
sudo iptables -A OUTPUT -p tcp --dport 80 -d YOUR_DASHBOARD_IP -j ACCEPT

# 允许 DNS 查询
sudo iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
```

#### Linux (firewalld)

```bash
# firewalld 默认允许出站连接，无需特殊配置
```

#### Windows 防火墙

```powershell
# 允许 komari-agent.exe 的出站连接
New-NetFirewallRule -DisplayName "Komari Agent Outbound" `
  -Direction Outbound `
  -Program "C:\Program Files\komari-agent\komari-agent.exe" `
  -Action Allow `
  -Profile Any
```

### 代理配置

如果需要通过代理连接：

```bash
# 设置环境变量
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
export NO_PROXY=localhost,127.0.0.1

# 或在 systemd 服务中配置
[Service]
Environment="HTTP_PROXY=http://proxy.example.com:8080"
Environment="HTTPS_PROXY=http://proxy.example.com:8080"
```

---

## 监控验证

### 验证 agent 正常运行

**1. 检查进程状态**

```bash
# Linux（注意：进程名是 agent，不是 komari-agent）
ps aux | grep "agent"
# 或者更精确
ps aux | grep "/opt/komari/agent"

# Windows
tasklist | findstr agent
```

**2. 检查网络连接**

```bash
# Linux（注意：进程名是 agent）
sudo netstat -tulpn | grep agent
# 或
sudo ss -tulpn | grep agent
# 或查看到 dashboard 的连接
sudo ss -tn | grep "YOUR_DASHBOARD_IP:443"

# Windows
netstat -ano | findstr "443"
```

**3. 验证监控数据上报**

在 dashboard 上检查：
- Agent 在线状态
- CPU、内存、磁盘等监控数据是否实时更新
- 最后更新时间戳

**4. 测试远程控制已禁用**

从 dashboard 尝试：
- 发送远程命令
- 打开 Web 终端
- 执行 Ping 任务

**预期行为：**
- Dashboard 显示"正在执行"但永远不会完成
- Agent 日志显示 `Ignoring ... in monitoring-only mode`
- 不会有任何实际操作执行

### 健康检查脚本

创建 `/usr/local/bin/komari-agent-health.sh`：

```bash
#!/bin/bash

# 检查进程是否运行（注意：进程名是 agent）
if ! pgrep -f "/opt/komari/agent" > /dev/null; then
    echo "ERROR: komari-agent process not running"
    exit 1
fi

# 检查日志中是否有连接错误
if journalctl -u komari-agent --since "5 minutes ago" | grep -q "Max retries reached"; then
    echo "WARNING: Connection issues detected"
    exit 2
fi

echo "OK: komari-agent is healthy"
exit 0
```

使其可执行：
```bash
sudo chmod +x /usr/local/bin/komari-agent-health.sh
```

---

## 故障排查

### 常见问题

**1. Agent 无法连接到 dashboard**

检查：
```bash
# DNS 解析
nslookup your-dashboard.example.com

# 网络连通性
curl -v https://your-dashboard.example.com

# 防火墙规则
sudo iptables -L -n
```

**2. 监控数据不准确**

- GPU 监控需要 `--gpu` 参数
- 网卡统计需要检查 `--include-nics` 和 `--exclude-nics`
- 磁盘统计需要检查 `--include-mountpoint`

**3. 权限不足**

```bash
# 检查 agent 运行用户
ps aux | grep komari-agent

# 检查文件权限
ls -la /var/lib/komari-agent
ls -la /etc/komari-agent
```

**4. 日志中出现 "Ignoring" 消息**

✅ **这是正常行为**！表示远程控制请求被正确拦截。

---

## 安全最佳实践

1. ✅ **使用专用低权限用户运行 agent**
2. ✅ **保护配置文件中的 token**（权限 640 或更严格）
3. ✅ **使用 HTTPS 连接 dashboard**（确保 `--endpoint` 使用 `https://`）
4. ✅ **定期更新到最新版本**（关注 GitHub Releases）
5. ✅ **监控日志中的异常活动**
6. ✅ **限制出站网络连接**（只允许到 dashboard 的连接）
7. ✅ **启用 systemd 安全特性**（NoNewPrivileges、ProtectSystem 等）
8. ✅ **设置资源限制**（防止资源耗尽）

---

## 参考资料

- [README.md](README.md) - 基本使用说明
- [SECURITY_REVIEW.md](SECURITY_REVIEW.md) - 完整安全审查报告
- [FORK_NOTES.md](FORK_NOTES.md) - 与官方版的差异说明
- [GitHub Releases](https://github.com/xultral/komari-agent/releases) - 版本发布

---

## 技术支持

如有问题，请：
1. 查看 agent 日志
2. 查阅本文档的故障排查章节
3. 在 GitHub Issues 提问：https://github.com/xultral/komari-agent/issues

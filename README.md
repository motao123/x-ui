# x-ui

`x-ui` 是一个基于 Go + Gin + Xray-core 的轻量级 Web 管理面板，面向 Linux 服务器部署，提供 Xray 入站配置、用户管理、流量统计、运行状态监控、HTTPS 面板访问、Telegram 通知与 v2-ui 数据迁移等能力。

本仓库基于原 x-ui 项目整理维护，并补充了安全加固，专注 Linux 服务器部署。

## 项目特性

- Web 可视化管理 Xray 入站配置
- 支持多用户、多协议、多传输方式
- 支持协议：`vmess`、`vless`、`trojan`、`shadowsocks`、`dokodemo-door`、`socks`、`http`
- 额外协议（需自定义 xray-core，集成 ）：`tuic`、`hysteria2`、`anytls`
- 支持 TCP、WebSocket、HTTP/2、gRPC、mKCP、QUIC、HTTPUpgrade、SplitHTTP 等传输配置
- 支持 TLS、XTLS、Reality 安全配置
- 系统状态监控：CPU、内存、磁盘、负载、网络流量、TCP/UDP 连接数
- 入站流量统计、到期时间限制、流量上限检查
- 支持面板 HTTPS、自定义访问路径 WebBasePath
- 支持 Telegram Bot 通知：流量统计、登录提醒、到期提醒、流量预警
- 支持从 v2-ui 迁移 inbound 账号数据
- 内置前端资源，无需额外前端构建步骤

## 安全加固说明

当前版本已针对常见面板风险做基础加固：

- 避免默认 `admin/admin` 弱口令长期存在，初始化时生成随机默认凭据
- 登录失败限速与临时锁定，降低暴力破解风险
- 会话 Cookie 增加 `HttpOnly`、`SameSite=Lax`，HTTPS 下自动启用 `Secure`
- 注销时清理会话 Cookie
- 对跨站写操作增加 Origin/Referer 同源校验
- 限制客户端 IP 获取的信任边界，避免直接信任伪造的代理头
- 对 WebBasePath、入站配置等关键输入增加基础校验
- Xray 运行配置文件使用更严格的文件权限写入

> 注意：该项目用于服务器代理面板管理，部署后请务必使用强密码、限制管理端口访问来源，并优先启用 HTTPS。

## 运行环境

### 服务端部署

推荐系统：

- Debian 8+
- Ubuntu 16+
- CentOS 7+

支持架构以发布包为准，常见为：`amd64`、`arm64`。

### 本地开发

- Go 1.20+（当前本地验证环境：Go 1.26.2）
- Git

## 快速开始

### 从源码运行

```bash
git clone https://github.com/motao123/x-ui.git
cd x-ui
go mod download
go run main.go
```

默认数据库路径为：

```text
/etc/x-ui/x-ui.db
```

如在本地开发环境运行，请确保当前用户有权限创建或访问该路径，或按需调整配置/运行环境。

### 构建

当前版本使用纯 Go SQLite 驱动（`github.com/glebarez/sqlite`），无需 CGO 即可编译：

```bash
go build -o x-ui main.go
```

对于服务器低内存环境（<512MB），可限制编译并行度：

```bash
GOMAXPROCS=1 go build -o x-ui main.go
```

运行：

```bash
./x-ui run
```

查看版本：

```bash
./x-ui -v
```

### Docker 构建

```bash
docker build -t x-ui .
docker run -itd --network=host \
  -v /etc/x-ui:/etc/x-ui \
  -v /root/cert:/root/cert \
  --name x-ui --restart=unless-stopped \
  x-ui
```

## 安装到 Linux 服务器

### 一键安装/更新

推荐使用当前仓库的一键安装命令：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/motao123/x-ui/main/install.sh)
```

也可以指定版本安装，例如：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/motao123/x-ui/main/install.sh) v1.0.0
```

> 说明：脚本会优先从 `https://github.com/motao123/x-ui/releases` 下载 `x-ui-linux-${arch}.tar.gz`。如果当前仓库还没有 Release 或下载失败，脚本会自动切换为源码构建安装。

### 从源码部署

适合当前仓库没有 Release 压缩包时使用：

```bash
apt update && apt install -y git curl tar golang
git clone https://github.com/motao123/x-ui.git /usr/local/x-ui-src
cd /usr/local/x-ui-src
go build -o x-ui main.go
chmod +x x-ui x-ui.sh bin/xray-linux-*
mkdir -p /usr/local/x-ui /etc/x-ui
cp -r x-ui x-ui.sh bin web config database logger util v2ui xray /usr/local/x-ui/
cp x-ui.service /etc/systemd/system/
ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
chmod +x /usr/bin/x-ui
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

安装后设置面板端口、用户名和密码：

```bash
x-ui setting -port 54321
x-ui setting -username your_user -password 'your_strong_password'
x-ui restart
```

### 手动安装 Release 包

手动安装流程示例：

```bash
cd /root/
tar zxvf x-ui-linux-amd64.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-*
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
mv x-ui/ /usr/local/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

常用管理命令：

```bash
x-ui              # 显示管理菜单
x-ui start        # 启动面板
x-ui stop         # 停止面板
x-ui restart      # 重启面板
x-ui status       # 查看状态
x-ui log          # 查看日志
x-ui enable       # 开机自启
x-ui disable      # 取消开机自启
x-ui update       # 更新面板
x-ui uninstall    # 卸载面板
```

## 命令行用法

直接运行：

```bash
x-ui run
```

修改面板端口、用户名、密码：

```bash
x-ui setting -port 54321
x-ui setting -username your_user -password 'your_strong_password'
```

查看当前面板配置：

```bash
x-ui setting -show
```

重置面板设置：

```bash
x-ui setting -reset
```

迁移 v2-ui 入站账号数据：

```bash
x-ui v2-ui
```

指定 v2-ui 数据库路径：

```bash
x-ui v2-ui -db /etc/v2-ui/v2-ui.db
```

## 协议使用指南

### 协议总览

| 协议 | 传输层安全 | 适用场景 | 推荐指数 |
|------|-----------|---------|---------|
| **VLESS + Reality** | Reality | 无需域名和证书，抗检测能力最强 | ★★★★★ |
| **VLESS + TLS** | TLS | 有域名和证书，搭配 WS/H2/gRPC 传输 | ★★★★☆ |
| **VMess + WS + TLS** | TLS | 有域名和证书，CDN 中转友好 | ★★★★☆ |
| **Trojan + TLS** | TLS | 有域名和证书，配置简单 | ★★★★☆ |
| **Shadowsocks** | 可选TLS | 简单轻量，兼容性好 | ★★★☆☆ |
| **VMess** (裸TCP) | 无 | 内网/测试用 | ★★☆☆☆ |
| **Dokodemo-door** | 无 | 任意门转发 | ★★☆☆☆ |
| **SOCKS / HTTP** | 无 | 内网代理转发 | ★☆☆☆☆ |

---

### 1. VLESS + Reality（推荐首选）

**最推荐的协议组合**，无需域名和 TLS 证书，通过模拟访问真实网站实现抗检测。

#### 面板配置步骤

1. **协议** 选择 `vless`
2. **端口** 填 `443`（增强伪装效果）
3. **id** 保持默认的 UUID 即可
4. **传输** 选择 `tcp`
5. **reality** 开关 打开
6. 开启 reality 后会展开 Reality 设置区域：
   - **dest**：填写 `www.amazon.com:443`（回落目标，选择支持 TLS 1.3 和 H2 的大站）
   - **serverNames**：输入 `www.amazon.com`（与 dest 保持一致，回车添加）
   - **privateKey / publicKey**：点击输入框右侧的 🔄 按钮一键生成密钥对
   - **shortIds**：留空或输入随机十六进制字符串（回车添加）
   - **fingerprint**：选择 `chrome`
7. **flow** 选择 `xtls-rprx-vision`（开启 reality 后 flow 选项会显示在 id 下方）

> **重要**：flow 必须设置 `xtls-rprx-vision`，否则 Reality 的安全优势无法体现

#### 推荐的 serverName/dest 目标网站

选择标准：支持 TLS 1.3 + H2 的国外大站

- `www.amazon.com`
- `www.ebay.com`
- `www.paypal.com`
- `www.cloudflare.com`
- `dash.cloudflare.com`
- `aws.amazon.com`
- `www.microsoft.com`
- `www.apple.com`

#### 注意事项

- dest 和 serverNames 必须指向同一个真实网站
- 无需申请 TLS 证书
- privateKey 是服务端私钥，publicKey 是客户端公钥（分享链接自动包含）

---

### 2. VLESS + TLS + WS（有域名推荐）

适合有域名和 TLS 证书的场景，WebSocket 传输支持 CDN 中转。

#### 面板配置

| 配置项 | 推荐值 | 说明 |
|--------|-------|------|
| 协议 | `vless` | |
| 端口 | 443 | |
| 传输 | `ws` | WebSocket |
| 安全 | `tls` | 开启 TLS |
| 域名 | `your.domain.com` | 填写你的域名 |
| 路径 | `/随机路径` | 建议 UUID 格式的路径，如 `/{uuid}` |
| 证书 | 文件路径或内容 | 可使用 ACME 一键申请 |

#### CDN 中转配置

1. 将域名解析到 CDN（如 Cloudflare），开启代理（橙色云朵）
2. Cloudflare → SSL/TLS → 设为 Full 或 Full (Strict)
3. 面板中 TLS 域名填写你的域名
4. 客户端连接地址使用域名，端口 443

---

### 3. VLESS + TLS + gRPC

适合有域名和证书的场景，gRPC 基于 HTTP/2，性能优秀。

#### 面板配置

| 配置项 | 推荐值 | 说明 |
|--------|-------|------|
| 协议 | `vless` | |
| 端口 | 443 | |
| 传输 | `grpc` | |
| 安全 | `tls` | |
| serviceName | 随机字符串 | gRPC 服务名，建议使用 UUID |
| 域名 | `your.domain.com` | |

---

### 4. VMess + WS + TLS（经典方案）

最经典稳定的方案，客户端兼容性最好。

#### 面板配置

| 配置项 | 推荐值 | 说明 |
|--------|-------|------|
| 协议 | `vmess` | |
| 端口 | 443 | |
| 传输 | `ws` | |
| 安全 | `tls` | |
| UUID | 自动生成 | |
| alterId | `0` | 必须为 0（AEAD 加密） |
| 加密 | `auto` | 客户端自动协商 |
| 域名 | `your.domain.com` | |
| 路径 | `/随机路径` | |

#### 注意事项

- alterId 必须为 0，否则使用不安全的旧加密方式
- 建议勾选「禁用不安全加密」

---

### 5. Trojan + TLS

配置简单，协议本身基于 TLS，伪装性好。

#### 面板配置

| 配置项 | 推荐值 | 说明 |
|--------|-------|------|
| 协议 | `trojan` | |
| 端口 | 443 | |
| 传输 | `tcp` | Trojan 默认 TCP |
| 安全 | `tls`（自动开启） | Trojan 协议强制 TLS |
| 密码 | 自动生成 | |
| 域名 | `your.domain.com` | |
| 证书 | 文件路径或内容 | **必须配置有效证书** |

#### 重要提醒

- **Trojan 必须配置有效的 TLS 证书**，否则 xray 无法启动
- 如无证书，可使用面板内置的「一键申请 SSL」功能（ACME）
- 也可通过 Caddy/Nginx 反向代理实现自动证书管理

---

### 6. Shadowsocks

经典轻量代理协议，兼容性最好。

#### 面板配置

| 配置项 | 推荐值 | 说明 |
|--------|-------|------|
| 协议 | `shadowsocks` | |
| 端口 | 随机 | |
| 加密方法 | `aes-256-gcm` | 推荐使用 AEAD 加密 |
| 密码 | 自动生成 | |
| 网络 | `tcp,udp` | 同时支持 TCP 和 UDP |

#### 加密方法对比

| 方法 | 安全性 | 性能 | 推荐度 |
|------|-------|------|--------|
| `aes-256-gcm` | 高 | 好 | ★★★★★ |
| `aes-128-gcm` | 高 | 更快 | ★★★★☆ |
| `chacha20-poly1305` | 高 | ARM 优 | ★★★★☆ |

> 注意：旧的 Stream Cipher 加密方式（aes-256-cfb、aes-128-cfb、chacha20、chacha20-ietf）已被弃用，面板中已禁用。

---

### 7. 其他协议

#### Dokodemo-door（任意门）

透明代理/端口转发用，将入站流量直接转发到指定目标。

| 配置项 | 说明 |
|--------|------|
| 目标地址 | 转发到的目标 IP 或域名 |
| 目标端口 | 转发到的目标端口 |
| 网络 | tcp, udp 或 tcp+udp |

#### SOCKS / HTTP

标准代理协议，适合内网代理转发场景。

| 协议 | 说明 |
|------|------|
| SOCKS | 支持 UDP 转发、用户名密码认证 |
| HTTP | HTTP 代理，支持用户名密码认证 |

#### MTProto

Telegram 专用代理协议，当前使用较少。

---

### 传输方式对比

| 传输方式 | CDN 支持 | 性能 | 特点 |
|---------|---------|------|------|
| `tcp` | 不支持 | 高 | 默认传输，最稳定 |
| `ws` (WebSocket) | 支持 | 中 | CDN 中转首选 |
| `http` (HTTP/2) | 部分支持 | 高 | 需 TLS，多路复用 |
| `grpc` | 不支持 | 高 | 基于 HTTP/2，性能好 |
| `kcp` (mKCP) | 不支持 | 丢包优 | UDP 传输，伪装可选 |
| `quic` | 不支持 | 丢包优 | UDP 传输，自带加密 |
| `httpupgrade` | 支持 | 高 | 类似 WS 但更高效 |
| `splithttp` | 支持 | 高 | 分块 HTTP，并发传输 |

---

### 安全层对比

| 安全层 | 需要证书 | 抗检测 | 适用协议 |
|--------|---------|--------|---------|
| `none` | 不需要 | 弱 | VMess(裸)、SS |
| `tls` | 需要 | 强 | VMess、VLESS、Trojan、SS |
| `xtls` | 需要 | 强 | VLESS、Trojan（仅 TCP） |
| `reality` | 不需要 | 最强 | VLESS、Trojan（TCP/WS/gRPC 等） |

---

### TLS 证书配置

TLS 证书是多数协议的必要条件，有以下获取方式：

#### 方式一：面板 ACME 一键申请

在 TLS 设置中点击「一键申请 SSL」，输入邮箱（可选），面板自动通过 ACME 申请 Let's Encrypt 免费证书。

前提条件：
- 域名已解析到服务器 IP
- 80 端口可访问（ACME HTTP 验证）

#### 方式二：手动申请

```bash
# 使用 acme.sh 申请
apt install socat -y
curl https://get.acme.sh | sh
~/.acme.sh/acme.sh --issue -d your.domain.com --standalone
```

申请后在面板中填写证书路径：
- 公钥：`/root/.acme.sh/your.domain.com/fullchain.cer`
- 密钥：`/root/.acme.sh/your.domain.com/your.domain.com.key`

#### 方式三：使用 Caddy 自动管理

安装 Caddy 作为前端反向代理，Caddy 自动申请和续期证书：

```
your.domain.com {
    reverse_proxy /path 127.0.0.1:内部端口
}
```

---

### 常见问题

#### xray 状态显示"未运行"

最常见原因：**入站配置了 TLS 但未填写证书路径**。

1. 检查日志：`x-ui log` 或 `journalctl -u x-ui -n 50`
2. 手动测试：`/usr/local/x-ui/bin/xray-linux-amd64 -c /usr/local/x-ui/bin/config.json`
3. 如提示 `failed to parse certificate`，检查所有 TLS 入站的证书路径是否有效

#### 连接超时

1. 确认 xray 正在运行：`x-ui status`
2. 检查端口是否放行：`ss -tlnp | grep 端口号`
3. 检查防火墙：`ufw status` 或 `iptables -L -n`

#### Reality 连接失败

1. 确认 dest 目标网站可正常访问
2. 确认 serverName 与 dest 一致
3. 确认 flow 设置为 `xtls-rprx-vision`
4. 确认 publicKey 在客户端正确配置

---

## Telegram Bot 通知

面板支持通过 Telegram Bot 做运行通知与流量提醒。可通过命令配置：

```bash
x-ui setting -tgbottoken '<bot_token>'
x-ui setting -tgbotchatid 123456789
x-ui setting -tgbotRuntime '@daily'
```

Cron 表达式示例：

```text
30 * * * * *   每分钟第 30 秒执行
@hourly         每小时执行
@daily          每天执行
@every 8h       每 8 小时执行
```

## HTTPS 与证书

面板支持配置证书文件和密钥文件后以 HTTPS 方式访问。生产环境建议：

1. 使用真实域名访问面板
2. 配置有效 TLS 证书
3. 修改默认端口和访问路径
4. 仅允许可信 IP 访问管理端口

## 从 v2-ui 迁移

在已安装 v2-ui 的服务器上安装并启动 x-ui 后执行：

```bash
x-ui v2-ui
```

该命令会迁移本机 v2-ui 的 inbound 账号数据；面板设置、用户名、密码不会迁移。

迁移完成后请停止 v2-ui 并重启 x-ui，避免 inbound 端口冲突。

## 开发与检查

格式化代码：

```bash
gofmt -w .
```

运行测试/编译检查：

```bash
go test ./...
```

本仓库当前已通过：

```text
go test ./...
```

## 项目结构

```text
config/          项目名称、版本、日志与数据库路径配置
database/        SQLite/GORM 数据库初始化与模型
logger/          日志封装
util/            通用工具、随机数、系统信息工具
v2ui/            v2-ui 数据迁移逻辑
web/             Web 服务、控制器、页面模板、静态资源、任务调度
xray/            Xray 配置生成、进程管理、流量统计
bin/             Xray 二进制与 geoip/geosite 数据
```

## 免责声明

本项目仅供学习、研究与合法合规的服务器管理场景使用。使用者应遵守所在地法律法规，并自行承担部署、配置及使用产生的风险。

## License

本项目遵循仓库内 `LICENSE` 文件声明的许可证。

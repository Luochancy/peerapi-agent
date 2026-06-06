# PeerAPI Agent 部署指南

## 架构概览

```
┌─────────────────────────────────────────────────┐
│                  用户 / 前端                      │
│            iedon-net-frontend (Vuetify)          │
└──────────────────────┬──────────────────────────┘
                       │ HTTP
┌──────────────────────▼──────────────────────────┐
│              IEDON-NET-API (Hono.js)             │
│         中心 API 服务器 · MySQL + Redis          │
│         端口 3000 (TCP 或 Unix Socket)           │
└──────────────────────┬──────────────────────────┘
                       │ HTTP + JWT
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
┌──────────────┐┌──────────────┐┌──────────────┐
│ peerapi-agent││ peerapi-agent││ peerapi-agent│
│   节点 A     ││   节点 B     ││   节点 C     │
│  (BIRD2+WG) ││  (BIRD2+WG) ││  (BIRD2+GRE)│
└──────────────┘└──────────────┘└──────────────┘
```

- **iedon-net-api**：中心 API，管理用户认证、peer 会话、邮件通知、WHOIS 查询
- **peerapi-agent**：部署在每个 DN42 节点，负责 BIRD 配置、隧道管理、指标采集、Looking Glass

---

## 一、IEDON-NET-API 部署

### 1.1 环境要求

| 依赖 | 版本 |
|------|------|
| Bun | 1.0+ |
| MySQL | 5.7+ 或 8.0+ |
| Redis | 6.0+ |

### 1.2 安装

```bash
git clone https://github.com/Luochancy/iedon-net-api.git
cd iedon-net-api

# 安装依赖
bun install
cd acorle-sdk && bun install && cd ..

# 复制配置文件
cp config.default.js config.js
```

### 1.3 配置

编辑 `config.js`，主要配置项：

```js
// 监听方式：TCP 或 Unix Socket
listen: {
  type: 'tcp',        // 'tcp' 或 'unix'
  hostname: 'localhost',
  port: 3000,
  path: '/var/run/peerapi.sock'  // unix socket 时使用
},

// 数据库
dbSettings: {
  dialect: 'mysql',   // 或 'sqlite'
  host: 'localhost',
  port: 3306,
  user: '',
  password: '',
  database: 'iedon-peerapi',
},

// Redis
redisSettings: {
  driver: {
    host: 'localhost',
    port: 6379,
    password: '',
    db: 0,
    keyPrefix: 'peerapi:',
  }
},

// Agent 认证密钥 — peerapi-agent 的 config.json 中 peerApiCenter.secret 必须与此一致
authHandler: {
  agentApiKey: 'YOUR_AGENT_API_TOKEN',
  stateSignSecret: 'YOUR_STATE_SIGN_SECRET',
},

// 探针服务器
probeServerSettings: {
  enabled: true,
  bindUdpPort: 2189,
  encryptionKey: 'YOUR_32_BYTE_PROBE_KEY',  // 必须与 agent 一致
},
```

> **提示**：完整的配置项说明见 `config.default.js` 中的注释。

### 1.4 运行

**开发模式**（热重载）：

```bash
bun run dev
```

**生产模式**：

```bash
bun run prod
```

### 1.5 Docker 部署

```bash
# 确保 config.js 已配置好
docker compose up -d
```

`docker-compose.yml` 包含：
- `peerapi`：API 服务（端口 3000）
- `redis`：Redis 实例

### 1.6 Systemd 部署

```bash
# 安装到 /opt/iedon/peerapi/
mkdir -p /opt/iedon/peerapi
cp -r . /opt/iedon/peerapi/

# 安装服务
cp peerapi.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now peerapi

# 查看日志
journalctl -u peerapi -f
```

服务文件中默认以 `www-data` 用户运行，确保文件权限正确。

---

## 二、PeerAPI Agent 部署

### 2.1 环境要求

| 依赖 | 说明 |
|------|------|
| Go 1.24+ | 编译用（二进制部署则不需要） |
| BIRD 2.0+ | BGP 路由守护进程 |
| Linux | 依赖 `/proc`、`ip` 命令 |
| iproute2 | 网络接口管理 |
| WireGuard tools | WireGuard 隧道管理（如使用 WG） |
| root 权限 | 网络接口操作需要 |
| MaxMind GeoLite2 | 可选，地理位置验证 |

### 2.2 安装

**方式一：下载二进制**

```bash
curl -L -o peerapi-agent https://github.com/Luochancy/peerapi-agent/releases/latest/download/peerapi-agent-linux-amd64
chmod +x peerapi-agent
```

**方式二：从源码编译**

```bash
git clone https://github.com/Luochancy/peerapi-agent.git
cd peerapi-agent/src

# 中国环境设置代理
go env -w GOPROXY=https://goproxy.cn,direct

go mod tidy
go build -o ../peerapi-agent .
```

### 2.3 目录结构

```
/opt/peerapi-agent/
├── peerapi-agent              # 二进制
├── config.json                # 配置文件
├── templates/
│   └── bird_peer.conf         # BIRD 配置模板
├── GeoLite2-Country.mmdb      # 可选，GeoIP 数据库
└── logs/
    └── peerapi-agent.log      # 日志
```

### 2.4 配置

`config.json` 完整示例见仓库中 `config.json.example`。关键配置项：

```jsonc
{
  "peerApiCenter": {
    "url": "https://peerapi.example.org",     // 中心 API 地址
    "secret": "YOUR_AGENT_API_TOKEN",          // 必须与 API 的 agentApiKey 一致
    "routerUuid": "自动生成的 UUID",
    "agentSecret": "本地 API 认证密钥",
    "heartbeatInterval": 30,                   // 心跳间隔（秒）
    "syncInterval": 300,                       // 会话同步间隔
    "metricInterval": 60,                      // 指标采集间隔
    "wanInterfaces": ["eth0"],                 // 监控的 WAN 接口
    "sessionPassthroughJwtSecret": "JWT密钥"
  },

  "bird": {
    "controlSocket": "/var/run/bird/bird.ctl",
    "poolSize": 5,
    "poolSizeMax": 128,
    "bgpPeerConfDir": "/etc/bird/peers",
    "bgpPeerConfTemplateFile": "./templates/bird_peer.conf",
    "ipCommandPath": "/usr/sbin/ip"
  },

  "wireguard": {
    "wgCommandPath": "/usr/bin/wg",
    "ipv4": "172.23.x.x",                     // 本节点 DN42 IPv4
    "ipv6": "fd42:xxxx::x",                    // 本节点 DN42 IPv6
    "ipv6LinkLocal": "fe80::xxx",
    "privateKeyPath": "/etc/wireguard/privatekey",
    "publicKeyPath": "/etc/wireguard/publickey",
    "localEndpointHost": "节点公网域名或IP"
  },

  "gre": {
    "ipv4": "172.23.x.x",
    "ipv6": "fd42:xxxx::x",
    "ipv6LinkLocal": "fe80::xxx",
    "localEndpointHost4": "节点公网 IPv4",
    "localEndpointHost6": "节点公网 IPv6"
  }
}
```

### 2.5 BIRD 配置

确保 BIRD 已安装并运行，且有一个基础配置文件（`/etc/bird/bird.conf`）。peerapi-agent 会在 `/etc/bird/peers/` 目录下自动生成每个 peer 的配置文件。

基础 `bird.conf` 需要包含：
- Router ID
- DN42 相关的 filter（`dn42_import_filter`、`dn42_export_filter`）
- 模板引用 `include "/etc/bird/peers/*.conf";`

### 2.6 运行

**直接运行：**

```bash
./peerapi-agent -c config.json
```

**Docker 部署：**

```bash
# 需要 host 网络模式和 privileged（操作 BIRD 和网络接口）
docker compose up -d
```

`docker-compose.yml` 包含：
- `peerapi-agent`：主机网络，privileged
- `searxng`：搜索引擎（可选，端口 8888）
- `redis`：searxng 依赖

**Systemd 部署：**

```bash
mkdir -p /data/peerapi-agent
cp peerapi-agent config.json /data/peerapi-agent/
cp -r templates /data/peerapi-agent/

cp peerapi-agent.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now peerapi-agent

# 查看日志
journalctl -u peerapi-agent -f
```

### 2.7 验证部署

1. **检查 agent 状态**：
   ```bash
   curl -H "Authorization: Bearer YOUR_AGENT_SECRET" http://localhost:8080/status
   ```

2. **检查 BIRD 会话**：
   ```bash
   birdc show protocols
   ```

3. **手动触发同步**：
   ```bash
   curl -H "Authorization: Bearer YOUR_AGENT_SECRET" http://localhost:8080/sync
   ```

4. **Looking Glass**：
   ```bash
   # 查看协议列表（无需认证）
   curl http://localhost:8080/lg/protocols
   
   # 查看路由详情（需要认证）
   curl -H "Authorization: Bearer YOUR_AGENT_SECRET" http://localhost:8080/lg/routes
   ```

---

## 三、安全注意事项

1. **密钥管理**：`agentApiKey`、`stateSignSecret`、`encryptionKey` 等密钥必须使用强随机字符串，且 agent 与 API 保持一致
2. **网络隔离**：API 的 Unix Socket 模式适合反向代理场景；agent 使用 host 网络是必要的（操作 BIRD）
3. **认证**：LG 端点的公开/认证分离由 API 层控制，agent 侧已移除 LG 认证
4. **GeoIP**：GeoLite2 数据库需定期更新

---

## 四、常见问题

| 问题 | 排查 |
|------|------|
| Agent 连不上 API | 检查 `peerApiCenter.url` 和 `secret` 是否匹配 |
| BIRD 会话不建立 | 检查 `/etc/bird/peers/` 下配置文件是否生成，`birdc show protocols` 查看状态 |
| 接口创建失败 | 确认 root 权限、`ip` 和 `wg` 命令路径正确 |
| 指标不更新 | 检查 `metricInterval`，查看 agent 日志 |
| Docker 内 BIRD 不启动 | 确认 `entrypoint.sh` 有执行权限，BIRD socket 目录已挂载 |

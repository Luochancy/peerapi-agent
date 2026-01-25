# peerapi-agent 配置文件解析

本文基于仓库根目录的 [config.json](config.json) 进行逐项说明。字段结构与代码定义一致，详见 [src/config.go](src/config.go)。

## 顶层结构
- `server`: HTTP 服务与监听配置
- `ipConfig`: 本机用于接口地址分配的 IP 池（用于接口本端地址）
- `logger`: 日志输出与滚动
- `peerApiCenter`: PeerAPI 中心服务与会话同步配置
- `bird`: BIRD 控制与模板生成配置
- `sysctl`: 绑定接口的 sysctl 参数
- `metric`: RTT/性能采集与 GeoIP 规则
- `wireguard`: WireGuard 命令与默认参数
- `gre`: GRE/IPv6 GRE 本地端点
- `peerProbe`: 会话探测任务

---

## server
- `debug`: 是否开启调试日志（访问日志更详细）。
- `listenerType`: 监听类型，`tcp` 或 `unix`。
- `listen`: 监听地址；`tcp` 时为 `host:port`，`unix` 时为 socket 路径。
- `readTimeout`: 读取超时（秒）。
- `writeTimeout`: 写入超时（秒）。
- `idleTimeout`: 空闲连接超时（秒）。
- `writeBufferSize`: TCP 写缓冲大小（字节）。
- `readBufferSize`: TCP 读缓冲大小（字节）。
- `bodyLimit`: 请求体最大字节数。
- `trustedProxies`: 信任的代理 IP/CIDR（用于解析 X-Forwarded-For）。

## ipConfig
- `ipv4`: WireGuard/GRE 接口分配的本端 IPv4 地址。
- `ipv6`: WireGuard/GRE 接口分配的本端 IPv6 地址。
- `ipv6LinkLocal`: WireGuard/GRE 接口分配的本端 IPv6 Link-Local 地址。

## logger
- `file`: 日志文件路径。
- `maxSize`: 单个日志最大 MB。
- `maxBackups`: 保留的历史日志数量。
- `maxAge`: 历史日志保留天数。
- `compress`: 是否压缩历史日志。
- `consoleLogging`: 是否同时输出到控制台。

## peerApiCenter
- `apiUrl`: PeerAPI 中心服务地址。
- `probeServerIPv4`: 探测服务 IPv4。
- `probeServerIPv6`: 探测服务 IPv6。
- `probeServerIPv6Prefix`: 探测服务 IPv6 前缀（用于路由注入）。
- `probeServerPort`: 探测服务 UDP 端口。
- `secret`: 与中心服务通信的共享密钥。
- `requestTimeout`: 请求超时（秒）。
- `routerUuid`: 路由器 UUID 标识。
- `agentSecret`: agent API 认证密钥。
- `heartbeatInterval`: 心跳上报间隔（秒）。
- `syncInterval`: 会话同步间隔（秒）。
- `metricInterval`: 采样上报间隔（秒，最低 60）。
- `wanInterfaces`: WAN 口列表（用于流量统计）。
- `sessionPassthroughJwtSecert`: 会话透传 JWT 密钥。
- `interfaceIpAllowPublic`: 是否允许接口分配公网 IP。
- `interfaceIpBlacklist`: 接口地址黑名单（CIDR）。

## bird
- `controlSocket`: BIRD 控制 socket 路径。
- `poolSize`: 连接池初始大小。
- `poolSizeMax`: 连接池最大大小。
- `connectionMaxRetries`: 连接重试次数。
- `connectionRetryDelayMs`: 重试延迟（毫秒）。
- `bgpPeerConfDir`: 生成的 peer 配置目录。
- `bgpPeerConfTemplateFile`: peer 模板文件路径。
- `ipCommandPath`: `ip` 命令路径。

## sysctl
- `commandPath`: `sysctl` 命令路径。
- `ifaceIpForwarding`: IPv4 转发。
- `ifaceIp6Forwarding`: IPv6 转发。
- `ifaceIp6AcceptRa`: IPv6 接受 RA。
- `ifaceIp6AutoConfig`: IPv6 自动配置。
- `ifaceRpFilter`: 反向路径过滤（0/1/2）。
- `ifaceAcceptLocal`: 允许本地地址（用于 Anycast 等）。

## metric
- `autoTeardown`: 触发规则后自动 teardown。
- `maxMindGeoLiteCountryMmdbPath`: GeoIP 数据库路径。
- `geoIpCountryMode`: `blacklist` 或 `whitelist`。
- `blacklistGeoCountries`: 黑名单国家。
- `whitelistGeoCountries`: 白名单国家。
- `pingCommandPath`: `ping` 命令路径。
- `pingTimeout`: ping 超时（秒）。
- `pingCount`: ping 次数。
- `pingCountOnFail`: 失败时的 ping 次数。
- `pingWorkerCount`: RTT 并发 worker 数。
- `sessionWorkerCount`: 会话 metrics 并发 worker 数。
- `maxRTTMetricsHistroy`: RTT 历史数量上限。
- `geoCheckInterval`: GeoIP 检查间隔（秒）。
- `filterParamsUpdateInterval`: BIRD community/过滤参数更新间隔（秒）。

## wireguard
- `wgCommandPath`: `wg` 命令路径。
- `privateKeyPath`: 本机私钥路径。
- `publicKeyPath`: 本机公钥路径。
- `persistentKeepaliveInterval`: keepalive 间隔（秒）。
- `allowedIps`: 默认 AllowedIPs 列表。
- `dnsUpdateInterval`: DNS 端点刷新间隔（秒）。
- `localEndpointHost`: 本机对外 Endpoint Host（用于返回给对端）。
- `dn42BandwidthCommunity`: DN42 带宽社区值。
- `dn42InterfaceSecurityCommunity`: DN42 安全社区值。

## gre
- `localEndpointHost4`: GRE IPv4 本地端点 IP。
- `localEndpointHost6`: GRE IPv6 本地端点 IP。
- `localEndpointDesc4`: 返回给对端的 IPv4 端点描述。
- `localEndpointDesc6`: 返回给对端的 IPv6 端点描述。
- `dn42BandwidthCommunity`: DN42 带宽社区值。
- `dn42InterfaceSecurityCommunity`: DN42 安全社区值。

## peerProbe
- `enabled`: 是否开启探测任务。
- `intervalSeconds`: 探测间隔（秒）。
- `probePacketCount`: 每次探测包数。
- `probePacketIntervalMs`: 探测包间隔（毫秒）。
- `probePacketEncryptionKey`: 探测包加密 key。
- `sessionWorkerCount`: 并发 worker 数。
- `probePacketBanner`: 探测包 banner。
- `probeSummaryCooldownSeconds`: 探测结果冷却时间（秒）。

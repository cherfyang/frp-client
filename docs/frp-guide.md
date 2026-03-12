# frp 客户端说明

## 1. frp 是什么

frp 是一个内网穿透和反向代理工具，采用 `frps` 服务端 + `frpc` 客户端的 C/S 架构。

- `frps` 通常部署在有公网 IP 的服务器上
- `frpc` 部署在内网机器、家庭网络、NAT 或防火墙后面
- `frpc.toml` 是客户端配置文件，决定客户端如何连接服务端、暴露哪些本地服务、以及如何访问私有代理

这个项目只聚焦 `frpc.toml`，也就是客户端配置。

## 2. frpc.toml 的结构

客户端配置大致分成两类：

1. 根级公共配置
2. 数组表段落

根级公共配置负责“客户端怎么连服务器”；数组表负责“客户端要暴露什么服务，或者访问什么私有服务”。

最常见结构如下：

```toml
serverAddr = "x.x.x.x"
serverPort = 7000

[auth]
token = "your-token"

[transport]
protocol = "tcp"

[[proxies]]
name = "ssh"
type = "tcp"
localIP = "127.0.0.1"
localPort = 22
remotePort = 6000

[[visitors]]
name = "ssh_visitor"
type = "stcp"
serverName = "secret_ssh"
secretKey = "shared-key"
bindAddr = "127.0.0.1"
bindPort = 6000
```

## 3. 根级公共配置说明

### `serverAddr`

`frpc` 要连接的 `frps` 地址，通常是公网 IP 或域名。

### `serverPort`

`frpc` 连接 `frps` 的端口，默认常见值是 `7000`。

### `[auth]`

客户端认证配置。

- `method`: 认证方式，常见是 `token`
- `token`: 与 `frps` 保持一致的共享认证令牌
- `oidc.*`: 如果走 OIDC，可以配置 OIDC 客户端参数

最常用的是：

```toml
[auth]
token = "your-token"
```

### `[transport]`

客户端与服务端之间的传输层配置。

常见字段：

- `protocol`: 与 `frps` 通信所用协议，常见有 `tcp`、`kcp`、`quic`、`websocket`、`wss`
- `proxyURL`: 如果客户端访问公网必须经过 HTTP/SOCKS5 代理，可在这里配置
- `poolCount`: 连接池数量
- `tcpMux`: 是否启用 TCP 复用
- `tls.*`: 与服务端通信时的 TLS 配置

常见用途：

- 默认场景一般保持 `tcp`
- 需要走代理上网时配置 `transport.proxyURL`
- 需要更强安全性时关注 `transport.tls.*`

### `[log]`

客户端日志配置，用于控制日志级别、输出位置和保留天数。

### `[webServer]`

客户端本地管理接口配置。启用后可用于：

- `frpc reload -c ./frpc.toml`
- `frpc status -c ./frpc.toml`

典型配置：

```toml
webServer.addr = "127.0.0.1"
webServer.port = 7400
```

### `start`

只启动指定代理名。适合一个配置里写了很多代理，但临时只想启用其中一部分。

### `includes`

允许从额外目录读取 `proxy` 和 `visitor` 配置，便于拆分大配置文件。

## 4. `[[proxies]]` 是什么

`[[proxies]]` 表示“把当前机器上的某个本地服务通过 frp 暴露出去”。

每一个 `[[proxies]]` 段落就是一个代理实例。

通用字段：

- `name`: 代理名，必须唯一
- `type`: 代理类型
- `localIP`: 本地服务 IP，默认常见是 `127.0.0.1`
- `localPort`: 本地服务端口
- `transport.*`: 单个代理自己的传输层配置
- `loadBalancer.*`: 负载均衡配置
- `healthCheck.*`: 健康检查配置
- `plugin.*`: 客户端插件配置

### 4.1 TCP

最常见，适合 SSH、MySQL、RDP、Redis 等普通 TCP 服务。

关键字段：

- `remotePort`: 在 `frps` 上对外暴露的端口

示例：

```toml
[[proxies]]
name = "ssh"
type = "tcp"
localIP = "127.0.0.1"
localPort = 22
remotePort = 6000
```

### 4.2 UDP

适合 DNS、游戏服务、语音服务等 UDP 场景。

关键字段和 TCP 类似，也是通过 `remotePort` 暴露端口。

### 4.3 HTTP

适合把本地网站通过域名暴露出去。

常见字段：

- `customDomains`: 绑定的域名列表
- `subdomain`: 子域名前缀
- `locations`: 按路径转发
- `hostHeaderRewrite`: 改写 Host 头
- `httpUser` / `httpPassword`: Basic Auth

适合场景：

- 网站后台
- 管理面板
- API 服务

### 4.4 HTTPS

和 HTTP 类似，但用于 HTTPS 站点。

常见字段：

- `customDomains`
- `subdomain`
- `hostHeaderRewrite`

### 4.5 TCPMUX

通过同一监听端口复用多个服务。

关键字段：

- `multiplexer`: 常见值是 `httpconnect`
- `customDomains` / `subdomain`

适合多服务共用一个入口端口。

### 4.6 STCP

私有 TCP 代理，不直接向公网暴露，需要访问方再通过 `visitor` 连接。

关键字段：

- `secretKey`: 服务端和访问端必须一致
- `allowUsers`: 允许哪些 visitor 用户访问

适合：

- 私有 SSH
- 私有桌面服务
- 不希望公开暴露到公网的内部端口

### 4.7 XTCP

点对点打洞模式，适合内网互访。

关键字段：

- `secretKey`
- `allowUsers`
- 与 NAT 打洞相关的 `natTraversal.*`

### 4.8 SUDP

私有 UDP 代理，思路类似 STCP，只是面向 UDP 服务。

## 5. `[[visitors]]` 是什么

`[[visitors]]` 表示“访问另一个客户端暴露出来的私有代理”。

它不是把本机服务暴露出去，而是在本机监听一个本地地址，然后把访问流量转发到远端私有代理。

通用字段：

- `name`: visitor 名称
- `type`: `stcp`、`xtcp`、`sudp`
- `serverName`: 要访问的远端代理名
- `secretKey`: 与远端私有代理保持一致
- `bindAddr`: 本地监听地址
- `bindPort`: 本地监听端口
- `serverUser`: 指定远端代理所属用户
- `transport.*`: visitor 传输层配置

### 5.1 STCP Visitor

用于访问远端 `stcp` 代理。

本地访问方式通常是：

1. 在本地启动 visitor
2. 访问 `bindAddr:bindPort`
3. frp 再把流量转发到远端私有服务

### 5.2 XTCP Visitor

用于访问远端 `xtcp` 代理。

额外字段：

- `protocol`: 打洞隧道协议，常见 `quic` 或 `kcp`
- `keepTunnelOpen`: 保持隧道常驻
- `maxRetriesAnHour`: 每小时最多重试次数
- `minRetryInterval`: 最小重试间隔
- `fallbackTo`: 回退到另一个 visitor
- `fallbackTimeoutMs`: 建连超时后触发回退

### 5.3 SUDP Visitor

用于访问远端 `sudp` 代理，适合私有 UDP 服务访问。

## 6. `transport.*` / `loadBalancer.*` / `healthCheck.*`

这些是代理和访问器的常见附加段落。

### `transport.*`

单个代理自己的传输层配置。

常见字段：

- `transport.useEncryption`: 启用该代理的数据加密
- `transport.useCompression`: 启用压缩
- `transport.bandwidthLimit`: 单代理限速
- `transport.bandwidthLimitMode`: `client` 或 `server`
- `transport.proxyProtocolVersion`: `v1` 或 `v2`
- `transport.poolCount`: 连接池大小

### `loadBalancer.*`

用于多个代理组成一个组做轮询负载均衡。

常见字段：

- `loadBalancer.group`
- `loadBalancer.groupKey`

常用于多个 HTTP/TCP 节点共同对外服务。

### `healthCheck.*`

健康检查，避免把流量打到不可用后端。

常见字段：

- `healthCheck.type`: `tcp` 或 `http`
- `healthCheck.timeoutSeconds`
- `healthCheck.maxFailed`
- `healthCheck.intervalSeconds`
- `healthCheck.path`: 当类型为 `http` 时使用

## 7. `plugin.*` 是什么

客户端插件用于“即使本地没有显式监听的 TCP/UDP 服务，也能由 frpc 内建一些能力”。

启用插件后，很多场景下不再需要配置 `localIP` 和 `localPort`。

常见插件：

### `plugin.type = "http_proxy"`

让 frpc 自身提供 HTTP 代理能力。

常见字段：

- `plugin.httpUser`
- `plugin.httpPassword`

### `plugin.type = "socks5"`

让 frpc 自身提供 SOCKS5 代理能力。

常见字段：

- `plugin.username`
- `plugin.password`

### `plugin.type = "static_file"`

把本地目录直接作为静态文件服务暴露出去。

常见字段：

- `plugin.localPath`
- `plugin.stripPrefix`
- `plugin.httpUser`
- `plugin.httpPassword`

### `plugin.type = "unix_domain_socket"`

把 UNIX Socket 服务通过 frp 暴露。

常见字段：

- `plugin.unixPath`

### `plugin.type = "http2https"`

前端 HTTP，请求转发到本地 HTTPS 服务。

常见字段：

- `plugin.localAddr`
- `plugin.hostHeaderRewrite`

### `plugin.type = "https2http"`

前端 HTTPS，请求转发到本地 HTTP 服务。

常见字段：

- `plugin.localAddr`
- `plugin.hostHeaderRewrite`
- `plugin.enableHTTP2`
- `plugin.crtPath`
- `plugin.keyPath`

### `plugin.type = "https2https"`

前端 HTTPS，请求转发到本地 HTTPS 服务。

### `plugin.type = "tls2raw"`

把 TLS 请求转发到普通本地服务。

### `plugin.type = "virtual_net"`

用于虚拟网络能力。

## 8. 常见配置思路

### 把本机 SSH 暴露到公网

用 `tcp` 代理：

- `localPort = 22`
- `remotePort = 6000`

### 把本机网站暴露到域名

用 `http` 或 `https` 代理：

- `customDomains`
- `subdomain`
- `locations`

### 私有访问另一台设备的 SSH

服务端机器用 `stcp` 或 `xtcp` 代理，访问端机器用 `visitor`。

### 做多节点后端

多个代理配置相同的 `loadBalancer.group`，再加 `healthCheck.*`。

## 9. 在本项目里怎么理解

本项目的“添加段落”页面，本质上是在帮你生成这些客户端配置段落：

- 根配置仍然建议直接在“原文件”里改
- 暴露服务时用 `[[proxies]]`
- 访问私有代理时用 `[[visitors]]`
- 需要额外功能时填 `transport.*`、`healthCheck.*`、`loadBalancer.*`、`plugin.*`
- 模板里没有覆盖的字段，可以用“额外属性”补

## 10. 官方来源

- frp 概览: https://gofrp.org/en/docs/overview/
- 客户端配置: https://gofrp.org/en/docs/reference/client-configures/
- 代理配置: https://gofrp.org/en/docs/reference/proxy/
- 访问器配置: https://gofrp.org/en/docs/reference/visitor/
- 客户端插件配置: https://gofrp.org/en/docs/reference/client-plugin/
- 访问器插件配置: https://gofrp.org/en/docs/reference/visitor-plugin/
- 官方仓库: https://github.com/fatedier/frp

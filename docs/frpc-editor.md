# frpc.toml 可视化编辑页

## Goal

- 提供一个本地网页，默认通过本地开发服务直接读取当前目录下的 `frpc.toml`，也支持用户上传其他 `frpc.toml`。
- 解析并展示根配置和 `[[proxies]]`、`[[visitors]]` 等数组表段落。
- 支持查看原始 TOML 文本。
- 支持通过官方模板表单追加新的 `[[proxies]]`、`[[visitors]]` 或其他数组表段落，并写回原始文件。
- 提供一个 `frp说明` 模块，前端直接读取项目内 Markdown 文档，展示 frp 客户端配置说明、段落用法和参数作用。
- 顶部栏最右侧提供 `重启frp服务` 按钮，通过本地开发服务端点执行 `frpc` 重启。
- 当本机未找到 `frpc` 时，支持一键安装官方 `frpc` 二进制到项目本地目录，并在安装完成后继续执行重启。

## Non-goals

- 不做服务端接口，不依赖后端存储。
- 不保证保留 TOML 注释格式化风格的逐字符一致性，但优先保留原始文本并在尾部追加新段落。
- 不覆盖 FRP 全部高级语义校验，只做基础字段类型校验和可读性提示。

## User Flow

1. 页面启动时默认通过本地开发服务读取仓库根目录 `frpc.toml`。
2. 用户也可以在“上传文件”模块中选择自己的 `frpc.toml`。
3. 浏览器优先使用 File System Access API 打开本地文件；不支持时回退到普通文件上传。
4. 页面读取原始文本，解析出：
   - 根配置键值
   - `[[section]]` 数组表（重点是 `proxies`、`visitors`）
5. 页面拆成 4 个顶部模块：
   - frp说明
   - 上传文件
   - 查看段落
   - 原文件
   - 添加段落
6. 顶部右侧按钮可重启当前目录 `frpc.toml` 对应的 `frpc` 进程。
7. 若重启时发现未安装 `frpc`，前端弹出确认框：`未找到 frpc 可执行文件,是否现在安装`。
8. 用户点击“是”后，页面调用本地开发服务端点执行项目根目录 `setup-frpc.sh`。
9. 安装完成后，页面自动再次发起重启。
10. 用户在“添加段落”里选择段落分组和模板，填写字段后追加到原始文本末尾。
11. “原文件”模块只保留“保存”按钮，点击后先保存，再自动重新解析当前文本。
12. 默认文件来源为仓库根目录时，保存直接写回当前目录 `frpc.toml`。
13. 若用户上传的是其他本地文件且浏览器提供文件句柄，则优先写回用户选择的原文件。

## Data Handling

- 使用前端 TOML 解析库读取现有配置，仅用于展示和校验。
- 保存时以“原始文本 + 新段落文本”的方式追加，尽量减少对已有内容的改动。
- 新段落表单提供基于官方文档整理的模板：
  - 代理模板：`tcp`、`udp`、`http`、`https`、`tcpmux`、`stcp`、`xtcp`、`sudp`
  - 访问器模板：`stcp visitor`、`xtcp visitor`、`sudp visitor`
  - 插件字段：`http_proxy`、`socks5`、`static_file`、`unix_domain_socket`、`http2https`、`https2http`、`https2https`、`tls2raw`、`virtual_net`
  - 允许补充自定义字段，覆盖未内置的属性
- 通用字段按官方配置拆成几组：
  - 基础代理字段
  - `transport.*`
  - `loadBalancer.*`
  - `healthCheck.*`
  - `plugin.*`
- `frp说明` 页面直接读取 `docs/frp-guide.md`，不通过后端接口。
- 默认配置文件读写走本地开发服务：
  - `GET /api/frp/config` 读取仓库根目录 `frpc.toml`
  - `POST /api/frp/config/save` 写回仓库根目录 `frpc.toml`
- `重启frp服务` 按钮调用本地开发服务的 `POST /api/frp/restart`。
- 若缺少二进制，前端改为调用本地开发服务的 `POST /api/frp/install`。
- 本地控制逻辑默认：
  - 使用仓库根目录 `frpc.toml`
  - 进程 PID 写入 `.frpc.pid`
  - 输出日志写入 `.frpc.log`
  - `frpc` 可执行文件查找顺序：`FRPC_BIN` -> 项目本地 `.tools/frp/bin/frpc` -> PATH
- 一键安装脚本：
  - 入口脚本是仓库根目录 `./setup-frpc.sh`
  - 优先从官方 GitHub release 拉取当前系统匹配的 `frp` 压缩包
  - 安装到项目本地 `.tools/frp/bin/frpc`
  - 支持通过 `FRPC_VERSION` 指定版本；未指定时自动拉取 latest release

## Validation

- 必填字段：段落名、`type`
- 数字字段：端口、超时时间等
- 布尔字段：`transport.useEncryption`、`transport.useCompression`、`keepTunnelOpen` 等
- 数组字段：使用逗号分隔输入，保存为 TOML 数组
- 对重复 `name` 给出提示，但不强制阻止

## Verification

### Automated

- 执行前端构建，确保 TypeScript 和打包通过。

### Manual

1. 运行根目录 `./run.sh`，启动本地开发服务器（监听 `0.0.0.0:6633`）。
2. 默认进入页面时确认仓库根目录 `frpc.toml` 已被加载。
3. 在“查看段落”中确认页面能列出已有 `proxies`、`visitors`。
4. 在“添加段落”中选择一个官方模板并追加。
5. 在“原文件”中点击保存，并确认页面自动重新解析且新段落已经写入。
6. 打开“frp说明”，确认页面能展示项目内 Markdown 总结文档。
7. 点击顶部 `重启frp服务`，确认开发服务返回成功；若本机未安装 `frpc`，应返回清晰错误信息。
8. 在未安装 `frpc` 的情况下点击顶部 `重启frp服务`，确认页面出现安装确认框。
9. 点击确认后，验证 `./setup-frpc.sh` 被执行，且 `.tools/frp/bin/frpc` 被成功安装。
10. 安装完成后再次点击 `重启frp服务`，确认页面优先使用项目本地 `frpc`。
11. 在未上传其他文件的情况下，修改“原文件”内容后点击保存，确认仓库根目录 `frpc.toml` 被更新。

## Rollout Notes

- 这是纯前端工具，无需数据库和后端配置。
- File System Access API 在 Chromium 系浏览器体验最佳；其他浏览器使用上传 + 下载回退方案。
- 一键启动脚本固定使用 `6633` 端口，并监听 `0.0.0.0` 方便局域网访问。
- 安装脚本依赖本机可访问 GitHub Release，且需要系统自带 `curl`、`tar` 和 `node`。

## Sources

- 官方 README: <https://github.com/fatedier/frp?tab=readme-ov-file>
- 客户端配置参考: <https://gofrp.org/en/docs/reference/client-configures/>
- 代理配置参考: <https://gofrp.org/en/docs/reference/proxy/>
- 访问器配置参考: <https://gofrp.org/en/docs/reference/visitor/>

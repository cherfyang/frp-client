# frpc.toml 配置中心

## Goal

- 提供一个可经由 frp 暴露访问的独立网页服务，统一管理 `frpc.toml`。
- 前端只负责界面和编辑，配置读写、配置列表存储、`frpc` 安装和重启都放到后端。
- 支持保存 20 份 `frpc` 配置快照，供后续反复切换和编辑。

## Runtime

- 前端使用 Vite 构建静态资源到 `dist/`。
- 后端使用 Node 原生 HTTP 服务静态资源和 API，监听 `0.0.0.0:6633`。
- `run.sh` 会先构建前端，再启动后端，不再依赖 Vite 开发服务提供控制接口。

## Top Navigation

- `frp说明`
- `配置列表`
- `查看段落`
- `配置文件`
- `添加段落`
- 顶部右侧固定按钮：`重启frp服务`

## Config Sources

编辑器支持 3 类来源：

1. 当前目录 `frpc.toml`
2. 浏览器上传的本地 `frpc.toml`
3. `frpc-config/` 下保存的槽位配置

保存规则：

- `当前目录 / frpc.toml`：点击保存直接写回仓库根目录 `frpc.toml`
- `上传文件`：浏览器支持文件句柄时写回原路径，否则下载
- `frpc-config/frpc-n.toml`：点击保存写回对应槽位文件

## Config List

- 目录固定为 `frpc-config/`
- 槽位固定为 `frpc-1.toml` 到 `frpc-20.toml`
- 元数据映射保存在 `frpc-config/manifest.json`
- 每条配置元数据包含：
  - `slot`
  - `name`
  - `description`
  - `updatedAt`

前端展示格式：

- 第一行：`名称` 加粗 + `-- frpc-n.toml`
- 第二行：描述
- 第三行：更新时间

## Editor Actions

`配置文件` 模块内提供：

- `上传文件`
- `读取当前目录`
- `保存到配置列表`
- `保存`

`保存到配置列表` 流程：

1. 弹出命名和描述输入框
2. 命名必填，描述可空
3. 后端寻找第一个空槽位
4. 保存内容到 `frpc-config/frpc-n.toml`
5. 写入 `manifest.json`
6. 刷新 `配置列表`

## Backend API

### Config API

- `GET /api/config/current`
  - 读取当前目录 `frpc.toml`
- `POST /api/config/current/save`
  - 保存当前目录 `frpc.toml`
- `GET /api/config/list`
  - 返回配置列表和容量
- `GET /api/config/list/:slot`
  - 读取指定槽位配置
- `POST /api/config/list/:slot/save`
  - 保存指定槽位配置
- `POST /api/config/list/save`
  - 以“命名 + 描述 + 内容”创建新的槽位配置

### FRP API

- `POST /api/frp/install`
  - 执行 `setup-frpc.sh`
- `POST /api/frp/restart`
  - 重启当前目录 `frpc.toml` 对应的 `frpc`

## FRP Binary

- 安装脚本：`./setup-frpc.sh`
- 默认安装路径：`.tools/frp/bin/frpc`
- 查找顺序：
  - `FRPC_BIN`
  - `.tools/frp/bin/frpc`
  - PATH 中的 `frpc`

## Start Scripts

目录 `start_frp/` 下提供：

- `start-frp-mac.sh`
- `start-frp-linux.sh`
- `start-frp-windows.bat`

默认都使用当前目录 `frpc.toml`，也支持把配置文件路径作为第一个参数传入。

## Sources

- 官方 README: <https://github.com/fatedier/frp?tab=readme-ov-file>
- 客户端配置参考: <https://gofrp.org/en/docs/reference/client-configures/>
- 代理配置参考: <https://gofrp.org/en/docs/reference/proxy/>
- 访问器配置参考: <https://gofrp.org/en/docs/reference/visitor/>

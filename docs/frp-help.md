# frp / frpc 使用说明

## 下载

当前应用内下载链接：

{{DOWNLOAD_URL}}

当前平台文件名：

{{FILENAME}}

常用镜像链接：

- GitHub 官方：`https://github.com/fatedier/frp/releases/download/{tag}/{filename}`
- ghproxy：`https://ghproxy.com/https://github.com/fatedier/frp/releases/download/{tag}/{filename}`

下载命令示例：

```bash
wget "{{DOWNLOAD_URL}}"
```

```bash
curl -L -o "{{FILENAME}}" "{{DOWNLOAD_URL}}"
```

```bash
aria2c -x 8 -s 8 "{{DOWNLOAD_URL}}"
```

## 路径

默认工作目录：

```text
{{WORK_DIR}}
```

默认工具路径：

```text
{{TOOL_PATH}}
```

默认配置路径：

```text
{{CONFIG_PATH}}
```

## 手动安装

macOS：

1. 下载 `darwin` 包。
2. 解压后把 `frpc` 放到设置里的工具路径。
3. 确保配置路径里的 `frpc.toml` 已存在。

Windows：

1. 下载 `windows_amd64.zip`。
2. 解压后把 `frpc.exe` 放到设置里的工具路径。
3. 设置里的工具路径必须和 `frpc.exe` 实际路径一致。
4. 如果 Windows 安全中心拦截，请手动下载、解压，并在设置里确认路径。

## 启动命令

macOS / Linux：

```bash
"{{TOOL_PATH}}" -c "{{CONFIG_PATH}}"
```

Windows：

```powershell
"& \"{{TOOL_PATH}}\" -c \"{{CONFIG_PATH}}\""
```

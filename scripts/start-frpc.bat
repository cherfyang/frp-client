@echo off
setlocal

if not "%OS%"=="Windows_NT" (
  echo 错误：当前系统不是 Windows，请使用 start-frpc.sh 脚本。
  exit /b 1
)

set ROOT_DIR=%~dp0..
set CONFIG_PATH=%ROOT_DIR%\config\frpc.toml
if not "%~1"=="" set CONFIG_PATH=%~1
set TOOLS_DIR=%ROOT_DIR%\.tools
set FRPC_BIN=%TOOLS_DIR%\windows\frpc.exe

if not exist "%FRPC_BIN%" (
  echo 未找到 Windows 的 frpc，正在安装...
  bash "%ROOT_DIR%\scripts\setup-frpc.sh"
)

if not exist "%FRPC_BIN%" (
  echo 错误：安装后仍未找到 %FRPC_BIN%
  exit /b 1
)

echo 使用配置文件：%CONFIG_PATH%
echo frpc 路径：%FRPC_BIN%
"%FRPC_BIN%" -c "%CONFIG_PATH%"

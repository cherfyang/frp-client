@echo off
setlocal

rem 检查是否在 Windows 系统运行
if not "%OS%"=="Windows_NT" (
  echo 错误：当前系统不是 Windows，请使用 macOS 或 Linux 系统对应的启动脚本。
  exit /b 1
)

set ROOT_DIR=%~dp0..
set CONFIG_PATH=%ROOT_DIR%\config\frpc.toml
if not "%~1"=="" set CONFIG_PATH=%~1
set LOCAL_BIN=%ROOT_DIR%\.tools\frp\bin\frpc.exe
set VERSION_FILE=%ROOT_DIR%\.tools\frp\VERSION

rem 检查 frpc 是否存在
if exist "%LOCAL_BIN%" (
  goto :run_frpc
)

rem 不存在则尝试自动安装
echo frpc 不存在，正在尝试自动安装...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri 'https://api.github.com/repos/fatedier/frp/releases/latest' -OutFile '%TEMP%\frp_release.json'" 2>nul || goto :fail_manual

for /f "tokens=2 delims=:" %%a in ('findstr /i "tag_name" "%TEMP%\frp_release.json"') do set VERSION=%%a
set VERSION=%VERSION:"=%
set VERSION=%VERSION: =%

set PLATFORM=windows
set ARCH=amd64
set DOWNLOAD_URL=https://github.com/fatedier/frp/releases/download/%VERSION%/frp_%VERSION:~1%_%PLATFORM%_%ARCH%.zip
set EXTRACT_DIR=%ROOT_DIR%\.tools\frp

echo 正在下载 %VERSION% (windows/%ARCH%)...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri '%DOWNLOAD_URL%' -OutFile '%TEMP%\frp.zip'" 2>nul || goto :fail_manual

echo 正在解压...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Expand-Archive -Path '%TEMP%\frp.zip' -DestinationPath '%TEMP%\frp_extract' -Force" 2>nul || goto :fail_manual

mkdir "%EXTRACT_DIR%\bin" 2>nul
copy "%TEMP%\frp_extract\*\frpc.exe" "%LOCAL_BIN%" >nul 2>&1
echo %VERSION% > "%VERSION_FILE%"

echo frpc 安装完成：%LOCAL_BIN%

:run_frpc
if not "%FRPC_BIN%"=="" (
  set FRPC_COMMAND=%FRPC_BIN%
) else if exist "%LOCAL_BIN%" (
  set FRPC_COMMAND=%LOCAL_BIN%
) else (
  where frpc >nul 2>nul
  if errorlevel 1 (
    echo 未找到 frpc 可执行文件。请先手动下载安装。
    exit /b 1
  )
  set FRPC_COMMAND=frpc
)

echo 使用配置文件：%CONFIG_PATH%
"%FRPC_COMMAND%" -c "%CONFIG_PATH%"
exit /b 0

:fail_manual
echo 自动安装失败，请手动下载 frpc: https://github.com/fatedier/frp/releases
exit /b 1

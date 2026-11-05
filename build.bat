@echo off
rem 一键构建发布包（Windows）：前端 → 嵌入 → 单文件后端
cd /d %~dp0

echo [1/3] 构建前端...
cd web
call npm run build || goto :err
cd ..

echo [2/3] 嵌入前端产物...
rmdir /s /q server\internal\webembed\webdist 2>nul
mkdir server\internal\webembed\webdist
xcopy /e /y /q web\dist\* server\internal\webembed\webdist\ >nul || goto :err
type nul > server\internal\webembed\webdist\.gitkeep

echo [3/3] 编译单文件后端...
cd server
go build -ldflags "-s -w" -o ..\release\huoben.exe ./cmd || goto :err
cd ..

echo.
echo 构建完成: release\huoben.exe
echo 部署: 把 huoben.exe 放到任意空目录运行，浏览器访问 http://localhost:8080
echo 开机自启(管理员): huoben.exe -install  查看状态: huoben.exe -status  卸载: huoben.exe -uninstall
goto :eof

:err
echo 构建失败
exit /b 1

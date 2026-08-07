@echo off
setlocal enabledelayedexpansion

set APP_NAME=proton
set BUILD_DIR=build
set CMD_DIR=.\cmd\proton

if "%1"=="" goto build
if "%1"=="build" goto build
if "%1"=="build-all" goto build-all
if "%1"=="test" goto test
if "%1"=="vet" goto vet
if "%1"=="fmt" goto fmt
if "%1"=="clean" goto clean
if "%1"=="install" goto install
if "%1"=="help" goto help
echo Unknown command: %1
goto help

:build
if not exist %BUILD_DIR% mkdir %BUILD_DIR%
go build -o %BUILD_DIR%\%APP_NAME%.exe %CMD_DIR%
copy /y %BUILD_DIR%\%APP_NAME%.exe %BUILD_DIR%\%APP_NAME%-installer.exe >nul
copy /y scripts\proton.bat %BUILD_DIR%\proton.bat >nul 2>&1
echo Built: %BUILD_DIR%\%APP_NAME%.exe
goto end

:build-all
call :clean
if not exist %BUILD_DIR% mkdir %BUILD_DIR%
for %%p in ("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64" "windows/arm64") do (
    for /f "tokens=1,2 delims=/" %%a in (%%p) do (
        set GOOS=%%a
        set GOARCH=%%b
        set SUFFIX=
        if "%%a"=="windows" set SUFFIX=.exe
        echo Building %%a/%%b...
        set GOOS=%%a&& set GOARCH=%%b&& go build -o %BUILD_DIR%\%APP_NAME%-%%a-%%b!SUFFIX! %CMD_DIR%
        copy /y %BUILD_DIR%\%APP_NAME%-%%a-%%b!SUFFIX! %BUILD_DIR%\%APP_NAME%-installer-%%a-%%b!SUFFIX! >nul
    )
)
echo All binaries and self-installers complete in %BUILD_DIR%\
goto end

:test
go test -count=1 ./...
goto end

:vet
go vet ./...
goto end

:fmt
gofmt -w .
goto end

:install
go install %CMD_DIR%
goto end

:clean
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
goto end

:help
echo Usage: make.cmd [command]
echo.
echo Commands:
echo   build      Build for current platform
echo   build-all  Cross-compile for all platforms
echo   test       Run tests
echo   vet        Run go vet
echo   fmt        Format code
echo   clean      Remove build artifacts
echo   install    Install to GOPATH/bin
echo   help       Show this help

:end
endlocal

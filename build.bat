@echo off
setlocal

cd /d "%~dp0"
if not exist "bin" mkdir "bin"

set BINARIES=gateway flusher meta send

for %%G in (%BINARIES%) do (
    echo Building %%G...
    go build -o "./bin/%%G.exe" "./cmd/%%G" || (
        echo ERROR: build failed for %%G
        exit /b 1
    )
)

echo ALL BUILDS SUCCEEDED.
endlocal
exit /b 0
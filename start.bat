cd /d "%~dp0"
start "" cmd /k "cd /d %~dp0 & bin\meta.exe --config config.meta.yaml"
timeout /t 2 /nobreak >nul
start "" cmd /k "cd /d %~dp0 & bin\registry.exe --config config.registry.yaml"
timeout /t 2 /nobreak >nul
start "" cmd /k "cd /d %~dp0 & bin\flusher.exe --config config.flusher.yaml"
timeout /t 2 /nobreak >nul
start "" cmd /k "cd /d %~dp0 & bin\send.exe --config config.send.yaml"
timeout /t 2 /nobreak >nul
start "" cmd /k "cd /d %~dp0 & bin\gateway.exe --config config.gateway.yaml"
start "" cmd /k "cd /d %~dp0 & bin\gateway2.exe --config config.gateway2.yaml"
exit /b 0
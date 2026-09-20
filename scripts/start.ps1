# Starts all components: Redis, Mock Upstream Services, and API Gateway
$ErrorActionPreference = "Stop"

$env:Path = "C:\Users\VISHALRR\.tools\go\bin;C:\Users\VISHALRR\.tools\redis;C:\Users\VISHALRR\.tools\k6;$env:Path"

Write-Host "[1/3] Starting Redis server..." -ForegroundColor Cyan
$redisProc = Start-Process -FilePath "C:\Users\VISHALRR\.tools\redis\redis-server.exe" -ArgumentList "--appendonly no" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 1

Write-Host "[2/3] Starting Mock Upstream services (ports 9001, 9002, 9003)..." -ForegroundColor Cyan
$upstreamProc = Start-Process -FilePath ".\scripts\mock_upstream.exe" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 1

Write-Host "[3/3] Starting API Gateway on :8080..." -ForegroundColor Cyan
$gatewayProc = Start-Process -FilePath ".\api-gateway.exe" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 1

Write-Host "`nAll services started successfully!" -ForegroundColor Green
Write-Host "  - Redis:        localhost:6379 (PID: $($redisProc.Id))"
Write-Host "  - Upstreams:    localhost:9001, 9002, 9003 (PID: $($upstreamProc.Id))"
Write-Host "  - API Gateway:  http://localhost:8080 (PID: $($gatewayProc.Id))"
Write-Host "`nRun .\scripts\test.ps1 to verify endpoints, or .\scripts\stop.ps1 to shut down."

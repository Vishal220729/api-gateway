# Stops all running instances of the API gateway, mock services, and Redis
Write-Host "Stopping services..." -ForegroundColor Yellow

Get-Process -Name "api-gateway" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "mock_upstream" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "redis-server" -ErrorAction SilentlyContinue | Stop-Process -Force

Write-Host "All gateway processes stopped." -ForegroundColor Green

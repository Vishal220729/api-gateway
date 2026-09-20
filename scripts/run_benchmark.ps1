param (
    [int]$Rate = 2000,
    [string]$Duration = "10s",
    [string]$BaseUrl = "http://localhost:8080"
)

$k6Path = "C:\Users\VISHALRR\.tools\k6\k6.exe"
if (-not (Test-Path $k6Path)) {
    Write-Error "k6 not found at $k6Path"
    exit 1
}

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "       Running Gateway Load Test         " -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "Target:   $BaseUrl"
Write-Host "Rate:     $Rate req/sec"
Write-Host "Duration: $Duration`n"

$env:BASE_URL = $BaseUrl
$env:RATE = "$Rate"
$env:DURATION = "$Duration"

& $k6Path run scripts/quick_load_test.js

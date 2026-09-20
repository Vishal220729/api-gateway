# Tests the running gateway across endpoints, auth, and rate limiting
$ErrorActionPreference = "Continue"

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "       API Gateway Test Suite            " -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

# 1. Health check
Write-Host "`n[1] Health Check (/healthz):" -ForegroundColor Yellow
try {
    $res = Invoke-WebRequest -Uri "http://localhost:8080/healthz" -UseBasicParsing
    Write-Host "  Status: $($res.StatusCode)" -ForegroundColor Green
    Write-Host "  Response: $($res.Content)"
} catch {
    Write-Host "  Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# 2. Public route (/api/orders)
Write-Host "`n[2] Public Route (/api/orders - IP based rate limiting):" -ForegroundColor Yellow
try {
    $res = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -UseBasicParsing
    Write-Host "  Status: $($res.StatusCode)" -ForegroundColor Green
    Write-Host "  X-RateLimit-Limit: $($res.Headers['X-RateLimit-Limit'])"
    Write-Host "  X-RateLimit-Remaining: $($res.Headers['X-RateLimit-Remaining'])"
    Write-Host "  Upstream Response: $($res.Content)"
} catch {
    Write-Host "  Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# 3. Protected route without auth (/api/users)
Write-Host "`n[3] Protected Route without Auth (/api/users):" -ForegroundColor Yellow
try {
    $res = Invoke-WebRequest -Uri "http://localhost:8080/api/users" -UseBasicParsing
    Write-Host "  Unexpected success: $($res.StatusCode)" -ForegroundColor Red
} catch {
    Write-Host "  Expected 401 Unauthorized received." -ForegroundColor Green
}

# 4. Protected route with JWT (/api/users)
Write-Host "`n[4] Protected Route with Valid JWT (/api/users):" -ForegroundColor Yellow
$token = & ".\scripts\generate_token.exe" "client-test-user"
$headers = @{ Authorization = "Bearer $token" }
try {
    $res = Invoke-WebRequest -Uri "http://localhost:8080/api/users" -Headers $headers -UseBasicParsing
    Write-Host "  Status: $($res.StatusCode)" -ForegroundColor Green
    Write-Host "  X-RateLimit-Limit: $($res.Headers['X-RateLimit-Limit'])"
    Write-Host "  X-RateLimit-Remaining: $($res.Headers['X-RateLimit-Remaining'])"
    Write-Host "  Upstream Response: $($res.Content)"
} catch {
    Write-Host "  Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# 5. Prometheus Metrics (/metrics)
Write-Host "`n[5] Prometheus Metrics (/metrics):" -ForegroundColor Yellow
try {
    $res = Invoke-WebRequest -Uri "http://localhost:8080/metrics" -UseBasicParsing
    $rateLimitedMetric = $res.Content -split "`n" | Where-Object { $_ -like "gateway_rate_limited_total*" } | Select-Object -First 1
    Write-Host "  Status: $($res.StatusCode)" -ForegroundColor Green
    Write-Host "  Sample Metric: $rateLimitedMetric"
} catch {
    Write-Host "  Failed: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=========================================" -ForegroundColor Cyan
Write-Host "           Tests Completed!              " -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

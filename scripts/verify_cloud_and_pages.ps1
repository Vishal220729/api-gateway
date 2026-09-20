# ==============================================================================
# Verification Script: 7-Page Multi-Page App + Cloud Integration Suite
# ==============================================================================

Write-Host "`n=== [1/5] Testing 7-Page Multi-Page Static Serving ===" -ForegroundColor Cyan
$pages = @("index.html", "traffic.html", "security.html", "apikeys.html", "routes.html", "cloud.html", "chaos.html")

foreach ($page in $pages) {
    try {
        $res = Invoke-WebRequest -Uri "http://localhost:8080/$page" -Method GET -UseBasicParsing
        if ($res.StatusCode -eq 200) {
            Write-Host "  [OK] Page: $page (HTTP 200, $($res.RawContentLength) bytes)" -ForegroundColor Green
        } else {
            Write-Host "  [FAIL] Page: $page returned $($res.StatusCode)" -ForegroundColor Red
        }
    } catch {
        Write-Host "  [ERROR] Page: $page -> $_" -ForegroundColor Red
    }
}

Write-Host "`n=== [2/5] Testing Cloud Status Endpoint (Multi-Region Topology) ===" -ForegroundColor Cyan
try {
    $cloudStatus = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/cloud/status" -Method GET
    Write-Host "  [OK] Regions Count: $($cloudStatus.regions.Count)" -ForegroundColor Green
    foreach ($r in $cloudStatus.regions) {
        Write-Host "       * $($r.name) [$($r.code)] - Ping: $($r.latency_ms)ms, Replication: $($r.replication), Status: $($r.status)" -ForegroundColor DarkGreen
    }
    Write-Host "  [OK] Webhooks registered: $($cloudStatus.webhooks.Count)" -ForegroundColor Green
    Write-Host "  [OK] Archives created: $($cloudStatus.archives.Count)" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] Cloud Status -> $_" -ForegroundColor Red
}

Write-Host "`n=== [3/5] Testing Cloud Webhooks & Test Alert Dispatch ===" -ForegroundColor Cyan
try {
    $webhookBody = @{
        name = "Datadog Cloud SIEM"
        url = "https://httpbin.org/post"
        provider = "datadog"
        events = @("rate_limit_spike", "circuit_trip", "waf_block")
    } | ConvertTo-Json

    $newWebhook = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/cloud/webhooks" -Method POST -Body $webhookBody -ContentType "application/json"
    Write-Host "  [OK] Webhook created: $($newWebhook.name) (ID: $($newWebhook.id), Secret: $($newWebhook.secret.Substring(0, 12))...)" -ForegroundColor Green

    # Send test alert
    $alertBody = @{
        event = "rate_limit_spike"
        message = "High traffic spike detected on /api/orders (950 req/min)"
        severity = "WARNING"
    } | ConvertTo-Json

    $alertRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/cloud/test-webhook" -Method POST -Body $alertBody -ContentType "application/json"
    Write-Host "  [OK] Alert dispatched to $($alertRes.dispatched) webhook(s) with HMAC-SHA256 signature!" -ForegroundColor Green

    # Clean up webhook
    Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/cloud/webhooks?id=$($newWebhook.id)" -Method DELETE | Out-Null
    Write-Host "  [OK] Webhook deleted successfully" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] Cloud Webhooks -> $_" -ForegroundColor Red
}

Write-Host "`n=== [4/5] Testing S3 / GCS Log Archival ===" -ForegroundColor Cyan
try {
    $archiveRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/cloud/archive" -Method POST -ContentType "application/json"
    Write-Host "  [OK] S3 Archive Created!" -ForegroundColor Green
    Write-Host "       * Bucket: s3://$($archiveRes.bucket)/$($archiveRes.file_name)" -ForegroundColor DarkGreen
    Write-Host "       * Records: $($archiveRes.record_count), Size: $($archiveRes.size_bytes) bytes, Compression: $($archiveRes.compression)" -ForegroundColor DarkGreen
} catch {
    Write-Host "  [FAIL] S3 Archival -> $_" -ForegroundColor Red
}

Write-Host "`n=== [5/5] Testing 1MB Body Size Guard (HTTP 413) ===" -ForegroundColor Cyan
try {
    $largePayload = "X" * (1024 * 1024 + 1024) # 1MB + 1KB
    $largeRes = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -Method POST -Body $largePayload -UseBasicParsing
    Write-Host "  [UNEXPECTED] Expected 413, got $($largeRes.StatusCode)" -ForegroundColor Yellow
} catch [System.Net.WebException] {
    $statusCode = [int]$_.Exception.Response.StatusCode
    if ($statusCode -eq 413) {
        Write-Host "  [OK] Oversized payload blocked with HTTP 413 Payload Too Large!" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] Expected HTTP 413, got $statusCode" -ForegroundColor Red
    }
} catch {
    Write-Host "  [FAIL] Body Size Guard -> $_" -ForegroundColor Red
}

Write-Host "`n>>> All Cloud Integration & Multi-Page Verifications Completed! <<<`n" -ForegroundColor Green

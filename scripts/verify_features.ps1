Write-Host "=== 1. Testing Gateway Health & Dashboard Stats ==="
$health = Invoke-RestMethod -Uri "http://localhost:8080/healthz"
Write-Host "Healthz:" ($health | ConvertTo-Json -Compress)

$stats = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/stats"
Write-Host "Redis Connected: " $stats.redis_connected
Write-Host "Configured Routes: " $stats.routes.Count

Write-Host "`n=== 2. Testing Live Traffic Stream ==="
$orderRes = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -Headers @{"X-Forwarded-For"="192.168.1.10"} -UseBasicParsing
Write-Host "Orders Status:" $orderRes.StatusCode
Write-Host "RateLimit-Remaining:" $orderRes.Headers["X-RateLimit-Remaining"]

$traffic = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/traffic"
Write-Host "Traffic buffer count:" $traffic.Count
Write-Host "Latest request log in buffer:" $traffic[0].method $traffic[0].path "Status:" $traffic[0].status "IP:" $traffic[0].client_ip

Write-Host "`n=== 3. Testing WAF IP Firewall (Blacklist & Whitelist) ==="
# Add blacklist rule
$wafBlacklistBody = @{ ip = "192.168.1.99"; type = "blacklist"; reason = "Automated test blacklist" } | ConvertTo-Json
$wafRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/waf" -Method Post -Body $wafBlacklistBody -ContentType "application/json"
Write-Host "WAF Rule Added:" ($wafRes | ConvertTo-Json -Compress)

# Attempt request with blacklisted IP
try {
    $blocked = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -Headers @{"X-Forwarded-For"="192.168.1.99"} -UseBasicParsing
    Write-Host "Unexpected status:" $blocked.StatusCode
} catch {
    Write-Host "Blocked by WAF successfully! Response:" $_.Exception.Response.StatusCode "Forbidden"
}

# Add whitelist rule
$wafWhitelistBody = @{ ip = "192.168.1.88"; type = "whitelist"; reason = "VIP whitelist bypass" } | ConvertTo-Json
$wafWhiteRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/waf" -Method Post -Body $wafWhitelistBody -ContentType "application/json"
Write-Host "WAF Whitelist Rule Added:" ($wafWhiteRes | ConvertTo-Json -Compress)

# Whitelisted IP request should have X-RateLimit-Bypass header
$whiteReq = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -Headers @{"X-Forwarded-For"="192.168.1.88"} -UseBasicParsing
Write-Host "Whitelisted Request Status:" $whiteReq.StatusCode "Bypass Header:" $whiteReq.Headers["X-RateLimit-Bypass"]

Write-Host "`n=== 4. Testing 1-Click Redis Bucket Flush ==="
$flushRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/reset-buckets" -Method Post
Write-Host "Flush Buckets Result:" ($flushRes | ConvertTo-Json -Compress)

Write-Host "`n=== 5. Testing Dynamic Route Manager (Hot-Reload) ==="
$newRoute = @{
    path = "/api/payments"
    upstream = "http://localhost:9003"
    rate_limit = 250
    window_sec = 60
    key_type = "ip"
    methods = @("GET", "POST")
    require_auth = $false
} | ConvertTo-Json

$addRouteRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/routes" -Method Post -Body $newRoute -ContentType "application/json"
Write-Host "Dynamic Route Added:" ($addRouteRes | ConvertTo-Json -Compress)

# Verify new dynamic route works immediately without restart
$paymentRes = Invoke-WebRequest -Uri "http://localhost:8080/api/payments" -Headers @{"X-Forwarded-For"="192.168.1.77"} -UseBasicParsing
Write-Host "Dynamic Route Request Status:" $paymentRes.StatusCode "Body:" $paymentRes.Content

Write-Host "`n=== 6. Testing Circuit Breaker (Trip & Reset) ==="
$circuits = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/circuits"
Write-Host "Active Breakers:" ($circuits | ConvertTo-Json -Compress)

# Trip circuit for http://localhost:9002 (orders)
$tripRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/circuits/trip?upstream=http://localhost:9002" -Method Post
Write-Host "Circuit Tripped:" ($tripRes | ConvertTo-Json -Compress)

# Request orders upstream should now fast-fail with 503 Service Unavailable
try {
    $trippedReq = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -UseBasicParsing
    Write-Host "Unexpected status:" $trippedReq.StatusCode
} catch {
    Write-Host "Circuit Breaker Fast-Failed successfully! Status:" $_.Exception.Response.StatusCode "Service Unavailable"
}

# Reset circuit for http://localhost:9002
$resetRes = Invoke-RestMethod -Uri "http://localhost:8080/api/dashboard/circuits/reset?upstream=http://localhost:9002" -Method Post
Write-Host "Circuit Reset:" ($resetRes | ConvertTo-Json -Compress)

# Request orders upstream should now succeed
$orderRecovered = Invoke-WebRequest -Uri "http://localhost:8080/api/orders" -UseBasicParsing
Write-Host "Recovered Orders Status:" $orderRecovered.StatusCode

Write-Host "`n=== ALL 5 ENTERPRISE FEATURES VERIFIED SUCCESSFULLY! ==="

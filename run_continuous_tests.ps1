# BYOG Continuous Testing & Bug Hunting Runner (until 15:00)
# Automatically executes alternating rounds of:
# 1. TestMassiveHardwareMatrixExhaustive (350+ hardware combinations)
# 2. TestRealClients150ScenariosStress (WebSocket tunnels, real clients, live disconnections)
# 3. Model scheduling & race detection verification
# Logs detailed execution metrics and detects any bugs/panics.

$logPath = "Z:\better-new-api\test_run.log"
"=== BYOG CONTINUOUS TEST SESSION STARTED: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') ===" | Out-File -FilePath $logPath -Encoding utf8

$targetTime = (Get-Date).Date.AddHours(15) # 15:00 today
$round = 1

while ((Get-Date) -lt $targetTime) {
    $nowStr = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "------------------------------------------------------------------------" | Out-File -FilePath $logPath -Append -Encoding utf8
    "[ROUND $round] Started at $nowStr (Target: 15:00)" | Out-File -FilePath $logPath -Append -Encoding utf8
    "------------------------------------------------------------------------" | Out-File -FilePath $logPath -Append -Encoding utf8

    # Phase 1: Massive Hardware Matrix Permutations (350+ scenarios)
    $hwStart = Get-Date
    $hwOut = & go test -v -run TestMassiveHardwareMatrixExhaustive ./service/byog 2>&1
    $hwElapsed = ((Get-Date) - $hwStart).TotalSeconds
    if ($LASTEXITCODE -eq 0) {
        "[ROUND $round] Phase 1 - 350+ Hardware Matrix Permutations: PASS (${hwElapsed}s)" | Out-File -FilePath $logPath -Append -Encoding utf8
    } else {
        "[ROUND $round] Phase 1 - 350+ Hardware Matrix Permutations: FAILED" | Out-File -FilePath $logPath -Append -Encoding utf8
        $hwOut | Out-File -FilePath $logPath -Append -Encoding utf8
    }

    # Phase 2: Real Clients 150 Lifecycle & Concurrency Scenarios
    $rcStart = Get-Date
    $rcOut = & go test -v -run TestRealClients150ScenariosStress ./controller 2>&1
    $rcElapsed = ((Get-Date) - $rcStart).TotalSeconds
    if ($LASTEXITCODE -eq 0) {
        "[ROUND $round] Phase 2 - 150 Real Client Scenarios: PASS (${rcElapsed}s)" | Out-File -FilePath $logPath -Append -Encoding utf8
    } else {
        "[ROUND $round] Phase 2 - 150 Real Client Scenarios: FAILED" | Out-File -FilePath $logPath -Append -Encoding utf8
        $rcOut | Out-File -FilePath $logPath -Append -Encoding utf8
    }

    # Phase 3: Service byog deep test & scheduling bounds
    $dpStart = Get-Date
    $dpOut = & go test -v -run "TestRamLimitAndStrictMobileTiering|TestAvailableRamMB" ./service/byog 2>&1
    $dpElapsed = ((Get-Date) - $dpStart).TotalSeconds
    if ($LASTEXITCODE -eq 0) {
        "[ROUND $round] Phase 3 - RAM & Mobile Tiering Scheduling: PASS (${dpElapsed}s)" | Out-File -FilePath $logPath -Append -Encoding utf8
    } else {
        "[ROUND $round] Phase 3 - RAM & Mobile Tiering Scheduling: FAILED" | Out-File -FilePath $logPath -Append -Encoding utf8
        $dpOut | Out-File -FilePath $logPath -Append -Encoding utf8
    }

    $roundEnd = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "[ROUND $round] Completed at $roundEnd" | Out-File -FilePath $logPath -Append -Encoding utf8
    $round++

    # Brief cooldown between stress cycles to allow TCP socket reuse
    Start-Sleep -Seconds 5
}

"=== BYOG CONTINUOUS TEST SESSION REACHED 15:00: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') ===" | Out-File -FilePath $logPath -Append -Encoding utf8

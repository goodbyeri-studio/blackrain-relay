param(
    [string]$RelayBaseUrl = "http://127.0.0.1:3010",
    [string]$PostgresContainer = "blackrain-relay-dev-pg",
    [string]$OutputPath = "",
    [string[]]$SelectedGroups = @(),
    [int]$ChatTimeoutSeconds = 180,
    [int]$ImageTimeoutSeconds = 300,
    [int]$AccountingTimeoutSeconds = 45,
    [bool]$RequireEndpointCoverage = $true
)

$ErrorActionPreference = "Stop"
$RelayBaseUrl = $RelayBaseUrl.TrimEnd("/")

function Get-PostgresConnection {
    $inspect = docker inspect $PostgresContainer | ConvertFrom-Json
    if (-not $inspect) {
        throw "PostgreSQL container not found: $PostgresContainer"
    }
    $envMap = @{}
    foreach ($entry in $inspect[0].Config.Env) {
        $parts = $entry -split "=", 2
        $envMap[$parts[0]] = $parts[1]
    }
    if (-not $envMap["POSTGRES_USER"] -or -not $envMap["POSTGRES_DB"]) {
        throw "PostgreSQL container configuration is incomplete"
    }
    return @{ User = $envMap["POSTGRES_USER"]; Database = $envMap["POSTGRES_DB"] }
}

$postgres = Get-PostgresConnection

function Invoke-Psql([string]$Sql) {
    return @(docker exec $PostgresContainer psql -U $postgres.User -d $postgres.Database -F "|" -Atc $Sql)
}

function Get-KeyFingerprint([string]$Key) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([Convert]::ToHexString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($Key)))).ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function Get-Snapshot([int]$UserId, [int]$TokenId) {
    $userRow = @(Invoke-Psql "SELECT quota, used_quota FROM users WHERE id = $UserId;") | Select-Object -First 1
    $tokenRow = @(Invoke-Psql "SELECT remain_quota, used_quota FROM tokens WHERE id = $TokenId;") | Select-Object -First 1
    if (-not $userRow -or -not $tokenRow) {
        throw "Unable to read quota snapshot for user $UserId/token $TokenId"
    }
    $user = $userRow -split "\|", 2
    $token = $tokenRow -split "\|", 2
    return [pscustomobject]@{
        UserQuota = [int64]$user[0]
        UserUsedQuota = [int64]$user[1]
        TokenRemainQuota = [int64]$token[0]
        TokenUsedQuota = [int64]$token[1]
    }
}

function Get-ConsumeLogs([int]$UserId) {
    $rows = Invoke-Psql "SELECT id, user_id, token_id, channel_id, prompt_tokens, completion_tokens, quota, \"group\", token_name, model_name FROM logs WHERE type = 2 AND user_id = $UserId ORDER BY id;"
    $logs = @()
    foreach ($line in $rows) {
        if ($line -notmatch "^\d+\|") { continue }
        $parts = $line -split "\|", 10
        $logs += [pscustomobject]@{
            Id = [int]$parts[0]
            UserId = [int]$parts[1]
            TokenId = [int]$parts[2]
            ChannelId = [int]$parts[3]
            PromptTokens = [int]$parts[4]
            CompletionTokens = [int]$parts[5]
            Quota = [int64]$parts[6]
            Group = $parts[7]
            TokenName = $parts[8]
            ModelName = $parts[9]
        }
    }
    return @($logs)
}

function Wait-ForAccounting([int]$UserId, [int]$TokenId, $Before, [int]$ExpectedLogCount, [int]$TimeoutSeconds, [bool]$CheckUser = $false) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $logs = Get-ConsumeLogs $UserId
        $matching = @($logs | Where-Object { $_.TokenId -eq $TokenId })
        if ($matching.Count -ge $ExpectedLogCount) {
            $quota = [int64](($matching | Measure-Object Quota -Sum).Sum)
            $snapshot = Get-Snapshot $UserId $TokenId
            $userSettled = -not $CheckUser -or ($snapshot.UserUsedQuota - $Before.UserUsedQuota -eq $quota -and $snapshot.UserQuota - $Before.UserQuota -eq (-$quota))
            if ($userSettled -and $snapshot.TokenUsedQuota - $Before.TokenUsedQuota -eq $quota -and $snapshot.TokenRemainQuota - $Before.TokenRemainQuota -eq (-$quota)) {
                return [pscustomobject]@{ Snapshot = $snapshot; Logs = $logs }
            }
        }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)
    return [pscustomobject]@{ Snapshot = Get-Snapshot $UserId $TokenId; Logs = Get-ConsumeLogs $UserId }
}

function Invoke-RelayCall([string]$TokenKey, [string]$Group, [string]$Model, [string]$Mode) {
    if ($Mode -eq "image") {
        $payload = @{ model = $Model; prompt = "A clean product icon of a black umbrella centered on a plain white background."; n = 1; size = "1024x1024" } | ConvertTo-Json -Depth 5
        $uri = "$RelayBaseUrl/v1/images/generations"
        $timeout = $ImageTimeoutSeconds
    } elseif ($Mode -eq "gemini") {
        $payload = @{ contents = @(@{ role = "user"; parts = @(@{ text = "Reply with one short sentence about a black umbrella." }) }); generationConfig = @{ responseModalities = @("TEXT") } } | ConvertTo-Json -Depth 8
        $uri = "$RelayBaseUrl/v1beta/models/$Model`:generateContent"
        $timeout = $ChatTimeoutSeconds
    } else {
        $payload = @{ model = $Model; messages = @(@{ role = "user"; content = "Translate this sentence into natural English and return only the translation: 明天下午三点提交项目报告。" }); stream = $false; max_tokens = 32 } | ConvertTo-Json -Depth 5
        $uri = "$RelayBaseUrl/v1/chat/completions"
        $timeout = $ChatTimeoutSeconds
    }

    try {
        $response = Invoke-WebRequest -Method Post -Uri $uri -Headers @{ Authorization = "Bearer sk-$TokenKey" } -ContentType "application/json" -Body $payload -SkipHttpErrorCheck -TimeoutSec $timeout
    } catch {
        return [pscustomobject]@{ HttpStatus = 0; Error = $_.Exception.Message }
    }
    $body = $null
    try { $body = $response.Content | ConvertFrom-Json } catch { }
    $message = $null
    if ([int]$response.StatusCode -ne 200) {
        $message = if ($body.error.message) { [string]$body.error.message } else { "HTTP $($response.StatusCode)" }
    }
    return [pscustomobject]@{
        HttpStatus = [int]$response.StatusCode
        Error = $message
        PromptTokens = if ($body.usage) { [int]$body.usage.prompt_tokens } elseif ($body.usageMetadata) { [int]$body.usageMetadata.promptTokenCount } else { $null }
        CompletionTokens = if ($body.usage) { [int]$body.usage.completion_tokens } elseif ($body.usageMetadata) { [int]$body.usageMetadata.candidatesTokenCount } else { $null }
    }
}

function Get-Mode([string]$Group, [string]$Model) {
    if ($Group -match "gemini" -or $Model -match "^gemini") { return "gemini" }
    if ($Group -match "image|dall-e|flux|stable-diffusion" -or $Model -match "image|dall-e|flux|stable-diffusion") { return "image" }
    return "chat"
}

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds().ToString() + [Random]::Shared.Next(1000, 9999)
$usernameA = "auditA$suffix"
$usernameB = "auditB$suffix"
$passwordA = "Aa9!" + [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(6)).ToLowerInvariant()
$passwordB = "Bb8!" + [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(6)).ToLowerInvariant()
$userIdA = 0
$userIdB = 0

try {
    $status = Invoke-RestMethod -Uri "$RelayBaseUrl/api/status" -TimeoutSec 30
    if (-not $status.success -or -not $status.data.register_enabled -or -not $status.data.password_login_enabled -or -not $status.data.password_register_enabled) {
        throw "Password registration/login is not enabled in the running service"
    }

    $pricing = Invoke-RestMethod -Uri "$RelayBaseUrl/api/pricing" -TimeoutSec 30
    $publishedGroups = @($pricing.usable_group.PSObject.Properties.Name | Sort-Object)
    if ($publishedGroups.Count -eq 0) { throw "Relay published no customer groups" }
    if ($SelectedGroups.Count -gt 0) {
        $unknownGroups = @($SelectedGroups | Where-Object { $_ -notin $publishedGroups })
        if ($unknownGroups.Count -gt 0) { throw "Selected groups are not published: $($unknownGroups -join ', ')" }
        $groups = @($SelectedGroups | Sort-Object -Unique)
    } else { $groups = $publishedGroups }

    $channelRows = Invoke-Psql @'
SELECT id, "group", COALESCE(NULLIF(test_model,''), split_part(models,',',1)), models, base_url, "key"
FROM channels
WHERE status = 1
ORDER BY id;
'@
    $channelsByGroup = @{}
    $channelKeyFingerprints = @{}
    foreach ($line in $channelRows) {
        if ($line -notmatch "^\d+\|") { continue }
        $parts = $line -split "\|", 6
        $channelId = [int]$parts[0]
        $channelGroups = @($parts[1] -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        if ($parts[4] -notmatch "(?i)^https?://deepkey\.top(?:/|$)") { continue }
        $keys = @($parts[5] -split "\r?\n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        if ($keys.Count -ne 1) { throw "Enabled DeepKey channel $channelId does not have exactly one upstream key" }
        $fingerprint = Get-KeyFingerprint $keys[0]
        $channelKeyFingerprints[$channelId] = $fingerprint
        foreach ($channelGroup in $channelGroups) {
            if (-not $channelsByGroup.ContainsKey($channelGroup)) { $channelsByGroup[$channelGroup] = @() }
            $channelsByGroup[$channelGroup] += [pscustomobject]@{ Id = $channelId; TestModel = $parts[2]; Models = @($parts[3] -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ }); KeyFingerprint = $fingerprint }
        }
    }

    foreach ($group in $groups) {
        if (-not $channelsByGroup.ContainsKey($group)) { throw "Published group without an enabled DeepKey channel: $group" }
        $groupFingerprints = @($channelsByGroup[$group].KeyFingerprint | Sort-Object -Unique)
        if ($groupFingerprints.Count -ne 1) { throw "DeepKey group $group maps to multiple upstream key fingerprints" }
    }

    foreach ($registration in @(@{ username = $usernameA; password = $passwordA }, @{ username = $usernameB; password = $passwordB })) {
        $response = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/user/register?turnstile=" -ContentType "application/json" -Body ($registration | ConvertTo-Json) -TimeoutSec 30
        if (-not $response.success) { throw "Registration failed: $($response.message)" }
    }
    $idRows = Invoke-Psql "SELECT username, id FROM users WHERE username IN ('$usernameA','$usernameB') ORDER BY username;"
    foreach ($line in $idRows) {
        $parts = $line -split "\|", 2
        if ($parts[0] -eq $usernameA) { $userIdA = [int]$parts[1] }
        if ($parts[0] -eq $usernameB) { $userIdB = [int]$parts[1] }
    }
    if ($userIdA -le 0 -or $userIdB -le 0 -or $userIdA -eq $userIdB) { throw "Registered customer IDs are missing or not unique" }
    Invoke-Psql "UPDATE users SET quota = 100000000, used_quota = 0 WHERE id IN ($userIdA,$userIdB);" | Out-Null

    $sessionA = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $sessionB = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    foreach ($login in @(@{ Session = $sessionA; Username = $usernameA; Password = $passwordA; Id = $userIdA }, @{ Session = $sessionB; Username = $usernameB; Password = $passwordB; Id = $userIdB })) {
        $result = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/user/login?turnstile=" -WebSession $login.Session -ContentType "application/json" -Body (@{ username = $login.Username; password = $login.Password } | ConvertTo-Json) -TimeoutSec 30
        if (-not $result.success -or [int]$result.data.id -ne $login.Id) { throw "Customer login did not return its unique user ID" }
    }

    $tokenRequests = @()
    foreach ($group in $groups) { $tokenRequests += [pscustomobject]@{ UserId = $userIdA; Session = $sessionA; Group = $group; Name = "audit-$group" } }
    $isolationGroup = @($groups | Where-Object { $_ -eq "gpt" } | Select-Object -First 1)
    if ($isolationGroup.Count -eq 0) { $isolationGroup = @($groups | Select-Object -First 1) }
    $tokenRequests += [pscustomobject]@{ UserId = $userIdB; Session = $sessionB; Group = $isolationGroup[0]; Name = "audit-second-customer" }
    foreach ($request in $tokenRequests) {
        $body = @{ name = $request.Name; expired_time = -1; remain_quota = 50000000; unlimited_quota = $false; model_limits_enabled = $false; model_limits = ""; allow_ips = ""; group = $request.Group; cross_group_retry = $false } | ConvertTo-Json
        $response = Invoke-RestMethod -Method Post -Uri "$RelayBaseUrl/api/token/" -WebSession $request.Session -Headers @{ "New-Api-User" = "$($request.UserId)" } -ContentType "application/json" -Body $body -TimeoutSec 30
        if (-not $response.success) { throw "Token creation failed for group $($request.Group): $($response.message)" }
    }

    $tokenRows = Invoke-Psql "SELECT id, user_id, \"group\", \"key\" FROM tokens WHERE user_id IN ($userIdA,$userIdB) ORDER BY user_id, id;"
    $tokens = @()
    foreach ($line in $tokenRows) {
        if ($line -notmatch "^\d+\|") { continue }
        $parts = $line -split "\|", 4
        $tokens += [pscustomobject]@{ Id = [int]$parts[0]; UserId = [int]$parts[1]; Group = $parts[2]; Key = $parts[3] }
    }
    $tokensA = @($tokens | Where-Object { $_.UserId -eq $userIdA })
    $tokensB = @($tokens | Where-Object { $_.UserId -eq $userIdB })
    if ($tokensA.Count -ne $groups.Count -or $tokensB.Count -ne 1) { throw "Unexpected generated token counts: A=$($tokensA.Count), B=$($tokensB.Count)" }
    $tokenKeysUnique = @($tokens.Key | Sort-Object -Unique).Count -eq $tokens.Count
    $upstreamCollision = [int]((Invoke-Psql "SELECT COUNT(*) FROM tokens t JOIN channels c ON position(t.\"key\" in c.\"key\") > 0 WHERE t.user_id IN ($userIdA,$userIdB);" | Select-Object -First 1))
    if (-not $tokenKeysUnique -or $upstreamCollision -ne 0) { throw "Generated customer keys are not unique or overlap an upstream key" }

    $beforeUsers = @{}
    $beforeUsers[$userIdA] = Get-Snapshot $userIdA $tokensA[0].Id
    $beforeUsers[$userIdB] = Get-Snapshot $userIdB $tokensB[0].Id
    $beforeTokens = @{}
    foreach ($token in $tokensA + $tokensB) {
        $beforeTokens["$($token.UserId):$($token.Id)"] = Get-Snapshot $token.UserId $token.Id
    }
    $tokenByGroup = @{}
    foreach ($token in $tokensA) { $tokenByGroup[$token.Group] = $token }
    $results = @()
    foreach ($group in $groups) {
        $channel = $channelsByGroup[$group][0]
        $token = $tokenByGroup[$group]
        $mode = Get-Mode $group $channel.TestModel
        $attempt = Invoke-RelayCall $token.Key $group $channel.TestModel $mode
        $results += [pscustomobject]@{ Group = $group; ChannelId = $channel.Id; AllowedChannelIds = @($channelsByGroup[$group].Id); TokenId = $token.Id; UserId = $userIdA; Model = $channel.TestModel; Mode = $mode; HttpStatus = $attempt.HttpStatus; PromptTokens = $attempt.PromptTokens; CompletionTokens = $attempt.CompletionTokens; Error = $attempt.Error }
        Write-Output "progress=$group mode=$mode status=$($attempt.HttpStatus)"
        if ($attempt.HttpStatus -ne 200) { throw "Customer call failed for group ${group}: $($attempt.Error)" }
    }

    $secondToken = $tokensB[0]
    $secondChannel = $channelsByGroup[$isolationGroup[0]][0]
    $secondBefore = Get-Snapshot $userIdB $secondToken.Id
    $secondCall = Invoke-RelayCall $secondToken.Key $isolationGroup[0] $secondChannel.TestModel (Get-Mode $isolationGroup[0] $secondChannel.TestModel)
    if ($secondCall.HttpStatus -ne 200) { throw "Second customer call failed: $($secondCall.Error)" }
    Wait-ForAccounting $userIdB $secondToken.Id $secondBefore 1 $AccountingTimeoutSeconds $true | Out-Null
    $failedBefore = Get-Snapshot $userIdB $secondToken.Id
    $failedLogCount = @(Get-ConsumeLogs $userIdB | Where-Object { $_.TokenId -eq $secondToken.Id }).Count
    $failedCall = Invoke-RelayCall $secondToken.Key $isolationGroup[0] "blackrain-audit-invalid-model" "chat"
    if ($failedCall.HttpStatus -eq 200) { throw "Invalid model request unexpectedly succeeded" }
    Start-Sleep -Seconds 2
    $failedAfter = Get-Snapshot $userIdB $secondToken.Id
    $failedLogCountAfter = @(Get-ConsumeLogs $userIdB | Where-Object { $_.TokenId -eq $secondToken.Id }).Count
    if ($failedLogCountAfter -ne $failedLogCount -or $failedAfter.UserUsedQuota -ne $failedBefore.UserUsedQuota -or $failedAfter.TokenUsedQuota -ne $failedBefore.TokenUsedQuota -or $failedAfter.UserQuota -ne $failedBefore.UserQuota -or $failedAfter.TokenRemainQuota -ne $failedBefore.TokenRemainQuota) {
        throw "Failed request created a consume log or changed quota"
    }

    $accounting = @{}
    foreach ($token in $tokensA + $tokensB) {
        $key = "$($token.UserId):$($token.Id)"
        $accounting[$key] = Wait-ForAccounting $token.UserId $token.Id $beforeTokens[$key] 1 $AccountingTimeoutSeconds
    }
    $allLogs = @(Get-ConsumeLogs $userIdA) + @(Get-ConsumeLogs $userIdB)
    foreach ($result in $results) {
        $matching = @($allLogs | Where-Object { $_.UserId -eq $result.UserId -and $_.TokenId -eq $result.TokenId -and $_.Group -eq $result.Group -and $_.ChannelId -in $result.AllowedChannelIds })
        if ($matching.Count -eq 0) { throw "No consume log matched successful group $($result.Group)" }
        foreach ($log in $matching) { if (-not $channelKeyFingerprints.ContainsKey($log.ChannelId)) { throw "Consume log for group $($result.Group) used a non-DeepKey channel $($log.ChannelId)" } }
    }

    $sumA = [int64](($allLogs | Where-Object { $_.UserId -eq $userIdA } | Measure-Object Quota -Sum).Sum)
    $sumB = [int64](($allLogs | Where-Object { $_.UserId -eq $userIdB } | Measure-Object Quota -Sum).Sum)
    $afterUserA = Get-Snapshot $userIdA $tokensA[0].Id
    $afterUserB = Get-Snapshot $userIdB $tokensB[0].Id
    if ($afterUserA.UserUsedQuota - $beforeUsers[$userIdA].UserUsedQuota -ne $sumA -or $afterUserA.UserQuota - $beforeUsers[$userIdA].UserQuota -ne (-$sumA)) { throw "Customer A quota accounting does not match consume logs" }
    if ($afterUserB.UserUsedQuota - $beforeUsers[$userIdB].UserUsedQuota -ne $sumB -or $afterUserB.UserQuota - $beforeUsers[$userIdB].UserQuota -ne (-$sumB)) { throw "Customer B quota accounting does not match consume logs" }
    foreach ($token in $tokensA + $tokensB) {
        $key = "$($token.UserId):$($token.Id)"
        $before = $beforeTokens[$key]
        $after = Get-Snapshot $token.UserId $token.Id
        $sum = [int64](($allLogs | Where-Object { $_.TokenId -eq $token.Id } | Measure-Object Quota -Sum).Sum)
        if ($after.TokenUsedQuota - $before.TokenUsedQuota -ne $sum -or $after.TokenRemainQuota - $before.TokenRemainQuota -ne (-$sum)) { throw "Token $($token.Id) quota accounting does not match consume logs" }
    }

    $modeCoverage = @($results.Mode | Sort-Object -Unique)
    if ($RequireEndpointCoverage) {
        foreach ($requiredMode in @("chat", "image", "gemini")) { if ($requiredMode -notin $modeCoverage) { throw "No published DeepKey group exercised the $requiredMode endpoint" } }
    }
    $resultData = [ordered]@{
        schema_version = 2
        generated_at_utc = (Get-Date).ToUniversalTime().ToString("o")
        relay_url = $RelayBaseUrl
        registered_customer_count = 2
        login_unique_user_ids = ($userIdA -ne $userIdB)
        generated_token_count = $tokens.Count
        all_generated_keys_unique = $tokenKeysUnique
        generated_keys_overlap_upstream = ($upstreamCollision -ne 0)
        published_group_count = $publishedGroups.Count
        tested_group_count = $groups.Count
        successful_group_count = $results.Count
        endpoint_modes = $modeCoverage
        failed_request_status = $failedCall.HttpStatus
        failed_request_created_consume_log = ($failedLogCountAfter -ne $failedLogCount)
        failed_request_changed_quota = ($failedAfter.UserUsedQuota -ne $failedBefore.UserUsedQuota -or $failedAfter.TokenUsedQuota -ne $failedBefore.TokenUsedQuota)
        total_consume_log_count = $allLogs.Count
        total_quota = [int64](($allLogs | Measure-Object Quota -Sum).Sum)
        groups = @($results | Select-Object Group, ChannelId, AllowedChannelIds, UserId, TokenId, Model, Mode, HttpStatus, PromptTokens, CompletionTokens, Error)
        logs = $allLogs
        deepkey_channel_key_fingerprints = @($channelKeyFingerprints.GetEnumerator() | Sort-Object Name | ForEach-Object { [ordered]@{ channel_id = [int]$_.Key; key_sha256 = $_.Value } })
    }
    $json = $resultData | ConvertTo-Json -Depth 10
    if ($OutputPath) {
        $resolved = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
        New-Item -ItemType Directory -Path (Split-Path -Parent $resolved) -Force | Out-Null
        [IO.File]::WriteAllText($resolved, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
        Write-Output "audit_result=$resolved"
    }
    Write-Output $json
} finally {
    Invoke-Psql "DELETE FROM logs WHERE user_id IN (SELECT id FROM users WHERE username IN ('$usernameA','$usernameB')); DELETE FROM quota_data WHERE user_id IN (SELECT id FROM users WHERE username IN ('$usernameA','$usernameB')); DELETE FROM tokens WHERE user_id IN (SELECT id FROM users WHERE username IN ('$usernameA','$usernameB')); DELETE FROM users WHERE username IN ('$usernameA','$usernameB');" | Out-Null
}

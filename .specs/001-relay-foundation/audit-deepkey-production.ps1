param(
    [Parameter(Mandatory = $true)]
    [string]$RelayBaseUrl,

    [Parameter(Mandatory = $true)]
    [string]$AdminAccessToken,

    [Parameter(Mandatory = $true)]
    [int]$AdminUserId,

    [string]$DeepKeyBaseUrl = "https://deepkey.top",
    [string]$OutputPath = ".specs/001-relay-foundation/deepkey-audit-result.json"
)

$ErrorActionPreference = "Stop"
$RelayBaseUrl = $RelayBaseUrl.TrimEnd("/")
$DeepKeyBaseUrl = $DeepKeyBaseUrl.TrimEnd("/")
$headers = @{
    Authorization = "Bearer $AdminAccessToken"
    "New-Api-User" = $AdminUserId.ToString()
}

$status = Invoke-RestMethod -Method Get -Uri "$RelayBaseUrl/api/status"
$catalog = Invoke-RestMethod -Method Get -Uri "$DeepKeyBaseUrl/api/pricing"
if (-not $catalog.success) {
    throw "DeepKey pricing request failed"
}

$channels = @()
$page = 1
do {
    $response = Invoke-RestMethod -Method Get -Uri "$RelayBaseUrl/api/channel/?p=$page&page_size=100" -Headers $headers
    if (-not $response.success) {
        throw "Relay channel request failed on page $page"
    }
    $items = @($response.data.items)
    $channels += $items
    $page++
} while ($channels.Count -lt [int]$response.data.total)

$deepKeyChannels = @($channels | Where-Object {
    $_.base_url -match "(^|//)deepkey\.top(/|$)" -or $_.name -match "DeepKey"
})
$channelResults = @()
foreach ($channel in $deepKeyChannels) {
    $configuredModels = @($channel.models -split "," | ForEach-Object { $_.Trim() } | Where-Object { $_ } | Sort-Object -Unique)
    $probe = Invoke-RestMethod -Method Get -Uri "$RelayBaseUrl/api/channel/fetch_models/$($channel.id)" -Headers $headers
    $authorizedModels = @()
    $probeError = $null
    if ($probe.success) {
        $authorizedModels = @($probe.data | ForEach-Object { [string]$_ } | Sort-Object -Unique)
    } else {
        $probeError = [string]$probe.message
    }
    $channelResults += [ordered]@{
        channel_id = [int]$channel.id
        name = [string]$channel.name
        group = [string]$channel.group
        enabled = ([int]$channel.status -eq 1)
        configured_model_count = $configuredModels.Count
        authorized_model_count = $authorizedModels.Count
        configured_not_authorized = @($configuredModels | Where-Object { $_ -notin $authorizedModels })
        authorized_not_configured = @($authorizedModels | Where-Object { $_ -notin $configuredModels })
        probe_error = $probeError
    }
}

$summary = [ordered]@{
    schema_version = 1
    generated_at_utc = (Get-Date).ToUniversalTime().ToString("o")
    relay_url = $RelayBaseUrl
    relay_version = [string]$status.data.version
    deepkey_url = $DeepKeyBaseUrl
    catalog_model_count = @($catalog.data).Count
    catalog_group_count = @($catalog.group_ratio.PSObject.Properties).Count
    deepkey_channel_count = $deepKeyChannels.Count
    enabled_deepkey_channel_count = @($deepKeyChannels | Where-Object { [int]$_.status -eq 1 }).Count
    successful_model_probes = @($channelResults | Where-Object { $null -eq $_.probe_error }).Count
    failed_model_probes = @($channelResults | Where-Object { $null -ne $_.probe_error }).Count
    channels_with_model_mismatch = @($channelResults | Where-Object {
        $_.configured_not_authorized.Count -gt 0 -or $_.authorized_not_configured.Count -gt 0
    }).Count
    channels = $channelResults
}

$json = $summary | ConvertTo-Json -Depth 8
$resolvedOutput = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
$outputDirectory = Split-Path -Parent $resolvedOutput
if ($outputDirectory) {
    New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
}
[System.IO.File]::WriteAllText($resolvedOutput, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
$hash = (Get-FileHash -Path $resolvedOutput -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Output "audit_result=$resolvedOutput"
Write-Output "sha256=$hash"

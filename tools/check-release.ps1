param(
    [string]$ExpectedVersion = $env:UTAUTTS_RELEASE_VERSION,
    [string]$PreviousVersion = $env:UTAUTTS_PREVIOUS_VERSION,
    [int]$PreviousUpdateSchema = 0,
    [int]$PreviousInstallLayout = 0,
    [int]$PreviousMigrationSchema = 0,
    [string]$AppInfoPath = "$(Join-Path $PSScriptRoot '..\internal\appinfo\appinfo.json')"
)

$ErrorActionPreference = 'Stop'

function Get-VersionParts([string]$Value) {
    $trimmed = $Value.Trim()
    $match = [regex]::Match($trimmed, '^v(\d+)\.(\d+)\.(\d+)$')
    if (-not $match.Success) {
        throw "Version must use the release tag format vMAJOR.MINOR.PATCH: $Value"
    }
    return [int[]]@(
        [int]$match.Groups[1].Value,
        [int]$match.Groups[2].Value,
        [int]$match.Groups[3].Value
    )
}

function Compare-Version([string]$Left, [string]$Right) {
    $leftParts = @(Get-VersionParts $Left)
    $rightParts = @(Get-VersionParts $Right)
    foreach ($index in 0..2) {
        if ($leftParts[$index] -lt $rightParts[$index]) { return -1 }
        if ($leftParts[$index] -gt $rightParts[$index]) { return 1 }
    }
    return 0
}

if (-not (Test-Path -LiteralPath $AppInfoPath -PathType Leaf)) {
    throw "Application metadata was not found: $AppInfoPath"
}

$metadata = Get-Content -LiteralPath $AppInfoPath -Raw | ConvertFrom-Json
$currentVersion = [string]$metadata.version
Get-VersionParts $currentVersion | Out-Null

if (-not [string]::IsNullOrWhiteSpace($ExpectedVersion) -and
    $currentVersion -ne $ExpectedVersion.Trim()) {
    throw "appinfo.json version $currentVersion does not match expected release $($ExpectedVersion.Trim())"
}
if (-not [string]::IsNullOrWhiteSpace($PreviousVersion) -and
    (Compare-Version $currentVersion $PreviousVersion.Trim()) -le 0) {
    throw "release version $currentVersion must be newer than $($PreviousVersion.Trim())"
}

# v1.2.2 predates the release metadata fields. Keep its known on-disk
# baseline here so the first release after v1.2.2 gets the migration check even
# when the caller only supplies -PreviousVersion.
if (-not [string]::IsNullOrWhiteSpace($PreviousVersion) -and
    $PreviousVersion.Trim() -eq 'v1.2.2') {
    if ($PreviousUpdateSchema -eq 0) { $PreviousUpdateSchema = 1 }
    if ($PreviousInstallLayout -eq 0) { $PreviousInstallLayout = 1 }
}

$updateSchema = 0
$migrationSchema = 0
$installLayout = 0
if ($null -ne $metadata.update_schema) { $updateSchema = [int]$metadata.update_schema }
if ($null -ne $metadata.migration_schema) { $migrationSchema = [int]$metadata.migration_schema }
if ($null -ne $metadata.install_layout) { $installLayout = [int]$metadata.install_layout }
if ($updateSchema -lt 1) { throw 'update_schema must be a positive integer' }
if ($migrationSchema -lt 1) { throw 'migration_schema must be a positive integer' }
if ($installLayout -lt 1) { throw 'install_layout must be a positive integer' }

if ($PreviousInstallLayout -gt 0) {
    if ($installLayout -lt $PreviousInstallLayout) {
        throw "install_layout cannot decrease ($installLayout < $PreviousInstallLayout)"
    }
    if ($installLayout -gt $PreviousInstallLayout -and
        $migrationSchema -le $PreviousMigrationSchema) {
        throw "install_layout changed from $PreviousInstallLayout to $installLayout without a newer migration_schema"
    }
}
if ($PreviousUpdateSchema -gt 0 -and $updateSchema -lt $PreviousUpdateSchema) {
    throw "update_schema cannot decrease ($updateSchema < $PreviousUpdateSchema)"
}
if ($PreviousMigrationSchema -gt 0 -and $migrationSchema -lt $PreviousMigrationSchema) {
    throw "migration_schema cannot decrease ($migrationSchema < $PreviousMigrationSchema)"
}

Write-Host ("Release metadata OK: version={0}, update_schema={1}, migration_schema={2}, install_layout={3}" -f `
    $currentVersion, $updateSchema, $migrationSchema, $installLayout)

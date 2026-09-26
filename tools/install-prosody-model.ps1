param(
    [Parameter(Mandatory = $true)]
    [string]$ModelPath,
    [string]$DestinationDirectory = "models",
    [string]$Id = "",
    [string]$DisplayName = "",
    [string]$Description = "",
    [string[]]$RecommendedRenderer = @()
)

$ErrorActionPreference = 'Stop'
$source = [IO.Path]::GetFullPath($ModelPath)
$destinationRoot = [IO.Path]::GetFullPath($DestinationDirectory)
$raw = Get-Content -LiteralPath $source -Raw -Encoding UTF8
$model = $raw | ConvertFrom-Json
$migrateIdentity = -not [string]::IsNullOrWhiteSpace($Id) -or
    -not [string]::IsNullOrWhiteSpace($DisplayName) -or
    -not [string]::IsNullOrWhiteSpace($Description) -or
    $RecommendedRenderer.Count -gt 0
if ($migrateIdentity -and -not [string]::IsNullOrWhiteSpace([string]$model.id) -and -not [string]::IsNullOrWhiteSpace([string]$model.display_name)) {
    throw 'The source already has model identity; do not use migration metadata arguments.'
}
if ($migrateIdentity) {
    if ([string]::IsNullOrWhiteSpace($Id) -or [string]::IsNullOrWhiteSpace($DisplayName)) {
        throw 'Migration requires both Id and DisplayName.'
    }
    $identity = [ordered]@{}
    if ([string]::IsNullOrWhiteSpace([string]$model.id)) { $identity['id'] = $Id }
    if ([string]::IsNullOrWhiteSpace([string]$model.display_name)) { $identity['display_name'] = $DisplayName }
    if (-not [string]::IsNullOrWhiteSpace($Description)) { $identity['description'] = $Description }
    if ($RecommendedRenderer.Count -gt 0) { $identity['recommended_renderers'] = @($RecommendedRenderer) }
    $metadata = ($identity | ConvertTo-Json -Compress).Trim('{', '}')
    $brace = $raw.IndexOf('{')
    if ($brace -lt 0) { throw 'The model JSON does not start with an object.' }
    $raw = $raw.Substring(0, $brace + 1) + "`n  " + ($metadata -replace ',', ",`n  ") + "," + $raw.Substring($brace + 1)
    $model = $raw | ConvertFrom-Json
}
if ([string]::IsNullOrWhiteSpace([string]$model.id)) {
    throw 'The model does not contain a stable id.'
}
if ([string]::IsNullOrWhiteSpace([string]$model.display_name)) {
    throw 'The model does not contain display_name.'
}
if ([string]$model.id -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') {
    throw "Invalid model id: $($model.id)"
}
if ([string]::IsNullOrWhiteSpace([string]$model.license)) {
    throw 'The model must contain a license description.'
}
$notices = @()
if ($null -ne $model.license_notices) {
    $notices = @($model.license_notices)
} elseif (-not [string]::IsNullOrWhiteSpace([string]$model.license_notice)) {
    $notices = @([string]$model.license_notice)
}
if ($notices.Count -eq 0) {
    throw 'The model must contain a license_notices array.'
}
foreach ($rawNotice in $notices) {
    $licenseNotice = ([string]$rawNotice).Trim()
    if ($licenseNotice -ne [string]$rawNotice -or
        $licenseNotice -notmatch '^licenses/[^/\\]+(?:/[^/\\]+)*$' -or
        $licenseNotice -match '(^|/)\.\.?(/|$)' -or
        $licenseNotice.Contains(':')) {
        throw 'The model license_notices must be normalized paths below licenses/.'
    }
}
New-Item -ItemType Directory -Force -Path $destinationRoot | Out-Null
$destination = Join-Path $destinationRoot ($model.id + '.json')
$raw = $raw -replace "`r`n", "`n"
[IO.File]::WriteAllText($destination, $raw.TrimEnd("`n") + "`n", [Text.UTF8Encoding]::new($false))
Write-Host "Installed model $($model.display_name) ($($model.id)): $destination"

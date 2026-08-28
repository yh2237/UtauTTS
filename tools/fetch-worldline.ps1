param(
    [Parameter(Mandatory = $true)]
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$source = Join-Path $root 'assets/worldline/win-x64/worldline.dll'
$expectedHash = '655D918375643BAD1A3FF95E9E76F0B560B6CAA009370BB56498407D1F5F0C28'
$output = [IO.Path]::GetFullPath($OutputPath)
$directory = Split-Path -Parent $output

New-Item -ItemType Directory -Force -Path $directory | Out-Null
$actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $source).Hash
if ($actualHash -ne $expectedHash) {
    throw "worldline.dll SHA-256 mismatch: $actualHash"
}
Copy-Item -Force -LiteralPath $source -Destination $output

# Regenerates qt/fonts/MaterialSymbolsOutlined-subset.ttf from upstream.
# Requires Python with fonttools (pip install fonttools).
# Usage: powershell -ExecutionPolicy Bypass -File tools/subset-material-symbols.ps1 [-Unicodes "U+E925,U+E3C9"]
param(
    [string]$Unicodes = "U+E925,U+E3C9,U+E5D4,U+E5CD,U+E145,U+E037,U+E034,U+E5D3,U+E5D5",
    [string]$UpstreamUrl = "https://raw.githubusercontent.com/google/material-design-icons/master/variablefont/MaterialSymbolsOutlined%5BFILL%2CGRAD%2Copsz%2Cwght%5D.ttf"
)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$workDir = Join-Path ([IO.Path]::GetTempPath()) 'utautts-material-symbols'
New-Item -ItemType Directory -Force -Path $workDir | Out-Null
$upstreamFile = Join-Path $workDir 'MaterialSymbolsOutlined-full.ttf'
$subsetFile = Join-Path $root 'qt/fonts/MaterialSymbolsOutlined-subset.ttf'
Invoke-WebRequest -Uri $UpstreamUrl -OutFile $upstreamFile
& python -m fontTools.subset $upstreamFile --unicodes=$Unicodes --layout-features="" `
    --no-hinting --desubroutinize --output-file=$subsetFile
if ($LASTEXITCODE -ne 0) { throw 'fontTools subset failed' }
Write-Output "Wrote $subsetFile ($((Get-Item -LiteralPath $subsetFile).Length) bytes)"
Write-Output 'Update licenses/MATERIAL-SYMBOLS.txt if the glyph set changed.'

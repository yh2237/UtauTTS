$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$dist = Join-Path $PSScriptRoot 'dist'

New-Item -ItemType Directory -Force -Path $dist | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $dist 'models') | Out-Null

$env:CGO_ENABLED = '0'
$env:GOOS = 'js'
$env:GOARCH = 'wasm'

Push-Location $root
try {
    & go build -trimpath -ldflags '-s -w' -o (Join-Path $dist 'utautts.wasm') ./cmd/utautts-wasm
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

$goroot = (& go env GOROOT).Trim()
Copy-Item -LiteralPath (Join-Path $goroot 'lib/wasm/wasm_exec.js') -Destination (Join-Path $dist 'wasm_exec.js') -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'fs-shim.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'openjtalk-bridge.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'world-bridge.js') -Destination $dist -Force
$modelFiles = Get-ChildItem -Path (Join-Path $root 'models') -Filter *.json -File
foreach ($modelFile in $modelFiles) {
    Copy-Item -LiteralPath $modelFile.FullName -Destination (Join-Path $dist 'models') -Force
}
$modelManifest = @{ models = @($modelFiles | ForEach-Object { $_.Name }) } | ConvertTo-Json -Compress
Set-Content -LiteralPath (Join-Path $dist 'models/manifest.json') -Value $modelManifest -Encoding utf8
Write-Host ("Bundled models: " + (($modelFiles | ForEach-Object { $_.Name }) -join ', '))

$voiceZip = Get-ChildItem -Path (Join-Path $root 'voice') -Filter *.zip -File -ErrorAction SilentlyContinue | Select-Object -First 1
if ($voiceZip) {
    $voiceStage = 'out/web-voice-' + [guid]::NewGuid().ToString('N')
    Push-Location $root
    try {
        $env:GOOS = 'windows'; $env:GOARCH = (& go env GOHOSTARCH).Trim()
        & go run ./cmd/tools/build-voice $voiceZip.FullName $voiceStage
    } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0) { throw "voice bank packaging failed" }
    $stagePath = [IO.Path]::GetFullPath((Join-Path $root $voiceStage))
    $outRoot = [IO.Path]::GetFullPath((Join-Path $root 'out')) + [IO.Path]::DirectorySeparatorChar
    if (-not $stagePath.StartsWith($outRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'voice staging path escaped out/' }
    New-Item -ItemType Directory -Force -Path (Join-Path $dist 'voice') | Out-Null
    Copy-Item -Path (Join-Path $stagePath '*') -Destination (Join-Path $dist 'voice') -Recurse -Force
    Remove-Item -LiteralPath $stagePath -Recurse -Force
} else {
    Write-Host "No bundled voicebank zip found under voice/"
}

Write-Host "Built $dist"
Write-Host "Build the browser UI next with qt/wasm/build.ps1"

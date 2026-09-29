# UtauTTS wasm向けビルド。GoのwasmとJS資産、既定の抑揚モデルを web/dist へ集める。
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$dist = Join-Path $PSScriptRoot 'dist'
$modelName = 'frame-intonation-tcn-v9.1-t.json'

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
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'index.html') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'main.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'client.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'worker.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'fs-shim.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'openjtalk-bridge.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'world-bridge.js') -Destination $dist -Force
Copy-Item -LiteralPath (Join-Path $root "models/$modelName") -Destination (Join-Path $dist "models/$modelName") -Force

Write-Host "Built $dist"
Write-Host "Serve it with:  python -m http.server --directory `"$dist`""

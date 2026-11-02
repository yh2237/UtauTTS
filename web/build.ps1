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
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'reference-inference.js') -Destination $dist -Force

# 参照照合の推論に使うONNX Runtime Web（MIT）。版とハッシュを固定して取る。
$ortVersion = '1.23.2'
$ortSha256 = '0b8707d7efab9a2bea63564d66cad79e897cfd0bd627e5ae000488ffc1c45c7d'
$ortDir = Join-Path $dist 'ort'
$ortVersionFile = Join-Path $ortDir 'version'
if (-not (Test-Path -LiteralPath $ortVersionFile) -or (Get-Content -LiteralPath $ortVersionFile -Raw).Trim() -ne $ortVersion) {
    $ortTgz = Join-Path ([IO.Path]::GetTempPath()) "onnxruntime-web-$ortVersion.tgz"
    Invoke-WebRequest -Uri "https://registry.npmjs.org/onnxruntime-web/-/onnxruntime-web-$ortVersion.tgz" -OutFile $ortTgz
    if ((Get-FileHash -LiteralPath $ortTgz -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ortSha256) { throw "onnxruntime-web hash mismatch" }
    if (Test-Path -LiteralPath $ortDir) { Remove-Item -LiteralPath $ortDir -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $ortDir | Out-Null
    & tar -xzf $ortTgz -C $ortDir --strip-components=2 package/dist/ort.wasm.min.js package/dist/ort-wasm-simd-threaded.mjs package/dist/ort-wasm-simd-threaded.wasm
    if ($LASTEXITCODE -ne 0) { throw "tar failed with exit code $LASTEXITCODE" }
    Set-Content -LiteralPath $ortVersionFile -Value $ortVersion -Encoding ascii
    Remove-Item -LiteralPath $ortTgz -Force
}
# 参照照合のモデル（tools/web-reference/export.pyで作る）は大きいので、UTAUTTS_WEB_REFERENCE を指定したときだけ写す。
if ($env:UTAUTTS_WEB_REFERENCE) {
    $referenceDir = Join-Path $dist 'reference'
    if (Test-Path -LiteralPath $referenceDir) { Remove-Item -LiteralPath $referenceDir -Recurse -Force }
    Copy-Item -LiteralPath $env:UTAUTTS_WEB_REFERENCE -Destination $referenceDir -Recurse
}
# 同梱から外したモデルが前回のビルドから残らないよう、先に消す。
Get-ChildItem -Path (Join-Path $dist 'models') -Filter *.json -File | Remove-Item -Force
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

# Qt wasm アプリをビルドし、web/dist と renderer をまとめた配布ディレクトリを作る。
# 事前に emsdk を有効化し、web/dist のGo wasm・Open JTalk・WORLD資産を用意しておくこと。
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$buildDir = Join-Path $root 'build\qt-wasm'
$releaseDir = Join-Path $root 'build\web-release'

$qtWasm = $env:QT_WASM_DIR
if (-not $qtWasm) { $qtWasm = 'C:\Users\2237n\Qt-wasm\6.8.3\wasm_singlethread' }
$qtHost = $env:QT_HOST_PATH
if (-not $qtHost) { $qtHost = Join-Path $root '.qt\6.8.3\mingw_64' }

& (Join-Path $qtWasm 'bin\qt-cmake') -S (Join-Path $root 'qt') -B $buildDir `
    -DCMAKE_BUILD_TYPE=Release "-DQT_HOST_PATH=$qtHost"
if ($LASTEXITCODE -ne 0) { throw "qt-cmake configure failed" }
& cmake --build $buildDir
if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }

if (Test-Path $releaseDir) { Remove-Item -Recurse -Force $releaseDir }
New-Item -ItemType Directory -Force -Path (Join-Path $releaseDir 'web') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $releaseDir 'renderer\utautts-world-phrase') | Out-Null
foreach ($name in 'index.html', 'engine-loader.js', 'engine-worker.js', 'utautts.js', 'utautts.wasm', 'qtloader.js') {
    Copy-Item -LiteralPath (Join-Path $buildDir $name) -Destination $releaseDir -Force
}
Copy-Item -Recurse -Force (Join-Path $root 'web\dist') (Join-Path $releaseDir 'web\dist')
Copy-Item -LiteralPath (Join-Path $root 'renderer\utautts-world-phrase\renderer.json') `
    -Destination (Join-Path $releaseDir 'renderer\utautts-world-phrase\renderer.json') -Force

Write-Host "Assembled $releaseDir"

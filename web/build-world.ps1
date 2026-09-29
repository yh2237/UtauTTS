# WORLD (WORLD C++ + native/world-engine) を WebAssembly へビルドする。
# 事前に Emscripten SDK を activate しておくこと。
$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$outDir = Join-Path $PSScriptRoot 'dist/world'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$emcc = if ($env:EMXX) { $env:EMXX } else { 'em++' }
$worldSrc = Join-Path $root 'third_party/world/src'
$sources = @(
    (Join-Path $root 'native/world-engine/world_engine.cpp'),
    (Join-Path $worldSrc 'cheaptrick.cpp'),
    (Join-Path $worldSrc 'common.cpp'),
    (Join-Path $worldSrc 'd4c.cpp'),
    (Join-Path $worldSrc 'fft.cpp'),
    (Join-Path $worldSrc 'harvest.cpp'),
    (Join-Path $worldSrc 'matlabfunctions.cpp'),
    (Join-Path $worldSrc 'synthesis.cpp')
)
$exported = '_UtauTTSWorldF0,_UtauTTSWorldAnalysisShape,_UtauTTSWorldAnalyze,_UtauTTSWorldSynthesize,_UtauTTSWorldAnalyzeFlat,_UtauTTSWorldSynthesizeFlat,_malloc,_free'

$arguments = @(
    '-O3', '-msimd128', '-std=c++17', "-I$worldSrc"
) + $sources + @(
    '-fexceptions',
    '-sMODULARIZE=1',
    '-sEXPORT_NAME=createUtauTTSWorld',
    '-sENVIRONMENT=web,worker,node',
    '-sALLOW_MEMORY_GROWTH=1',
    '-sSTACK_SIZE=16MB',
    '-sEXPORTED_RUNTIME_METHODS=ccall,cwrap,UTF8ToString,HEAPF64,HEAP32,HEAPU8',
    "-sEXPORTED_FUNCTIONS=$exported",
    '-o', (Join-Path $outDir 'utautts-world.js')
)

& $emcc @arguments
if ($LASTEXITCODE -ne 0) { throw "em++ failed with exit code $LASTEXITCODE" }
Write-Host "Built $outDir"

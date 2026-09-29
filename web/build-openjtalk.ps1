# Build Open JTalk (third_party/open_jtalk) to WebAssembly and link the C ABI wrapper.
# Requires: activated Emscripten SDK, cmake and ninja on PATH.
param([string]$DictionaryPath = '')

$ErrorActionPreference = 'Stop'

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$jtalk = Join-Path $root 'third_party/open_jtalk/src'
$buildDir = Join-Path $root 'build/openjtalk-wasm'
$outDir = Join-Path $PSScriptRoot 'dist/openjtalk'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

# 1) cmake: build the wasm static library and generate config.h.
if (-not (Test-Path (Join-Path $buildDir 'build.ninja'))) {
    $configure = @(
        'cmake', '-G', 'Ninja', '-S', $jtalk, '-B', $buildDir,
        '-DBUILD_SHARED_LIBS=OFF', '-DBUILD_PROGRAMS=OFF',
        '-DCMAKE_BUILD_TYPE=Release', '-DCMAKE_POLICY_VERSION_MINIMUM=3.5'
    )
    & emcmake @configure
    if ($LASTEXITCODE -ne 0) { throw "emcmake configure failed with exit code $LASTEXITCODE" }
}
& cmake --build $buildDir --target openjtalk
if ($LASTEXITCODE -ne 0) { throw "openjtalk build failed with exit code $LASTEXITCODE" }
$library = Join-Path $buildDir 'libopenjtalk.a'
if (-not (Test-Path $library)) { throw "libopenjtalk.a was not produced" }

# 2) Compile the wrapper and link it into JS + wasm.
$includeDirs = @(
    'jpcommon', 'mecab/src', 'mecab2njd', 'njd', 'njd2jpcommon',
    'njd_set_accent_phrase', 'njd_set_accent_type', 'njd_set_digit',
    'njd_set_long_vowel', 'njd_set_pronunciation', 'njd_set_unvoiced_vowel', 'text2mecab'
)
$includes = foreach ($dir in $includeDirs) { "-I$(Join-Path $jtalk $dir)" }

$defines = @(
    '-DHAVE_CONFIG_H', '-DDIC_VERSION=102', '-DCHARSET_UTF_8',
    '-DMECAB_UTF8_USE_ONLY', '-DMECAB_CHARSET=utf-8'
)

$exported = '_UtauTTSOpenJTalkInit,_UtauTTSOpenJTalkRun,_UtauTTSOpenJTalkError,_malloc,_free'

$arguments = @(
    '-O3', '-std=c++17',
    (Join-Path $root 'native/openjtalk-engine/openjtalk_engine.cpp'),
    $library
)
$arguments += $includes
$arguments += $defines
$arguments += @(
    '-fexceptions',
    '-sMODULARIZE=1',
    '-sEXPORT_NAME=createUtauTTSOpenJTalk',
    '-sENVIRONMENT=web,worker,node',
    '-sFORCE_FILESYSTEM=1',
    '-sALLOW_MEMORY_GROWTH=1',
    '-sSTACK_SIZE=8MB',
    '-sEXPORTED_RUNTIME_METHODS=ccall,cwrap,FS,HEAPF64,HEAPU8,UTF8ToString,stringToUTF8,lengthBytesUTF8',
    "-sEXPORTED_FUNCTIONS=$exported",
    '-o', (Join-Path $outDir 'utautts-openjtalk.js')
)

& em++ @arguments
if ($LASTEXITCODE -ne 0) { throw "em++ link failed with exit code $LASTEXITCODE" }

# 3) Stage the dictionary for the browser and write a manifest listing its files.
if ([string]::IsNullOrWhiteSpace($DictionaryPath)) {
    $DictionaryPath = Join-Path $root '.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11'
}
$dictOut = Join-Path $outDir 'dict'
if (-not (Test-Path $DictionaryPath)) {
    Write-Warning "dictionary not found, skipping staging: $DictionaryPath"
} else {
    New-Item -ItemType Directory -Force -Path $dictOut | Out-Null
    $names = @()
    foreach ($file in Get-ChildItem -LiteralPath $DictionaryPath -File) {
        $destination = Join-Path $dictOut $file.Name
        if (-not (Test-Path $destination) -or (Get-Item $destination).Length -ne $file.Length) {
            Copy-Item -LiteralPath $file.FullName -Destination $destination -Force
        }
        $names += $file.Name
    }
    $manifestJson = [ordered]@{ files = $names } | ConvertTo-Json
    [IO.File]::WriteAllText((Join-Path $outDir 'dict-manifest.json'), $manifestJson, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host "Staged dictionary: $($names.Count) files"
}
Write-Host "Built $outDir"

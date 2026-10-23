param(
    [string]$OutputDirectory = (Join-Path (Split-Path $PSScriptRoot -Parent) 'runtime'),
    [string]$QtMinGWRoot = $env:QT_MINGW_ROOT,
    [string]$QtToolsRoot = $env:QT_TOOLS_ROOT,
    [string]$CMakeCommand = $env:CMAKE,
    [string]$NinjaCommand = $env:NINJA
)

$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$toolsRoot = if ([string]::IsNullOrWhiteSpace($QtToolsRoot)) { Join-Path $root '.qt/Tools' } else { $QtToolsRoot }
$toolsRoot = [IO.Path]::GetFullPath($toolsRoot)
if ([string]::IsNullOrWhiteSpace($QtMinGWRoot)) {
    $compilers = @(Get-ChildItem -LiteralPath $toolsRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'bin/g++.exe') -PathType Leaf } |
        Sort-Object Name -Descending)
    if ($compilers.Count -gt 0) { $QtMinGWRoot = $compilers[0].FullName }
}
if ([string]::IsNullOrWhiteSpace($QtMinGWRoot)) { throw "Qt MinGW compiler was not found below $toolsRoot" }
$toolchain = Join-Path ([IO.Path]::GetFullPath($QtMinGWRoot)) 'bin'
$compiler = Join-Path $toolchain 'g++.exe'
$cCompiler = Join-Path $toolchain 'gcc.exe'

function Resolve-BuildTool([string]$Configured, [string]$Bundled, [string]$Name) {
    if (-not [string]::IsNullOrWhiteSpace($Configured)) {
        if (Test-Path -LiteralPath $Configured -PathType Leaf) { return [IO.Path]::GetFullPath($Configured) }
        $resolved = Get-Command $Configured -ErrorAction SilentlyContinue
        if ($null -ne $resolved) { return $(if ($resolved.Path) { $resolved.Path } else { $resolved.Source }) }
        throw "$Name was not found: $Configured"
    }
    if (Test-Path -LiteralPath $Bundled -PathType Leaf) { return [IO.Path]::GetFullPath($Bundled) }
    $resolved = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -ne $resolved) { return $(if ($resolved.Path) { $resolved.Path } else { $resolved.Source }) }
    throw "$Name was not found"
}

$cmake = Resolve-BuildTool $CMakeCommand (Join-Path $toolsRoot 'CMake_64/bin/cmake.exe') 'cmake'
$ninja = Resolve-BuildTool $NinjaCommand (Join-Path $toolsRoot 'Ninja/ninja.exe') 'ninja'
foreach ($path in @($cmake, $ninja, $compiler)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Required native build tool was not found: $path"
    }
}

$env:Path = "$toolchain;$env:Path"
$buildDirectory = Join-Path $root 'build/native-windows'
& $cmake -S (Join-Path $root 'native') -B $buildDirectory -G Ninja `
    -DCMAKE_BUILD_TYPE=Release "-DCMAKE_MAKE_PROGRAM=$ninja" "-DCMAKE_CXX_COMPILER=$compiler" "-DCMAKE_C_COMPILER=$cCompiler"
if ($LASTEXITCODE -ne 0) { throw "Native engine configure failed with exit code $LASTEXITCODE" }
& $cmake --build $buildDirectory --config Release
if ($LASTEXITCODE -ne 0) { throw "Native engine build failed with exit code $LASTEXITCODE" }

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
Copy-Item -LiteralPath (Join-Path $buildDirectory 'world-engine/utautts-world-engine.dll') -Destination $OutputDirectory -Force
Copy-Item -LiteralPath (Join-Path $buildDirectory 'openjtalk-engine/utautts-openjtalk-engine.dll') -Destination $OutputDirectory -Force

$dictionaryName = 'open_jtalk_dic_utf_8-1.11'
$dictionaryPath = Join-Path $OutputDirectory $dictionaryName
if (-not (Test-Path -LiteralPath (Join-Path $dictionaryPath 'sys.dic') -PathType Leaf)) {
    $archive = Join-Path $buildDirectory "$dictionaryName.tar.gz"
    Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/r9y9/open_jtalk/releases/download/v1.11.1/$dictionaryName.tar.gz" -OutFile $archive
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
    if ($hash -ne 'fe6ba0e43542cef98339abdffd903e062008ea170b04e7e2a35da805902f382a') {
        throw "Open JTalk dictionary checksum mismatch: $hash"
    }
    & (Join-Path $env:SystemRoot 'System32/tar.exe') -xzf $archive -C $OutputDirectory
    if ($LASTEXITCODE -ne 0) { throw "Extracting the Open JTalk dictionary failed with exit code $LASTEXITCODE" }
}

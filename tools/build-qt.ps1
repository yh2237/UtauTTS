param(
    [string]$QtRoot = $env:QT_ROOT,
    [string]$OutputDirectory = "",
    [string]$Msys2Root = $env:MSYS2_ROOT,
    [string]$QtMinGWRoot = $env:QT_MINGW_ROOT,
    [string]$QtToolsRoot = $env:QT_TOOLS_ROOT,
    [string]$CMakeCommand = $env:CMAKE,
    [string]$NinjaCommand = $env:NINJA
)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$standaloneDevelopmentPackage = [string]::IsNullOrWhiteSpace($OutputDirectory)
$nativeDir = Join-Path $root 'build/native'
$qtBuildDir = Join-Path $root 'build/qt'
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) { $OutputDirectory = Join-Path $root 'build/qt-package' }
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
$appDirectory = Join-Path $OutputDirectory 'app'

if ([string]::IsNullOrWhiteSpace($QtRoot)) {
    $localQtDirectory = Join-Path $root '.qt'
    if (Test-Path -LiteralPath $localQtDirectory -PathType Container) {
        $localKits = @(Get-ChildItem -LiteralPath $localQtDirectory -Directory | ForEach-Object {
            $kit = Join-Path $_.FullName 'mingw_64'
            if (Test-Path -LiteralPath (Join-Path $kit 'lib/cmake/Qt6/Qt6Config.cmake') -PathType Leaf) {
                Get-Item -LiteralPath $kit
            }
        } | Sort-Object { [version]$_.Parent.Name } -Descending)
        if ($localKits.Count -gt 0) { $QtRoot = $localKits[0].FullName }
    }
}
if ([string]::IsNullOrWhiteSpace($QtRoot)) {
    throw 'Qt 6.5+ SDK was not found. Install Qt Quick, Qt Multimedia, and Qt Concurrent, then set QT_ROOT to the compiler kit directory (for example C:\Qt\6.8.3\mingw_64), or install it under .qt/<version>/mingw_64.'
}
$QtRoot = [IO.Path]::GetFullPath($QtRoot)
$qtConfig = Join-Path $QtRoot 'lib/cmake/Qt6/Qt6Config.cmake'
$deployTool = Join-Path $QtRoot 'bin/windeployqt.exe'
if (-not (Test-Path -LiteralPath $qtConfig -PathType Leaf)) { throw "Qt6Config.cmake was not found under $QtRoot" }
if (-not (Test-Path -LiteralPath $deployTool -PathType Leaf)) { throw "windeployqt.exe was not found under $QtRoot" }

$toolsRoot = $QtToolsRoot
if ([string]::IsNullOrWhiteSpace($toolsRoot)) {
    $toolsRoot = Join-Path $QtRoot '../../Tools'
}
$toolsRoot = [IO.Path]::GetFullPath($toolsRoot)

if ([string]::IsNullOrWhiteSpace($QtMinGWRoot)) {
    $defaultCompilerRoot = Join-Path $toolsRoot 'mingw1310_64'
    if (Test-Path -LiteralPath (Join-Path $defaultCompilerRoot 'bin/gcc.exe') -PathType Leaf) {
        $QtMinGWRoot = $defaultCompilerRoot
    } else {
        $compilerCandidates = @(Get-ChildItem -LiteralPath $toolsRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'bin/gcc.exe') -PathType Leaf } |
            Sort-Object Name -Descending)
        if ($compilerCandidates.Count -gt 0) {
            $QtMinGWRoot = $compilerCandidates[0].FullName
        }
    }
}
if ([string]::IsNullOrWhiteSpace($QtMinGWRoot)) {
    throw "Qt MinGW compiler was not found below $toolsRoot. Set QT_MINGW_ROOT to the compiler directory."
}
$QtMinGWRoot = [IO.Path]::GetFullPath($QtMinGWRoot)
$compilerDirectory = Join-Path $QtMinGWRoot 'bin'
$cc = Join-Path $compilerDirectory 'gcc.exe'
$cxx = Join-Path $compilerDirectory 'g++.exe'
$windres = Join-Path $compilerDirectory 'windres.exe'
$msys2RootValue = $Msys2Root
if ([string]::IsNullOrWhiteSpace($msys2RootValue)) { $msys2RootValue = 'C:\msys64' }
$msys2RootValue = [IO.Path]::GetFullPath($msys2RootValue)
$goCC = Join-Path $msys2RootValue 'mingw64/bin/clang.exe'
$goCXX = Join-Path $msys2RootValue 'mingw64/bin/clang++.exe'
$gendef = Join-Path $compilerDirectory 'gendef.exe'
$dlltool = Join-Path $compilerDirectory 'dlltool.exe'

function Resolve-BuildTool([string]$Configured, [string]$BundledPath, [string]$CommandName) {
    if (-not [string]::IsNullOrWhiteSpace($Configured)) {
        if (Test-Path -LiteralPath $Configured -PathType Leaf) {
            return [IO.Path]::GetFullPath($Configured)
        }
        $resolved = Get-Command $Configured -ErrorAction SilentlyContinue
        if ($null -ne $resolved) {
            if ($resolved.Path) { return $resolved.Path }
            return $resolved.Source
        }
        throw "Configured $CommandName was not found: $Configured"
    }
    if (Test-Path -LiteralPath $BundledPath -PathType Leaf) {
        return [IO.Path]::GetFullPath($BundledPath)
    }
    $resolved = Get-Command $CommandName -ErrorAction SilentlyContinue
    if ($null -ne $resolved) {
        if ($resolved.Path) { return $resolved.Path }
        return $resolved.Source
    }
    throw "$CommandName was not found. Install it or set the corresponding environment variable."
}

$cmake = Resolve-BuildTool $CMakeCommand (Join-Path $toolsRoot 'CMake_64/bin/cmake.exe') 'cmake'
$ninja = Resolve-BuildTool $NinjaCommand (Join-Path $toolsRoot 'Ninja/ninja.exe') 'ninja'
foreach ($tool in @($cc,$cxx,$windres,$goCC,$goCXX,$gendef,$dlltool,$cmake,$ninja)) { if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) { throw "Required Qt build tool was not found: $tool" } }

New-Item -ItemType Directory -Force -Path $nativeDir, $qtBuildDir, $OutputDirectory | Out-Null
if (Test-Path -LiteralPath $appDirectory) { Remove-Item -LiteralPath $appDirectory -Recurse -Force }
New-Item -ItemType Directory -Force -Path $appDirectory | Out-Null
$previousCgo = $env:CGO_ENABLED; $previousCC=$env:CC; $previousCXX=$env:CXX; $previousPath=$env:Path; $previousGoCache=$env:GOCACHE
try {
    $env:CGO_ENABLED='1';$env:CC=$goCC;$env:CXX=$goCXX;$env:GOCACHE=Join-Path $root 'build\go-cache-qt-cgo';$env:Path=(Split-Path $goCC -Parent)+';'+$env:Path
    Push-Location $root
    try { & go build -buildmode=c-shared -o (Join-Path $nativeDir 'utautts_native.dll') ./cmd/utautts-native; if ($LASTEXITCODE -ne 0) { throw 'Go native library build failed' } }
    finally { Pop-Location }
    Push-Location $nativeDir
    try {
        & $gendef 'utautts_native.dll'; if ($LASTEXITCODE -ne 0) { throw 'gendef failed' }
        $nativeDefinition = Get-Content -LiteralPath 'utautts_native.def' -Raw
        $requiredExports = @('UtauTTSCreate','UtauTTSLastError','UtauTTSCall','UtauTTSDestroy','UtauTTSFree')
        $missingExports = @($requiredExports | Where-Object { $nativeDefinition -notmatch "(?m)^$([regex]::Escape($_))\s*$" })
        if ($missingExports.Count -gt 0) {
            throw "Go native DLL is missing C exports: $($missingExports -join ', '). Keep each //export directive directly above its Go function."
        }
        & $dlltool -d 'utautts_native.def' -l 'utautts_native.dll.a' -D 'utautts_native.dll'; if ($LASTEXITCODE -ne 0) { throw 'dlltool failed' }
    } finally { Pop-Location }
} finally { $env:CGO_ENABLED=$previousCgo;$env:CC=$previousCC;$env:CXX=$previousCXX;$env:GOCACHE=$previousGoCache;$env:Path=$previousPath }

$ninjaDirectory = Split-Path -Parent $ninja
$env:Path = $ninjaDirectory + ';' + $compilerDirectory + ';' + $env:Path
& $cmake -S (Join-Path $root 'qt') -B $qtBuildDir -G Ninja "-DCMAKE_PREFIX_PATH=$QtRoot" "-DUTAUTTS_NATIVE_DIR=$nativeDir" "-DCMAKE_C_COMPILER=$cc" "-DCMAKE_CXX_COMPILER=$cxx" -DCMAKE_BUILD_TYPE=Release
if ($LASTEXITCODE -ne 0) { throw 'Qt CMake configure failed' }
& $cmake --build $qtBuildDir --config Release
if ($LASTEXITCODE -ne 0) { throw 'Qt build failed' }
$executable = Get-ChildItem -LiteralPath $qtBuildDir -Recurse -Filter 'utautts.exe' -File | Select-Object -First 1
if ($null -eq $executable) { throw 'Qt executable was not produced' }
Copy-Item -LiteralPath $executable.FullName -Destination (Join-Path $appDirectory 'utautts-gui.exe') -Force
Copy-Item -LiteralPath (Join-Path $nativeDir 'utautts_native.dll') -Destination $appDirectory -Force
& $deployTool --release --qmldir (Join-Path $root 'qt/qml') --no-system-d3d-compiler `
    --no-system-dxc-compiler --no-opengl-sw (Join-Path $appDirectory 'utautts-gui.exe')
if ($LASTEXITCODE -ne 0) { throw 'windeployqt failed' }
$previousLauncherGoCache = $env:GOCACHE
$env:GOCACHE = Join-Path $root 'build\go-cache'
$launcherDirectory = Join-Path $root 'cmd/utautts-launcher'
$launcherIcon = Join-Path $launcherDirectory 'utautts.ico'
$launcherResource = Join-Path $launcherDirectory 'utautts.syso'
Copy-Item -LiteralPath (Join-Path $root 'icons/utautts.ico') -Destination $launcherIcon -Force
Push-Location $launcherDirectory
try {
    & $windres 'utautts.rc' '--output-format=coff' '--target=pe-x86-64' '--output=utautts.syso'
    if ($LASTEXITCODE -ne 0) { throw 'Windows launcher icon resource generation failed' }
} finally {
    Pop-Location
}
$guiToolsDirectory = Join-Path $OutputDirectory 'tools'
New-Item -ItemType Directory -Force -Path $guiToolsDirectory | Out-Null
Push-Location $root
try {
    & go build -trimpath -ldflags '-H windowsgui' -o (Join-Path $OutputDirectory 'utautts.exe') ./cmd/utautts-launcher
    if ($LASTEXITCODE -ne 0) { throw 'Qt launcher build failed' }
    & go build -trimpath -ldflags '-H windowsgui' -o (Join-Path $guiToolsDirectory 'utautts-updater.exe') ./cmd/utautts-updater
    if ($LASTEXITCODE -ne 0) { throw 'Qt updater build failed' }
} finally {
    Pop-Location
    $env:GOCACHE = $previousLauncherGoCache
    Remove-Item -LiteralPath $launcherIcon, $launcherResource -Force -ErrorAction SilentlyContinue
}
$staleDeployDirectories = @('generic','iconengines','imageformats','multimedia','networkinformation','platforms','qml','qmltooling','tls','translations')
foreach ($name in $staleDeployDirectories) {
	$stalePath = Join-Path $OutputDirectory $name
	if (Test-Path -LiteralPath $stalePath -PathType Container) { Remove-Item -LiteralPath $stalePath -Recurse -Force }
}
$appQmlToolingPath = Join-Path $appDirectory 'qmltooling'
if (Test-Path -LiteralPath $appQmlToolingPath -PathType Container) { Remove-Item -LiteralPath $appQmlToolingPath -Recurse -Force }
Get-ChildItem -LiteralPath $OutputDirectory -Filter '*.dll' -File | Remove-Item -Force
if ($standaloneDevelopmentPackage) {
    foreach ($name in @('models','plugins','runtime','voice')) {
        $assetPath = Join-Path $OutputDirectory $name
        if (Test-Path -LiteralPath $assetPath) { Remove-Item -LiteralPath $assetPath -Recurse -Force }
    }
    Copy-Item -LiteralPath (Join-Path $root 'models') -Destination (Join-Path $OutputDirectory 'models') -Recurse
    New-Item -ItemType Directory -Force -Path (Join-Path $OutputDirectory 'plugins') | Out-Null
    Copy-Item -LiteralPath (Join-Path $root 'plugins/renderers') -Destination (Join-Path $OutputDirectory 'plugins') -Recurse
    $runtimeCandidates = @(
        (Join-Path $root 'runtime'),
        (Join-Path $root 'release/UtauTTS/runtime')
    )
    $runtimeSource = $runtimeCandidates | Where-Object {
        Test-Path -LiteralPath (Join-Path $_ 'utautts-openjtalk-features.exe') -PathType Leaf
    } | Select-Object -First 1
    if ($runtimeSource) {
        Copy-Item -LiteralPath $runtimeSource -Destination (Join-Path $OutputDirectory 'runtime') -Recurse
    } else {
        Write-Warning 'Runtime assets were not found. Run build.bat once to prepare Open JTalk and WORLDLINE assets.'
    }
    $voiceDirectory = Join-Path $OutputDirectory 'voice'
    New-Item -ItemType Directory -Force -Path $voiceDirectory | Out-Null
    $voiceArchives = @(Get-ChildItem -LiteralPath (Join-Path $root 'voice') -Filter '*.zip' -File)
    foreach ($archive in $voiceArchives) {
        Expand-Archive -LiteralPath $archive.FullName -DestinationPath $voiceDirectory
    }
}
Write-Host "Built Qt package at $OutputDirectory"

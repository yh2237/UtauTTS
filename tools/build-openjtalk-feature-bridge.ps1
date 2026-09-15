param(
    [string]$PackageRoot = (Join-Path $PSScriptRoot '..\.tmp-openjtalk'),
    [string]$PyInstallerRoot = (Join-Path $PSScriptRoot '..\.tmp-pyinstaller'),
    [string]$Python = $env:PYTHON
)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$pythonCommand = $Python
if ([string]::IsNullOrWhiteSpace($pythonCommand)) { $pythonCommand = 'python' }
$packageRoot = [IO.Path]::GetFullPath($PackageRoot)
$pyInstallerRoot = [IO.Path]::GetFullPath($PyInstallerRoot)
$microsoftRuntimeStage = Join-Path $root '.tmp-openjtalk-ms-runtime'
$microsoftRuntimeSourceManifest = Join-Path $microsoftRuntimeStage 'source-manifest.json'

function Resolve-MsvcRedistDirectory {
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($env:UTAUTTS_MSVC_REDIST_DIR)) {
        $configuredRoot = [IO.Path]::GetFullPath($env:UTAUTTS_MSVC_REDIST_DIR)
        $candidates += @(
            $configuredRoot
            (Join-Path $configuredRoot 'x64/Microsoft.VC143.CRT')
            (Join-Path $configuredRoot 'Microsoft.VC143.CRT')
        )
    }
    if (-not [string]::IsNullOrWhiteSpace($env:VCToolsRedistDir)) {
        $candidates += @(
            $env:VCToolsRedistDir
            (Join-Path $env:VCToolsRedistDir 'x64/Microsoft.VC143.CRT')
        )
    }
    $visualStudioRoots = @(
        @(${env:ProgramFiles}, ${env:ProgramFiles(x86)}) |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
            ForEach-Object { Join-Path $_ 'Microsoft Visual Studio' }
    )
    $visualStudioRoots = @($visualStudioRoots | Select-Object -Unique)
    foreach ($visualStudioRoot in $visualStudioRoots) {
        if (-not (Test-Path -LiteralPath $visualStudioRoot -PathType Container)) { continue }
        $autoCandidates = @(Get-ChildItem -LiteralPath $visualStudioRoot -Directory -ErrorAction SilentlyContinue |
            ForEach-Object {
                Get-ChildItem -LiteralPath $_.FullName -Directory -ErrorAction SilentlyContinue |
                    ForEach-Object {
                        $redistRoot = Join-Path $_.FullName 'VC/Redist/MSVC'
                        if (-not (Test-Path -LiteralPath $redistRoot -PathType Container)) { return }
                        Get-ChildItem -LiteralPath $redistRoot -Directory -ErrorAction SilentlyContinue |
                            ForEach-Object {
                                $x64Root = Join-Path $_.FullName 'x64'
                                if (-not (Test-Path -LiteralPath $x64Root -PathType Container)) { return }
                                Get-ChildItem -LiteralPath $x64Root -Directory -ErrorAction SilentlyContinue |
                                    Where-Object { $_.Name -match '^Microsoft\.VC\d+\.CRT$' } |
                                    Select-Object -ExpandProperty FullName
                            }
                    }
            } | Sort-Object -Descending)
        $candidates += $autoCandidates
    }
    $resolved = @($candidates |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace($_) -and
            (Test-Path -LiteralPath (Join-Path $_ 'msvcp140.dll') -PathType Leaf) -and
            (Test-Path -LiteralPath (Join-Path $_ 'vcruntime140.dll') -PathType Leaf) -and
            (Test-Path -LiteralPath (Join-Path $_ 'vcruntime140_1.dll') -PathType Leaf)
        } |
        Select-Object -Unique)
    if ($resolved.Count -eq 0) {
        throw 'Microsoft Visual C++ x64 redist directory was not found. Set UTAUTTS_MSVC_REDIST_DIR to a Microsoft.VC*.CRT directory.'
    }
    return $resolved[0]
}

function Resolve-UcrtRedistDirectory {
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($env:UTAUTTS_UCRT_REDIST_DIR)) {
        $candidates += [IO.Path]::GetFullPath($env:UTAUTTS_UCRT_REDIST_DIR)
    }
    if (-not [string]::IsNullOrWhiteSpace($env:WindowsSdkDir)) {
        $sdkRedistRoot = Join-Path $env:WindowsSdkDir 'Redist'
        if (Test-Path -LiteralPath $sdkRedistRoot -PathType Container) {
            $candidates += @(Get-ChildItem -LiteralPath $sdkRedistRoot -Directory -ErrorAction SilentlyContinue |
                Where-Object { $_.Name -match '^10\.' } |
                Sort-Object Name -Descending |
                ForEach-Object { Join-Path $_.FullName 'ucrt/DLLs/x64' })
        }
        $candidates += Join-Path $env:WindowsSdkDir 'Redist/ucrt/DLLs/x64'
    }
    $kitsRedistRoot = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits/10/Redist'
    if (Test-Path -LiteralPath $kitsRedistRoot -PathType Container) {
        $candidates += @(Get-ChildItem -LiteralPath $kitsRedistRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match '^10\.' } |
            Sort-Object Name -Descending |
            ForEach-Object { Join-Path $_.FullName 'ucrt/DLLs/x64' })
        $candidates += Join-Path $kitsRedistRoot 'ucrt/DLLs/x64'
    }
    $resolved = @($candidates |
        Where-Object {
            (Test-Path -LiteralPath (Join-Path $_ 'ucrtbase.dll') -PathType Leaf) -and
            @(Get-ChildItem -LiteralPath $_ -Filter 'api-ms-win-*.dll' -File -ErrorAction SilentlyContinue).Count -gt 0
        } |
        Select-Object -Unique)
    if ($resolved.Count -eq 0) {
        throw 'Windows SDK UCRT redist directory was not found. Set UTAUTTS_UCRT_REDIST_DIR to Windows Kits/10/Redist/ucrt/DLLs/x64.'
    }
    return $resolved[0]
}

function Get-SourceFileRecord([IO.FileInfo]$File, [string]$Family, [string]$Package, [string]$Version) {
    return [ordered]@{
        name = $File.Name
        family = $Family
        package = $Package
        version = $Version
        size = $File.Length
        sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $File.FullName).Hash.ToUpperInvariant()
        file_version = $File.VersionInfo.FileVersion
    }
}

$msvcRedistDirectory = Resolve-MsvcRedistDirectory
$ucrtRedistDirectory = Resolve-UcrtRedistDirectory
$msvcPackage = Split-Path -Leaf $msvcRedistDirectory
$msvcVersion = Split-Path -Leaf (Split-Path -Parent (Split-Path -Parent $msvcRedistDirectory))
$ucrtVersion = (Get-Item -LiteralPath (Join-Path $ucrtRedistDirectory 'ucrtbase.dll')).VersionInfo.FileVersion
$microsoftRuntimeSourceFiles = @()
$microsoftRuntimeFiles = @(
    Get-ChildItem -LiteralPath $msvcRedistDirectory -File |
        Where-Object { $_.Name -match '^msvcp140(?:_\d+)?\.dll$|^vcruntime140(?:_\d+)?\.dll$' }
    Get-ChildItem -LiteralPath $ucrtRedistDirectory -File |
        Where-Object { $_.Name -match '^ucrtbase\.dll$|^api-ms-win-(?:core|crt)-[a-z0-9-]+\.dll$' }
)
if ($microsoftRuntimeFiles.Count -eq 0) {
    throw 'No Microsoft runtime files were found in the official redist directories.'
}
if (Test-Path -LiteralPath $microsoftRuntimeStage) {
    Remove-Item -Recurse -Force -LiteralPath $microsoftRuntimeStage
}
New-Item -ItemType Directory -Force -Path $microsoftRuntimeStage | Out-Null
foreach ($file in $microsoftRuntimeFiles) {
    Copy-Item -LiteralPath $file.FullName -Destination (Join-Path $microsoftRuntimeStage $file.Name) -Force
    $family = if ($file.DirectoryName -eq $msvcRedistDirectory) {
        'Microsoft Visual C++ Redistributable'
    } else {
        'Windows SDK UCRT Redist'
    }
    $package = if ($family -eq 'Microsoft Visual C++ Redistributable') {
        $msvcPackage
    } else {
        'Windows Kits 10 UCRT/DLLs/x64'
    }
    $version = if ($family -eq 'Microsoft Visual C++ Redistributable') { $msvcVersion } else { $ucrtVersion }
    $microsoftRuntimeSourceFiles += Get-SourceFileRecord $file $family $package $version
}
$microsoftRuntimeSourceManifestObject = [ordered]@{
    format_version = 1
    kind = 'microsoft-runtime-source-manifest'
    architecture = 'x64'
    sources = [ordered]@{
        visual_cpp_redist = [ordered]@{
            family = 'Microsoft Visual C++ Redistributable'
            package = $msvcPackage
            version = $msvcVersion
            guidance = 'https://learn.microsoft.com/en-us/visualstudio/releases/2022/redistribution'
        }
        windows_sdk_ucrt = [ordered]@{
            family = 'Windows SDK UCRT Redist'
            package = 'Windows Kits 10 UCRT/DLLs/x64'
            version = $ucrtVersion
            guidance = 'https://learn.microsoft.com/en-us/cpp/windows/universal-crt-deployment?view=msvc-170'
        }
    }
    files = @($microsoftRuntimeSourceFiles)
}
$microsoftRuntimeSourceManifestObject | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $microsoftRuntimeSourceManifest -Encoding UTF8
$pythonPackage = Join-Path $packageRoot 'pyopenjtalk'
if (-not (Test-Path -LiteralPath $pythonPackage)) {
    & $pythonCommand -m pip install --target $packageRoot 'pyopenjtalk==0.4.1'
    if ($LASTEXITCODE -ne 0) { throw "Installing pyopenjtalk failed with exit code $LASTEXITCODE" }
}
$dictionaryPath = Join-Path $pythonPackage 'open_jtalk_dic_utf_8-1.11'
if (-not (Test-Path -LiteralPath $dictionaryPath)) {
    $previousPythonPath = $env:PYTHONPATH
    try {
        $env:PYTHONPATH = $packageRoot
        & $pythonCommand -c "import pyopenjtalk; pyopenjtalk.run_frontend('テスト')"
        if ($LASTEXITCODE -ne 0) { throw "Downloading the Open JTalk dictionary failed with exit code $LASTEXITCODE" }
    } finally {
        $env:PYTHONPATH = $previousPythonPath
    }
}
if (-not (Test-Path -LiteralPath (Join-Path $pyInstallerRoot 'PyInstaller'))) {
    & $pythonCommand -m pip install --target $pyInstallerRoot 'pyinstaller==6.16.0'
    if ($LASTEXITCODE -ne 0) { throw "Installing PyInstaller failed with exit code $LASTEXITCODE" }
}
$extension = @(Get-ChildItem -LiteralPath $pythonPackage -Filter 'openjtalk.cp*-win_amd64.pyd' -File)
if ($extension.Count -ne 1) {
    throw "Expected one pyopenjtalk 0.4.1 extension matching the active Python in $packageRoot"
}
$inputPath = Join-Path $root '.tmp-openjtalk-bridge-input'
$workPath = Join-Path $root '.tmp-openjtalk-bridge-build'
$specPath = Join-Path $root '.tmp-openjtalk-bridge-spec'
$distPath = Join-Path $root 'tools/openjtalk-feature-bridge/bin'
foreach ($path in @($inputPath, $workPath, $specPath, $distPath)) {
    if (-not $path.StartsWith($root + [IO.Path]::DirectorySeparatorChar)) {
        throw "Unsafe bridge output path: $path"
    }
    if (Test-Path -LiteralPath $path) {
        Remove-Item -Recurse -Force -LiteralPath $path
    }
    New-Item -ItemType Directory -Force -Path $path | Out-Null
}
Copy-Item -LiteralPath $extension[0].FullName -Destination (Join-Path $inputPath 'openjtalk.pyd')
$runtimeBinaryArguments = @()
foreach ($runtimeFile in Get-ChildItem -LiteralPath $microsoftRuntimeStage -File |
    Where-Object { $_.Name -match '^msvcp140(?:_\d+)?\.dll$|^vcruntime140(?:_\d+)?\.dll$|^ucrtbase\.dll$|^api-ms-win-(?:core|crt)-[a-z0-9-]+\.dll$' }) {
    $runtimeBinaryArguments += @('--add-binary', "$($runtimeFile.FullName);.")
}
$previousPythonPath = $env:PYTHONPATH
$previousPath = $env:Path
try {
    $env:PYTHONPATH = $pyInstallerRoot
    $env:Path = $microsoftRuntimeStage + [IO.Path]::PathSeparator + $previousPath
    & $pythonCommand -m PyInstaller --noconfirm --clean --onefile `
        --name utautts-openjtalk-features `
        --exclude-module _hashlib `
        --paths $inputPath `
        --hidden-import openjtalk `
        @runtimeBinaryArguments `
        --distpath $distPath `
        --workpath $workPath `
        --specpath $specPath `
        (Join-Path $root 'tools/openjtalk-feature-bridge.py')
    if ($LASTEXITCODE -ne 0) {
        throw "PyInstaller failed with exit code $LASTEXITCODE"
    }
} finally {
    $env:PYTHONPATH = $previousPythonPath
    $env:Path = $previousPath
}
$helperPath = Join-Path $distPath 'utautts-openjtalk-features.exe'
$verificationCorpus = Join-Path $root 'out/prosody/openjtalk-accent-features-v1.json'
& $pythonCommand (Join-Path $root 'tools/verify-openjtalk-feature-bridge.py') `
    --helper $helperPath --dictionary $dictionaryPath --corpus $verificationCorpus
if ($LASTEXITCODE -ne 0) { throw "Open JTalk helper verification failed with exit code $LASTEXITCODE" }
Get-Item -LiteralPath $helperPath

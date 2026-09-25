param(
    [string]$Python = $env:PYTHON,
    [ValidateSet('Full', 'Japanese')]
    [string]$Profile = 'Full',
    [string]$OutputDirectory = '',
    [string]$ExpectedVersion = $env:UTAUTTS_RELEASE_VERSION,
    [string]$PreviousVersion = $env:UTAUTTS_PREVIOUS_VERSION,
    [int]$PreviousUpdateSchema = 0,
    [int]$PreviousInstallLayout = 0,
    [int]$PreviousMigrationSchema = 0,
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem

$stepStopwatch = [System.Diagnostics.Stopwatch]::StartNew()
function Write-Step([string]$Name) {
    Write-Host ("=== {0} === (+{1:N1}s)" -f $Name, $stepStopwatch.Elapsed.TotalSeconds)
    $stepStopwatch.Restart()
}
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$pythonCommand = $Python
if ([string]::IsNullOrWhiteSpace($pythonCommand)) { $pythonCommand = 'python' }
$releaseCheck = Join-Path $PSScriptRoot 'check-release.ps1'
& $releaseCheck `
    -ExpectedVersion $ExpectedVersion `
    -PreviousVersion $PreviousVersion `
    -PreviousUpdateSchema $PreviousUpdateSchema `
    -PreviousInstallLayout $PreviousInstallLayout `
    -PreviousMigrationSchema $PreviousMigrationSchema
$releaseRoot = Join-Path $root 'release'
if (-not [string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $releaseRoot = [IO.Path]::GetFullPath($OutputDirectory)
    if (-not $releaseRoot.StartsWith($root + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Release output must be a subdirectory of the project'
    }
}
$guiPath = Join-Path $releaseRoot 'UtauTTS'
$serverPath = Join-Path $releaseRoot 'UtauTTS-Server'
$guiToolsPath = Join-Path $guiPath 'tools'
$guiRuntimePath = Join-Path $guiPath 'runtime'
$guiModelsPath = Join-Path $guiPath 'models'
$guiRendererPath = Join-Path $guiPath 'renderer'
$serverRuntimePath = Join-Path $serverPath 'runtime'
$serverModelsPath = Join-Path $serverPath 'models'
$serverRendererPath = Join-Path $serverPath 'renderer'
$guiZip = Join-Path $releaseRoot 'UtauTTS-win-x64.zip'
$serverZip = Join-Path $releaseRoot 'UtauTTS-Server-win-x64.zip'
$bundledVoicebankDirectory = Join-Path $root 'voice'
$bundledVoicebankSHA256 = 'B96D1B21145F22E573AFD9EC8AEAAD0EC9CBAEE581C2623C64ADDEB31DE46B3D'

function Invoke-Checked([string]$Command, [string[]]$Arguments) {
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command failed with exit code $LASTEXITCODE"
    }
}

function Reset-Directory([string]$Path) {
    if (-not $Path.StartsWith($releaseRoot + [IO.Path]::DirectorySeparatorChar)) {
        throw "Unsafe output path: $Path"
    }
    if (Test-Path -LiteralPath $Path) {
        Remove-Item -Recurse -Force -LiteralPath $Path
    }
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Compress-Package([string]$SourceDirectory, [string]$DestinationZip) {
    if (Test-Path -LiteralPath $DestinationZip) {
        Remove-Item -Force -LiteralPath $DestinationZip
    }
    # ZipFile.CreateFromDirectory is markedly faster than Compress-Archive for
    # the large GUI tree and produces the same archive layout.
    [System.IO.Compression.ZipFile]::CreateFromDirectory(
        $SourceDirectory, $DestinationZip,
        [System.IO.Compression.CompressionLevel]::Optimal, $false)
}

function Expand-BundledVoicebank([string]$Destination) {
    $archives = @(Get-ChildItem -LiteralPath $bundledVoicebankDirectory -Filter '*.zip' -File)
    if ($archives.Count -ne 1) {
        throw "Expected exactly one bundled voicebank archive, found $($archives.Count) in $bundledVoicebankDirectory"
    }
    $bundledVoicebankArchive = $archives[0].FullName
    $actualHash = (Get-FileHash -LiteralPath $bundledVoicebankArchive -Algorithm SHA256).Hash.ToUpperInvariant()
    if ($actualHash -ne $bundledVoicebankSHA256) {
        throw "Bundled voicebank hash mismatch: expected $bundledVoicebankSHA256, got $actualHash"
    }
    # The official voicebank archive contains CP932-encoded Japanese entry
    # names. Expand-Archive decodes those names incorrectly on the hosted
    # Windows runner, so use Python's explicit metadata_encoding support.
    $extractScript = @'
import os
import sys
import zipfile

archive_path, destination = sys.argv[1:3]
os.makedirs(destination, exist_ok=True)
with zipfile.ZipFile(archive_path, metadata_encoding='cp932') as archive:
    archive.extractall(destination)
'@
    & $pythonCommand -c $extractScript $bundledVoicebankArchive $Destination
    if ($LASTEXITCODE -ne 0) {
        throw "Bundled voicebank extraction failed with exit code $LASTEXITCODE"
    }
}

Reset-Directory $guiPath
Reset-Directory $serverPath
New-Item -ItemType Directory -Force -Path $guiToolsPath, $guiRuntimePath, $guiModelsPath, $guiRendererPath, $serverRuntimePath, $serverModelsPath, $serverRendererPath | Out-Null
foreach ($zip in @($guiZip, $serverZip)) {
    if (Test-Path -LiteralPath $zip) {
        Remove-Item -Force -LiteralPath $zip
    }
}

$env:GOCACHE = Join-Path $root 'build\go-cache'
Push-Location $root
try {
    if ($SkipTests) {
        Write-Step 'Test (skipped)'
    } else {
        Write-Step 'Test'
        Invoke-Checked 'go' @('test', './...')
    }

    Write-Step 'Build GUI package'
    & (Join-Path $PSScriptRoot 'build-qt.ps1') -OutputDirectory $guiPath
    if ($LASTEXITCODE -ne 0) { throw "Qt GUI build failed with exit code $LASTEXITCODE" }
    Invoke-Checked 'go' @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $guiToolsPath 'utautts-cli.exe'), './cmd/utautts-cli')
    Invoke-Checked 'go' @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $guiToolsPath 'utautts-ustx.exe'), './cmd/tools/utautts-ustx')

    Write-Step 'Build server package'
    Invoke-Checked 'go' @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $serverPath 'utautts-server.exe'), './cmd/utautts-server')

    Write-Step 'Build Open JTalk frontend helper'
    & (Join-Path $PSScriptRoot 'build-openjtalk-feature-bridge.ps1') -Python $pythonCommand
    if ($LASTEXITCODE -ne 0) {
        throw "Open JTalk frontend helper build failed with exit code $LASTEXITCODE"
    }

    Write-Step 'Build native worldline bridge'
    Invoke-Checked 'go' @(
        'build', '-trimpath', '-o', (Join-Path $guiRuntimePath 'utautts-worldline-bridge.exe'),
        './cmd/utautts-worldline-bridge'
    )
    & (Join-Path $PSScriptRoot 'build-world-engine.ps1') -OutputDirectory $guiRuntimePath
    if ($LASTEXITCODE -ne 0) { throw "UtauTTS WORLD engine build failed with exit code $LASTEXITCODE" }
    if ($Profile -eq 'Full') {
        Write-Step 'Build DiffSinger bridge'
        & (Join-Path $PSScriptRoot 'build-diffsinger-bridge.ps1') -OutputDirectory $guiRuntimePath
        if ($LASTEXITCODE -ne 0) { throw "DiffSinger bridge build failed with exit code $LASTEXITCODE" }
    }

    Copy-Item -Path (Join-Path $guiRuntimePath '*') -Destination $serverRuntimePath -Recurse -Force

    $openJTalkHelper = Join-Path $root 'tools/openjtalk-feature-bridge/bin/utautts-openjtalk-features.exe'
    $openJTalkDictionary = Join-Path $root '.tmp-openjtalk/pyopenjtalk/open_jtalk_dic_utf_8-1.11'
    $microsoftRuntimeSourceManifest = Join-Path $root '.tmp-openjtalk-ms-runtime/source-manifest.json'
    if (-not (Test-Path -LiteralPath $microsoftRuntimeSourceManifest -PathType Leaf)) {
        throw "Microsoft runtime source manifest was not produced: $microsoftRuntimeSourceManifest"
    }
    foreach ($runtimePath in @($guiRuntimePath, $serverRuntimePath)) {
        Copy-Item -LiteralPath $openJTalkHelper -Destination $runtimePath
        Copy-Item -LiteralPath $openJTalkDictionary -Destination $runtimePath -Recurse
        $licensePath = Join-Path $runtimePath 'licenses'
        New-Item -ItemType Directory -Force -Path $licensePath | Out-Null
        $pythonInfo = Get-Command $pythonCommand -ErrorAction Stop
        $pythonExecutable = if ($pythonInfo.Path) { $pythonInfo.Path } else { $pythonInfo.Source }
        $pythonStdlibOutput = @(& $pythonCommand -c "import sysconfig; print(sysconfig.get_path('stdlib'))")
        $pythonExitCode = $LASTEXITCODE
        if ($pythonExitCode -ne 0) { throw 'Could not determine the Python standard-library path' }
        $pythonStdlib = $pythonStdlibOutput | Select-Object -First 1
        if ([string]::IsNullOrWhiteSpace($pythonStdlib) -or
            -not (Test-Path -LiteralPath $pythonStdlib -PathType Container)) {
            throw "Python standard-library path was not found: $pythonStdlib"
        }
        $pythonLicense = @(
            Get-ChildItem -LiteralPath $pythonStdlib -Filter 'LICENSE*' -File -ErrorAction SilentlyContinue
            Get-ChildItem -LiteralPath (Split-Path $pythonStdlib -Parent) -Filter 'LICENSE*' -File -ErrorAction SilentlyContinue
            Get-ChildItem -LiteralPath (Split-Path $pythonExecutable) -Filter 'LICENSE*' -File -ErrorAction SilentlyContinue
            Get-ChildItem -LiteralPath (Split-Path (Split-Path $pythonExecutable)) -Filter 'LICENSE*' -File -ErrorAction SilentlyContinue
        ) | Select-Object -First 1
        if ($null -eq $pythonLicense) { throw "Python license was not found for $pythonExecutable" }
        Copy-Item -LiteralPath $pythonLicense.FullName -Destination (Join-Path $licensePath 'PYTHON_LICENSE.txt')
        $pyInstallerLicense = @(Get-ChildItem -LiteralPath (Join-Path $root '.tmp-pyinstaller') -Recurse -Filter 'COPYING.txt' -File | Where-Object { $_.FullName -like '*pyinstaller-*.dist-info*' })
        if ($pyInstallerLicense.Count -ne 1) { throw 'Expected exactly one PyInstaller COPYING.txt' }
        Copy-Item -LiteralPath $pyInstallerLicense[0].FullName -Destination (Join-Path $licensePath 'PYINSTALLER_COPYING.txt')
        $previousPythonPath = $env:PYTHONPATH
        try {
            $env:PYTHONPATH = Join-Path $root '.tmp-pyinstaller'
            Invoke-Checked $pythonCommand @(
                (Join-Path $root 'tools/collect-pyinstaller-runtime-licenses.py'),
                '--archive', (Join-Path $runtimePath 'utautts-openjtalk-features.exe'),
                '--output-dir', $licensePath,
                '--python-license', $pythonLicense.FullName,
                '--pyinstaller-license', $pyInstallerLicense[0].FullName,
                '--microsoft-runtime-source-manifest', $microsoftRuntimeSourceManifest
            )
        } finally {
            $env:PYTHONPATH = $previousPythonPath
        }
    }

    Copy-Item -LiteralPath 'LICENSE', 'LICENSE-SCOPE.md', 'README.md', 'THIRD_PARTY_NOTICES.txt', 'THIRD_PARTY_NOTICES-WINDOWS-GUI.txt' -Destination $guiPath

    $sourceModels = Join-Path $root 'models'
    $bundledModels = @()
    if (Test-Path -LiteralPath $sourceModels) {
		$bundledModels = @(Get-ChildItem -LiteralPath $sourceModels -Filter '*.json' -File)
    }
    if ($bundledModels.Count -eq 0) {
        throw 'No bundled prosody models found. Install self-describing models into models/ with tools/install-prosody-model.ps1.'
    }
    $modelReadmePath = Join-Path $sourceModels 'README.md'
    if (-not (Test-Path -LiteralPath $modelReadmePath -PathType Leaf)) {
        throw 'models/README.md is required when bundling prosody models.'
    }
    $bundledModels | Copy-Item -Destination $guiModelsPath
    $bundledModels | Copy-Item -Destination $serverModelsPath
    Copy-Item -LiteralPath $modelReadmePath -Destination $guiModelsPath
    Copy-Item -LiteralPath $modelReadmePath -Destination $serverModelsPath
    foreach ($packagePath in @($guiPath, $serverPath)) {
        Invoke-Checked $pythonCommand @(
            (Join-Path $root 'tools/copy-model-license-notices.py'),
            '--models', (Join-Path $packagePath 'models'),
            '--repository-root', $root,
            '--package-root', $packagePath
        )
    }
    Copy-Item -Path (Join-Path $root 'renderer/*') -Destination $guiRendererPath -Recurse
    Copy-Item -Path (Join-Path $root 'renderer/*') -Destination $serverRendererPath -Recurse
    foreach ($directoryName in @('Resamplers', 'Wavtools', 'Dependencies')) {
        Copy-Item -LiteralPath (Join-Path $root $directoryName) -Destination $guiPath -Recurse
        Copy-Item -LiteralPath (Join-Path $root $directoryName) -Destination $serverPath -Recurse
    }
    foreach ($rendererPath in @($guiRendererPath, $serverRendererPath)) {
        if ($Profile -eq 'Japanese') {
            foreach ($optionalRenderer in @('diffsinger')) {
                $optionalRendererPath = Join-Path $rendererPath $optionalRenderer
                if (Test-Path -LiteralPath $optionalRendererPath) {
                    Remove-Item -LiteralPath $optionalRendererPath -Recurse -Force
                }
            }
        }
    }
    $guiDocs = Join-Path $guiPath 'docs'
    New-Item -ItemType Directory -Force -Path $guiDocs | Out-Null
    Copy-Item -Path 'docs/*' -Destination $guiDocs -Recurse

    $guiVoiceDirectory = Join-Path $guiPath 'voice'
    New-Item -ItemType Directory -Force -Path $guiVoiceDirectory | Out-Null
    Expand-BundledVoicebank $guiVoiceDirectory

    $serverVoiceDirectory = Join-Path $serverPath 'voice'
    New-Item -ItemType Directory -Force -Path $serverVoiceDirectory | Out-Null
    Set-Content -LiteralPath (Join-Path $serverVoiceDirectory 'PUT_VOICEBANKS_HERE.txt') -Encoding UTF8 -Value 'Place each UTAU voicebank in its own folder here.'

    Copy-Item -LiteralPath 'docs/server.md' -Destination (Join-Path $serverPath 'README.md')
    Copy-Item -LiteralPath 'docs/manual-pitch.md' -Destination $serverPath
    Copy-Item -LiteralPath 'LICENSE', 'LICENSE-SCOPE.md', 'THIRD_PARTY_NOTICES.txt' -Destination $serverPath

    Write-Step 'Collect exact third-party licenses'
    $qtAuditRoot = Join-Path $root 'build/license-audit/Qt/windows'
    & (Join-Path $PSScriptRoot 'collect-third-party-licenses.ps1') -PackageRoot $guiPath -Variant windows-gui -AuditDirectory $qtAuditRoot
    if ($LASTEXITCODE -ne 0) { throw 'GUI third-party license collection failed' }
    Invoke-Checked $pythonCommand @(
        (Join-Path $root 'tools/verify-qt-sbom.py'),
        '--package-root', $guiPath,
        '--sbom-root', $qtAuditRoot
    )
    & (Join-Path $PSScriptRoot 'collect-third-party-licenses.ps1') -PackageRoot $serverPath -Variant windows-server
    if ($LASTEXITCODE -ne 0) { throw 'Server third-party license collection failed' }

    foreach ($packagePath in @($guiPath, $serverPath)) {
        Get-ChildItem -LiteralPath $packagePath -Recurse -File |
            Where-Object {
                $_.Extension -in @('.pdb', '.lib', '.exp') -and
                $_.FullName.Substring($packagePath.Length + 1) -notmatch '^(licenses[\\/])'
            } |
            Remove-Item -Force
    }
    $qmlToolingPath = Join-Path $guiPath 'app/qmltooling'
    if (Test-Path -LiteralPath $qmlToolingPath) {
        Remove-Item -LiteralPath $qmlToolingPath -Recurse -Force
    }

    Write-Step 'Package'
    Compress-Package $guiPath $guiZip
    Compress-Package $serverPath $serverZip

    & (Join-Path $PSScriptRoot 'test-release-package.ps1') -ReleaseRoot $releaseRoot -Profile $Profile
    if ($LASTEXITCODE -ne 0) { throw "Release package smoke test failed with exit code $LASTEXITCODE" }

    Write-Host 'GUI:'
    Get-ChildItem -LiteralPath $guiPath | Select-Object Name, Length
    Write-Host 'Server:'
    Get-ChildItem -LiteralPath $serverPath | Select-Object Name, Length
    Get-Item -LiteralPath $guiZip, $serverZip | Select-Object FullName, Length
} finally {
    Pop-Location
}

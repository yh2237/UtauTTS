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
    [switch]$SkipTests,
    [switch]$SkipPackageTest
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression.FileSystem

$stepStopwatch = [System.Diagnostics.Stopwatch]::StartNew()
$stepTimings = [System.Collections.Generic.List[object]]::new()
$lastStepName = 'Start'
function Write-Step([string]$Name) {
    $elapsed = $stepStopwatch.Elapsed.TotalSeconds
    $stepTimings.Add([pscustomobject]@{ Step = $script:lastStepName; Seconds = $elapsed })
    Write-Host ("=== {0} === (+{1:N1}s)" -f $Name, $elapsed)
    $script:lastStepName = $Name
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

# Find-SevenZip returns the 7-Zip executable when it is installed.
function Find-SevenZip() {
    foreach ($candidate in @(
            (Join-Path $env:ProgramFiles '7-Zip/7z.exe'),
            (Join-Path ${env:ProgramFiles(x86)} '7-Zip/7z.exe'))) {
        if ($candidate -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            return $candidate
        }
    }
    return $null
}

function Compress-Package([string]$SourceDirectory, [string]$DestinationZip) {
    if (Test-Path -LiteralPath $DestinationZip) {
        Remove-Item -Force -LiteralPath $DestinationZip
    }
    $sevenZip = Find-SevenZip
    if ($sevenZip) {
        # 7-Zip compresses with multiple threads. Model weights do not compress
        # well, so a low level keeps the archive close to the fastest setting.
        # -mcu=on stores names as UTF-8 with the UTF-8 flag. Without it the
        # updater (Go archive/zip) reads Japanese names as CP437 and fails.
        Push-Location -LiteralPath $SourceDirectory
        try {
            & $sevenZip a -tzip -mx=3 -mmt=on -mcu=on -y -bso0 -bsp0 $DestinationZip '*'
            if ($LASTEXITCODE -gt 1) {
                throw "7-Zip failed for $DestinationZip (exit $LASTEXITCODE)"
            }
        } finally {
            Pop-Location
        }
        return
    }
    # Faster than Compress-Archive for a large GUI package.
    [System.IO.Compression.ZipFile]::CreateFromDirectory(
        $SourceDirectory, $DestinationZip,
        [System.IO.Compression.CompressionLevel]::Fastest, $false)
}

# Compress-PackagesInParallel creates the GUI and server ZIPs at the same time.
function Compress-PackagesInParallel([string]$FirstSource, [string]$FirstZip,
        [string]$SecondSource, [string]$SecondZip) {
    $script = {
        param($SourceDirectory, $DestinationZip)
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        if (Test-Path -LiteralPath $DestinationZip) {
            Remove-Item -Force -LiteralPath $DestinationZip
        }
        [System.IO.Compression.ZipFile]::CreateFromDirectory(
            $SourceDirectory, $DestinationZip,
            [System.IO.Compression.CompressionLevel]::Fastest, $false)
    }
    $jobs = @(
        Start-Job -ScriptBlock $script -ArgumentList $FirstSource, $FirstZip
        Start-Job -ScriptBlock $script -ArgumentList $SecondSource, $SecondZip
    )
    $jobs | Wait-Job | Out-Null
    $output = $jobs | Receive-Job
    $failures = @($jobs | Where-Object { $_.State -ne 'Completed' -or $_.ChildJobs[0].Error.Count -gt 0 })
    $jobs | Remove-Job -Force
    if ($failures.Count -gt 0) {
        throw "Package compression failed: $($failures | ForEach-Object { $_.ChildJobs[0].Error } | Out-String)"
    }
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
    # 同梱音源のZIP名はCP932。Expand-Archiveの誤復号を避け、Pythonで文字コードを指定する。
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
    if ($SkipTests -or $env:UTAUTTS_SKIP_TESTS -eq '1') {
        Write-Step 'Test (skipped)'
    } else {
        Write-Step 'Test'
        # 出力・作業フォルダの実験用Goコードは、配布ビルドのテスト対象に含めない。
        Invoke-Checked 'go' @('test', '.', './cmd/...', './internal/...')
    }

    Write-Step 'Build GUI package'
    & (Join-Path $PSScriptRoot 'build-qt.ps1') -OutputDirectory $guiPath
    if ($LASTEXITCODE -ne 0) { throw "Qt GUI build failed with exit code $LASTEXITCODE" }
    Invoke-Checked 'go' @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $guiToolsPath 'utautts-cli.exe'), './cmd/utautts-cli')
    Invoke-Checked 'go' @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $guiToolsPath 'utautts-ustx.exe'), './cmd/tools/utautts-ustx')

    Write-Step 'Build server package'
    Invoke-Checked 'go' @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $serverPath 'utautts-server.exe'), './cmd/utautts-server')
    Copy-Item -LiteralPath (Join-Path $serverPath 'utautts-server.exe') -Destination $guiPath

    Write-Step 'Build native engines'
    & (Join-Path $PSScriptRoot 'build-native-engines.ps1') -OutputDirectory $guiRuntimePath
    if ($LASTEXITCODE -ne 0) { throw "Native engine build failed with exit code $LASTEXITCODE" }
    if ($Profile -eq 'Full') {
        Write-Step 'Build DiffSinger bridge'
        & (Join-Path $PSScriptRoot 'build-diffsinger-bridge.ps1') -OutputDirectory $guiRuntimePath
        if ($LASTEXITCODE -ne 0) { throw "DiffSinger bridge build failed with exit code $LASTEXITCODE" }
    }

    Copy-Item -Path (Join-Path $guiRuntimePath '*') -Destination $serverRuntimePath -Recurse -Force

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
    if ($Profile -eq 'Full') {
        $referencePath = Join-Path $sourceModels 'reference'
        foreach ($packageModelsPath in @($guiModelsPath, $serverModelsPath)) {
            $destination = Join-Path $packageModelsPath 'reference'
            New-Item -ItemType Directory -Force -Path $destination | Out-Null
            foreach ($name in @('kokoro-v1_0.pth', 'config.json', 'af_heart.pt', 'ef_dora.pt', 'pf_dora.pt', 'jf_alpha.pt', 'us_gold.json', 'us_silver.json', 'notices.json')) {
                $source = Join-Path $referencePath $name
                if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
                    throw "Reference model is missing: $source. Run tools/fetch-reference-models.ps1."
                }
                Copy-Item -LiteralPath $source -Destination $destination
            }
            $wavlmSource = Join-Path $referencePath 'wavlm'
            $wavlmDestination = Join-Path $destination 'wavlm'
            New-Item -ItemType Directory -Force -Path $wavlmDestination | Out-Null
            foreach ($name in @('wavlm-base-plus.safetensors', 'config.json')) {
                $source = Join-Path $wavlmSource $name
                if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
                    throw "WavLM model is missing: $source. Run tools/fetch-reference-models.ps1."
                }
                Copy-Item -LiteralPath $source -Destination $wavlmDestination
            }
            $chineseSource = Join-Path $referencePath 'kokoro-zh'
            $chineseDestination = Join-Path $destination 'kokoro-zh'
            New-Item -ItemType Directory -Force -Path $chineseDestination | Out-Null
            foreach ($name in @('kokoro-v1_1-zh.pth', 'config.json', 'zf_001.pt')) {
                $source = Join-Path $chineseSource $name
                if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
                    throw "Chinese reference model is missing: $source. Run tools/fetch-reference-models.ps1."
                }
                Copy-Item -LiteralPath $source -Destination $chineseDestination
            }
        }
    }
    Copy-Item -LiteralPath $modelReadmePath -Destination $guiModelsPath
    Copy-Item -LiteralPath $modelReadmePath -Destination $serverModelsPath
    foreach ($packagePath in @($guiPath, $serverPath)) {
        Invoke-Checked 'go' @(
            'run', './cmd/tools/copy-model-license-notices',
            '--models', (Join-Path $packagePath 'models'),
            '--embedded-models', (Join-Path $root 'internal/speechtiming'),
            '--repository-root', $root,
            '--package-root', $packagePath
        )
    }
    Copy-Item -Path (Join-Path $root 'renderer/*') -Destination $guiRendererPath -Recurse
    Copy-Item -Path (Join-Path $root 'renderer/*') -Destination $serverRendererPath -Recurse
    # 利用者が置くモデルのフォルダは、リポジトリで追跡しているファイル（README）だけを同梱する。
    # 開発者が手元に置いたvocoderの重み（配布条件が別のもの）を配布物へ入れない。
    foreach ($directoryName in @('Dependencies')) {
        $trackedFiles = @(& git -C $root ls-files -- $directoryName)
        if ($LASTEXITCODE -ne 0) { throw "git ls-files failed for $directoryName" }
        foreach ($packagePath in @($guiPath, $serverPath)) {
            New-Item -ItemType Directory -Force -Path (Join-Path $packagePath $directoryName) | Out-Null
            foreach ($trackedFile in $trackedFiles) {
                $destination = Join-Path $packagePath $trackedFile
                New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
                Copy-Item -LiteralPath (Join-Path $root $trackedFile) -Destination $destination
            }
        }
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
    Copy-Item -LiteralPath 'LICENSE', 'LICENSE-SCOPE.md', 'THIRD_PARTY_NOTICES.txt' -Destination $serverPath

    Write-Step 'Collect exact third-party licenses'
    $qtAuditRoot = Join-Path $root 'build/license-audit/Qt/windows'
    & (Join-Path $PSScriptRoot 'collect-third-party-licenses.ps1') -PackageRoot $guiPath -Variant windows-gui -AuditDirectory $qtAuditRoot
    if ($LASTEXITCODE -ne 0) { throw 'GUI third-party license collection failed' }
    Invoke-Checked 'go' @(
        'run', './cmd/tools/verify-qt-sbom',
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

    Write-Step 'Compress packages'
    if (Find-SevenZip) {
        # 7-Zip already uses every core, so the two archives are made in turn.
        Compress-Package $guiPath $guiZip
        Compress-Package $serverPath $serverZip
    } else {
        Compress-PackagesInParallel $guiPath $guiZip $serverPath $serverZip
    }

    Write-Step 'Test release packages'
    if ($SkipPackageTest -or $env:UTAUTTS_SKIP_PACKAGE_TEST -eq '1') {
        Write-Step 'Test release packages (skipped)'
    } else {
        & (Join-Path $PSScriptRoot 'test-release-package.ps1') -ReleaseRoot $releaseRoot -Profile $Profile
        if ($LASTEXITCODE -ne 0) { throw "Release package smoke test failed with exit code $LASTEXITCODE" }
    }

    Write-Host 'GUI:'
    Get-ChildItem -LiteralPath $guiPath | Select-Object Name, Length
    Write-Host 'Server:'
    Get-ChildItem -LiteralPath $serverPath | Select-Object Name, Length
    Get-Item -LiteralPath $guiZip, $serverZip | Select-Object FullName, Length

    $stepTimings.Add([pscustomobject]@{ Step = $script:lastStepName; Seconds = $stepStopwatch.Elapsed.TotalSeconds })
    Write-Host '=== Timings ==='
    $stepTimings | Sort-Object Seconds -Descending | ForEach-Object {
        Write-Host ("{0,6:N1}s  {1}" -f $_.Seconds, $_.Step)
    }
    Write-Host ("{0,6:N1}s  Total" -f (($stepTimings | Measure-Object Seconds -Sum).Sum))
} finally {
    Pop-Location
}

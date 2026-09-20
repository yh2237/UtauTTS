param(
    [string]$PreviousVersion = 'v1.3.0',
    [string]$CandidateZip = '',
    [string]$PreviousZip = '',
    [string]$Repository = $env:GITHUB_REPOSITORY
)

$ErrorActionPreference = 'Stop'
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if ([string]::IsNullOrWhiteSpace($CandidateZip)) {
    $CandidateZip = Join-Path $projectRoot 'release/UtauTTS-win-x64.zip'
}
$CandidateZip = [IO.Path]::GetFullPath($CandidateZip)
$appInfoPath = Join-Path $projectRoot 'internal/appinfo/appinfo.json'
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('utautts-update-compatibility-' + [Guid]::NewGuid().ToString('N'))

function Assert-Path([string]$Path, [string]$Description) {
    if (-not (Test-Path -LiteralPath $Path)) {
        throw ('Missing {0}: {1}' -f $Description, $Path)
    }
}

function Assert-Text([string]$Path, [string]$Expected, [string]$Description) {
    Assert-Path $Path $Description
    if ([IO.File]::ReadAllText($Path) -ne $Expected) {
        throw ('{0} was not preserved: {1}' -f $Description, $Path)
    }
}

function Stop-InstalledProcesses([string]$InstallRoot) {
    $prefix = [IO.Path]::GetFullPath($InstallRoot).TrimEnd('\') + '\'
    for ($attempt = 0; $attempt -lt 10; $attempt++) {
        $matched = @()
        foreach ($process in @(Get-Process -ErrorAction SilentlyContinue)) {
            try {
                if (-not [string]::IsNullOrWhiteSpace($process.Path) -and
                    $process.Path.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
                    $matched += $process
                }
            } catch {
                # Some system processes do not expose their executable path.
            }
        }
        foreach ($process in $matched) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        }
        if ($matched.Count -eq 0 -and $attempt -gt 1) { return }
        Start-Sleep -Milliseconds 500
    }
}

if (-not (Test-Path -LiteralPath $CandidateZip -PathType Leaf)) {
    throw ('Candidate archive was not found: {0}' -f $CandidateZip)
}
Assert-Path $appInfoPath 'application metadata'
$candidateVersion = [string](Get-Content -LiteralPath $appInfoPath -Raw -Encoding UTF8 | ConvertFrom-Json).version
if ([string]::IsNullOrWhiteSpace($candidateVersion)) {
    throw 'Candidate version is empty in internal/appinfo/appinfo.json'
}
if ($candidateVersion -eq $PreviousVersion) {
    throw ('Candidate version must differ from the previous version: {0}' -f $candidateVersion)
}
if ([string]::IsNullOrWhiteSpace($Repository)) {
    $Repository = 'yh2237/UtauTTS'
}

New-Item -ItemType Directory -Force -Path $temporaryRoot | Out-Null
$installRoot = Join-Path $temporaryRoot 'install'
try {
    if ([string]::IsNullOrWhiteSpace($PreviousZip)) {
        $downloadRoot = Join-Path $temporaryRoot 'download'
        New-Item -ItemType Directory -Force -Path $downloadRoot | Out-Null
        Write-Host ('Downloading {0} from {1}' -f $PreviousVersion, $Repository)
        & gh release download $PreviousVersion --repo $Repository --pattern 'UtauTTS-win-x64.zip' --dir $downloadRoot
        if ($LASTEXITCODE -ne 0) {
            throw ('Could not download the {0} Windows package' -f $PreviousVersion)
        }
        $downloaded = @(Get-ChildItem -LiteralPath $downloadRoot -Filter 'UtauTTS-win-x64.zip' -File)
        if ($downloaded.Count -ne 1) {
            throw ('Expected one previous Windows package, found {0}' -f $downloaded.Count)
        }
        $PreviousZip = $downloaded[0].FullName
    } else {
        $PreviousZip = [IO.Path]::GetFullPath($PreviousZip)
    }
    Assert-Path $PreviousZip 'previous release archive'

    $candidateReference = Join-Path $temporaryRoot 'candidate-reference'
    Expand-Archive -LiteralPath $CandidateZip -DestinationPath $candidateReference
    Expand-Archive -LiteralPath $PreviousZip -DestinationPath $installRoot

    $oldUpdater = Join-Path $installRoot 'tools/utautts-updater.exe'
    $candidateUpdater = Join-Path $candidateReference 'tools/utautts-updater.exe'
    Assert-Path $oldUpdater ($PreviousVersion + ' updater')
    Assert-Path $candidateUpdater ($candidateVersion + ' updater')

    $configMarker = [Guid]::NewGuid().ToString('N')
    $configFixture = '[update_compatibility]' + [Environment]::NewLine +
        'marker=' + $configMarker + [Environment]::NewLine
    [IO.File]::WriteAllText((Join-Path $installRoot 'config.ini'), $configFixture)
    $preservedFiles = @{
        'voice/update-compatibility/marker.txt' = 'voice marker'
        'Resamplers/update-compatibility/marker.txt' = 'resampler marker'
        'Wavtools/update-compatibility/marker.txt' = 'wavtool marker'
        'renderer/update-compatibility/marker.txt' = 'renderer marker'
    }
    foreach ($relative in $preservedFiles.Keys) {
        $path = Join-Path $installRoot $relative
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $path) | Out-Null
        [IO.File]::WriteAllText($path, $preservedFiles[$relative])
    }
    $rendererManifest = @{
        manifest_version = 2
        kind = 'synthesis-engine'
        id = 'update-compatibility'
        display_name = 'Update compatibility fixture'
        update_managed = $false
    } | ConvertTo-Json
    [IO.File]::WriteAllText((Join-Path $installRoot 'renderer/update-compatibility/renderer.json'), $rendererManifest)

    $obsoletePath = Join-Path $installRoot 'obsolete-update-compatibility.txt'
    [IO.File]::WriteAllText($obsoletePath, 'this file must disappear')

    Write-Host ('Applying {0} with the updater shipped in {1}' -f $candidateVersion, $PreviousVersion)
    Push-Location $temporaryRoot
    try {
        & $oldUpdater -target $installRoot -zip $CandidateZip -version $candidateVersion -elevated
        if ($LASTEXITCODE -ne 0) {
            throw ('The {0} updater failed with exit code {1}' -f $PreviousVersion, $LASTEXITCODE)
        }
    } finally {
        Pop-Location
    }

    # A successful update starts the installed application. Stop only processes
    # whose executable belongs to this disposable test installation.
    Stop-InstalledProcesses $installRoot

    $installedConfig = Join-Path $installRoot 'config.ini'
    Assert-Path $installedConfig 'configuration'
    if ([IO.File]::ReadAllText($installedConfig) -notmatch [Regex]::Escape($configMarker)) {
        throw ('Configuration marker was not preserved: {0}' -f $installedConfig)
    }
    foreach ($relative in $preservedFiles.Keys) {
        Assert-Text (Join-Path $installRoot $relative) $preservedFiles[$relative] $relative
    }
    if (Test-Path -LiteralPath $obsoletePath) {
        throw ('An obsolete file from {0} survived the update: {1}' -f $PreviousVersion, $obsoletePath)
    }

    $installedUpdater = Join-Path $installRoot 'tools/utautts-updater.exe'
    Assert-Path $installedUpdater 'installed candidate updater'
    $expectedUpdaterHash = (Get-FileHash -LiteralPath $candidateUpdater -Algorithm SHA256).Hash
    $installedUpdaterHash = (Get-FileHash -LiteralPath $installedUpdater -Algorithm SHA256).Hash
    if ($installedUpdaterHash -ne $expectedUpdaterHash) {
        throw 'The updater executable was not replaced by the candidate package'
    }

    $cli = Join-Path $installRoot 'tools/utautts-cli.exe'
    Assert-Path $cli 'installed candidate CLI'
    $reportedVersion = (& $cli --version 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reportedVersion -notmatch [Regex]::Escape($candidateVersion)) {
        throw ('Installed CLI did not report {0}: {1}' -f $candidateVersion, $reportedVersion)
    }

    $gui = Join-Path $installRoot 'app/utautts-gui.exe'
    Assert-Path $gui 'installed candidate GUI'
    $stdout = Join-Path $temporaryRoot 'gui.stdout.log'
    $stderr = Join-Path $temporaryRoot 'gui.stderr.log'
    $guiProcess = Start-Process -FilePath $gui -ArgumentList '--self-test' -WorkingDirectory $installRoot -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    if (-not $guiProcess.WaitForExit(120000)) {
        $guiProcess.Kill()
        throw 'Installed GUI self-test timed out'
    }
    if ($guiProcess.ExitCode -ne 0) {
        $guiError = if (Test-Path -LiteralPath $stderr) {
            Get-Content -LiteralPath $stderr -Raw
        } else {
            ''
        }
        throw ('Installed GUI self-test failed with exit code {0}: {1}' -f $guiProcess.ExitCode, $guiError)
    }

    Write-Host ('Update compatibility passed: {0} -> {1}' -f $PreviousVersion, $candidateVersion)
} finally {
    Stop-InstalledProcesses $installRoot
    if (Test-Path -LiteralPath $temporaryRoot) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
    foreach ($suffix in @('.old', '.stage', '.update-lock.json')) {
        $path = $installRoot + $suffix
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

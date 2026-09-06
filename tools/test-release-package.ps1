param(
    [string]$ReleaseRoot = "$(Join-Path $PSScriptRoot '..\release')",
    [ValidateSet('Full', 'Japanese')]
    [string]$Profile = 'Full'
)

$ErrorActionPreference = 'Stop'
$releaseCheck = Join-Path $PSScriptRoot 'check-release.ps1'
& $releaseCheck
$null = Add-Type -AssemblyName System.IO.Compression.FileSystem
$ReleaseRoot = [IO.Path]::GetFullPath($ReleaseRoot)
$guiZip = Join-Path $ReleaseRoot 'UtauTTS-win-x64.zip'
$serverZip = Join-Path $ReleaseRoot 'UtauTTS-Server-win-x64.zip'
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('utautts-release-test-' + [Guid]::NewGuid().ToString('N'))

function Assert-Path([string]$Path, [string]$Description) {
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Missing ${Description}: $Path"
    }
}

function Assert-Zip([string]$Path) {
    Assert-Path $Path "release archive"
    $archive = [IO.Compression.ZipFile]::OpenRead($Path)
    try {
        if ($archive.Entries.Count -eq 0) {
            throw "Release archive is empty: $Path"
        }
    } finally {
        $archive.Dispose()
    }
}

New-Item -ItemType Directory -Force -Path $temporaryRoot | Out-Null
try {
    Assert-Zip $guiZip
    Assert-Zip $serverZip
    $guiRoot = Join-Path $temporaryRoot 'gui'
    $serverRoot = Join-Path $temporaryRoot 'server'
    Expand-Archive -LiteralPath $guiZip -DestinationPath $guiRoot
    Expand-Archive -LiteralPath $serverZip -DestinationPath $serverRoot

    foreach ($packageRoot in @($guiRoot, $serverRoot)) {
        Assert-Path (Join-Path $packageRoot 'LICENSE') 'project license'
        Assert-Path (Join-Path $packageRoot 'THIRD_PARTY_NOTICES.txt') 'third-party notices'
        Assert-Path (Join-Path $packageRoot 'licenses/README.txt') 'license bundle manifest'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/GO-LICENSE.txt') 'Go runtime license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/APACHE-2.0.txt') 'Apache License 2.0 text'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/gopkg_in_yaml_v3-v3.0.1-LICENSE.txt') 'yaml.v3 license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/gopkg_in_yaml_v3-v3.0.1-NOTICE.txt') 'yaml.v3 notice'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/github_com_ikawaha_kagome-dict-v1.1.7-LICENSE.txt') 'kagome-dict license'
        Assert-Path (Join-Path $packageRoot 'licenses/OpenJTalk/HTS_ENGINE_API_COPYING.txt') 'hts_engine_API license'
        Assert-Path (Join-Path $packageRoot 'runtime/utautts-worldline-bridge.exe') 'native worldline bridge'
        if ($Profile -eq 'Full') {
            Assert-Path (Join-Path $packageRoot 'runtime/utautts-diffsinger-bridge.exe') 'DiffSinger bridge'
            Assert-Path (Join-Path $packageRoot 'runtime/worldline.dll') 'WORLDLINE runtime'
        } else {
            foreach ($optionalRuntime in @('utautts-diffsinger-bridge.exe', 'worldline.dll')) {
                if (Test-Path -LiteralPath (Join-Path $packageRoot "runtime/$optionalRuntime")) {
                    throw "Japanese package contains optional runtime: $optionalRuntime"
                }
            }
        }
        Assert-Path (Join-Path $packageRoot 'runtime/utautts-world-engine.dll') 'UtauTTS WORLD engine'
        Assert-Path (Join-Path $packageRoot 'licenses/WORLD/WORLD-LICENSE.txt') 'official WORLD license'
        Assert-Path (Join-Path $packageRoot 'licenses/WORLD/OOURA-NOTICE.txt') 'Ooura FFT notice'
        Assert-Path (Join-Path $packageRoot 'licenses/WORLD/MACRODEFINITIONS-LICENSE.txt') 'WORLD macro definitions license'
        Assert-Path (Join-Path $packageRoot 'licenses/PROSODY-MODELS.txt') 'prosody model license'
        Assert-Path (Join-Path $packageRoot 'models/README.md') 'model license readme'
        foreach ($rendererId in @('waveform', 'classic-utau', 'utautts-world-phrase')) {
            Assert-Path (Join-Path $packageRoot "renderer/$rendererId/renderer.json") "renderer manifest $rendererId"
        }
        if ($Profile -eq 'Full') {
            Assert-Path (Join-Path $packageRoot 'renderer/openutau-worldline-r-faithful/renderer.json') 'WORLDLINE-R renderer manifest'
            Assert-Path (Join-Path $packageRoot 'renderer/diffsinger/renderer.json') 'DiffSinger renderer manifest'
        } else {
            foreach ($optionalRenderer in @('openutau-worldline-r-faithful', 'diffsinger')) {
                if (Test-Path -LiteralPath (Join-Path $packageRoot "renderer/$optionalRenderer/renderer.json")) {
                    throw "Japanese package contains optional renderer: $optionalRenderer"
                }
            }
        }
        Assert-Path (Join-Path $packageRoot 'runtime/licenses/PYTHON_LICENSE.txt') 'Python runtime license'
        Assert-Path (Join-Path $packageRoot 'runtime/licenses/PYINSTALLER_COPYING.txt') 'PyInstaller license'
    }
    Assert-Path (Join-Path $guiRoot 'THIRD_PARTY_NOTICES-WINDOWS-GUI.txt') 'Windows GUI third-party addendum'
    if (Test-Path -LiteralPath (Join-Path $serverRoot 'THIRD_PARTY_NOTICES-WINDOWS-GUI.txt')) {
        throw 'Server release package must not contain the Windows GUI third-party addendum'
    }
    Assert-Path (Join-Path $serverRoot 'manual-pitch.md') 'server manual pitch documentation'
    Assert-Path (Join-Path $guiRoot 'docs/README.md') 'documentation index'
    Assert-Path (Join-Path $guiRoot 'docs/installation.md') 'installation documentation'
    Assert-Path (Join-Path $guiRoot 'docs/building.md') 'build documentation'
    Assert-Path (Join-Path $guiRoot 'docs/technical-design.md') 'technical design documentation'

    foreach ($asset in @(
        'licenses/Qt/LGPL-3.0.txt',
        'licenses/Qt/Qt-SOURCE-OFFER.txt',
        'licenses/Qt/Qt-RELINK-INSTRUCTIONS.txt',
        'licenses/Qt/Qt-THIRD-PARTY-ATTRIBUTIONS.txt',
        'licenses/Qt/FFmpeg-SOURCE-AND-LICENSE.txt',
        'licenses/Breeze/COPYING-ICONS.txt',
        'licenses/JSUT-DATA-AND-LABELS.txt',
        'licenses/MinGW/COPYING.RUNTIME',
        'licenses/MinGW/COPYING.MinGW-w64-runtime.txt'
    )) {
        Assert-Path (Join-Path $guiRoot $asset) "GUI license asset $asset"
    }

    $ffmpegNoticePath = Join-Path $guiRoot 'licenses/Qt/FFmpeg-SOURCE-AND-LICENSE.txt'
    $ffmpegNotice = Get-Content -LiteralPath $ffmpegNoticePath -Raw
    $ffmpegFiles = @(Get-ChildItem -LiteralPath (Join-Path $guiRoot 'app') -Recurse -File |
        Where-Object { $_.Name -match '^(avcodec|avformat|avutil|swresample|swscale)-\d+\.dll$|ffmpeg' })
    foreach ($ffmpegFile in $ffmpegFiles) {
        if ($ffmpegNotice -notmatch [regex]::Escape($ffmpegFile.Name)) {
            throw "FFmpeg-related file is missing from its package notice: $($ffmpegFile.Name)"
        }
    }

    $serverRuntime = Join-Path $serverRoot 'runtime'
    foreach ($removedRuntime in @('coreclr.dll', 'hostpolicy.dll', 'utautts-worldline-bridge.runtimeconfig.json')) {
        if (Test-Path -LiteralPath (Join-Path $serverRuntime $removedRuntime)) {
            throw "server package still contains an obsolete worldline runtime file: $removedRuntime"
        }
    }
    foreach ($packageRoot in @($guiRoot, $serverRoot)) {
        $gpuRenderer = Test-Path -LiteralPath (Join-Path $packageRoot 'renderer/utautts-world-phrase-cuda/renderer.json')
        if ($gpuRenderer) {
            throw 'experimental CUDA renderer must not be included in a release package'
        }
        $gpuBinary = Test-Path -LiteralPath (Join-Path $packageRoot 'runtime/utautts-waveform-gpu.dll')
        if ($gpuBinary) {
            throw 'experimental CUDA runtime must not be included in a release package'
        }
    }

    $unexpectedDebugFiles = @(Get-ChildItem -LiteralPath $guiRoot -Recurse -File |
        Where-Object { $_.Extension -in @('.pdb', '.lib', '.exp') })
    if ($unexpectedDebugFiles.Count -ne 0) {
        throw "Release package contains debug/development files: $($unexpectedDebugFiles.FullName -join ', ')"
    }
    $allowedTools = @('utautts-cli.exe', 'utautts-ustx.exe', 'utautts-updater.exe')
    $unexpectedTools = @(Get-ChildItem -LiteralPath (Join-Path $guiRoot 'tools') -File |
        Where-Object { $_.Name -notin $allowedTools })
    if ($unexpectedTools.Count -ne 0) {
        throw "GUI release package contains an unexpected tool: $($unexpectedTools.Name -join ', ')"
    }
    foreach ($unusedQtRuntime in @('opengl32sw.dll', 'dxcompiler.dll', 'dxil.dll', 'D3Dcompiler_47.dll')) {
        if (Test-Path -LiteralPath (Join-Path $guiRoot "app/$unusedQtRuntime")) {
            throw "Release package contains an unused Qt auxiliary runtime: $unusedQtRuntime"
        }
    }

    $voicebank = Get-ChildItem -LiteralPath (Join-Path $guiRoot 'voice') -Directory | Select-Object -First 1
    if ($null -eq $voicebank) {
        throw 'GUI release package contains no bundled voicebank'
    }
    $gui = Join-Path $guiRoot 'app/utautts-gui.exe'
    Assert-Path $gui 'packaged GUI'
    $savedQtLogging = $env:QT_FORCE_STDERR_LOGGING
    $env:QT_FORCE_STDERR_LOGGING = '1'
    try {
        $guiStartInfo = [Diagnostics.ProcessStartInfo]::new()
        $guiStartInfo.FileName = $gui
        $guiStartInfo.Arguments = '--self-test'
        $guiStartInfo.WorkingDirectory = $guiRoot
        $guiStartInfo.UseShellExecute = $false
        $guiStartInfo.CreateNoWindow = $true
        $guiStartInfo.RedirectStandardError = $true
        $guiProcess = [Diagnostics.Process]::new()
        $guiProcess.StartInfo = $guiStartInfo
        if (-not $guiProcess.Start()) {
            throw 'Packaged GUI self-test could not be started'
        }
        $guiErrorTask = $guiProcess.StandardError.ReadToEndAsync()
    } finally {
        $env:QT_FORCE_STDERR_LOGGING = $savedQtLogging
    }
    if (-not $guiProcess.WaitForExit(120000)) {
        $guiProcess.Kill()
        throw 'Packaged GUI self-test timed out'
    }
    $guiError = $guiErrorTask.Result
    if ($guiProcess.ExitCode -ne 0) {
        throw "Packaged GUI self-test failed with exit code $($guiProcess.ExitCode): $guiError"
    }

    $cli = Join-Path $guiRoot 'tools/utautts-cli.exe'
    Assert-Path $cli 'packaged CLI'
    Assert-Path (Join-Path $guiRoot 'tools/utautts-ustx.exe') 'packaged USTX export tool'
    $workingDirectory = Join-Path $temporaryRoot 'working-directory'
    New-Item -ItemType Directory -Force -Path $workingDirectory | Out-Null
    $outputWav = Join-Path $workingDirectory 'package-smoke.wav'
    $smokeText = -join @([char]0x3053, [char]0x3093, [char]0x306B, [char]0x3061, [char]0x306F)
    Push-Location $workingDirectory
    try {
        & $cli --renderer waveform --voicebank $voicebank.FullName --text $smokeText --out $outputWav
        if ($LASTEXITCODE -ne 0) {
            throw "Packaged CLI failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
    Assert-Path $outputWav 'packaged CLI output'

    if ($Profile -eq 'Full') {
    $worldlineRWav = Join-Path $workingDirectory 'package-worldline-r-smoke.wav'
    Push-Location $workingDirectory
    try {
        & $cli --voicebank $voicebank.FullName --text $smokeText --prosody frame-intonation-v8 `
            --renderer openutau-worldline-r-faithful --apply-pitch --intonation-strength 1 --out $worldlineRWav
        if ($LASTEXITCODE -ne 0) {
            throw "Packaged WORLDLINE-R synthesis failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
    Assert-Path $worldlineRWav 'packaged WORLDLINE-R renderer output'
    }

    $utauTTSWorldWav = Join-Path $workingDirectory 'package-utautts-world-smoke.wav'
    Push-Location $workingDirectory
    try {
        & $cli --voicebank $voicebank.FullName --text $smokeText --prosody frame-intonation-v8 `
            --renderer utautts-world-phrase --apply-pitch --intonation-strength 1 --out $utauTTSWorldWav
        if ($LASTEXITCODE -ne 0) {
            throw "Packaged UtauTTS WORLD synthesis failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
    Assert-Path $utauTTSWorldWav 'packaged UtauTTS WORLD renderer output'

    $savedErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $nanOutput = & $cli --renderer waveform --voicebank $voicebank.FullName --text $smokeText --mora-ms NaN --out (Join-Path $workingDirectory 'nan.wav') 2>&1
    } finally {
        $ErrorActionPreference = $savedErrorActionPreference
    }
    $nanExitCode = $LASTEXITCODE
    if ($nanExitCode -eq 0 -or ($nanOutput -join "`n") -match 'panic') {
        throw "Packaged CLI accepted or panicked on NaN input: exit=$nanExitCode output=$($nanOutput -join ' ')"
    }

    $server = Join-Path $serverRoot 'utautts-server.exe'
    Assert-Path $server 'packaged server'
    $port = 18000 + (Get-Random -Minimum 0 -Maximum 1000)
    $stdout = Join-Path $temporaryRoot 'server.stdout.log'
    $stderr = Join-Path $temporaryRoot 'server.stderr.log'
    $savedPath = $env:Path
    Remove-Item Env:PATH -ErrorAction SilentlyContinue
    try {
        $process = Start-Process -FilePath $server -ArgumentList @(
            '--host', '127.0.0.1', '--port', $port.ToString(), '--voice-dir', $voicebank.FullName, '--renderer', 'waveform'
        ) -WorkingDirectory $workingDirectory -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
    } finally {
        $env:Path = $savedPath
    }
    try {
        $health = $null
        for ($attempt = 0; $attempt -lt 40; $attempt++) {
            if ($process.HasExited) {
                $errorText = if (Test-Path -LiteralPath $stderr) { Get-Content -LiteralPath $stderr -Raw } else { '' }
                throw "Packaged server exited with code $($process.ExitCode): $errorText"
            }
            try {
                $health = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$port/api/health" -TimeoutSec 2
                break
            } catch {
                Start-Sleep -Milliseconds 250
            }
        }
        if ($null -eq $health -or $health.StatusCode -ne 200) {
            throw 'Packaged server health check timed out'
        }

        $baseUrl = "http://127.0.0.1:$port"
        $console = Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/" -TimeoutSec 5
        if ($console.StatusCode -ne 200 -or $console.Content -notmatch 'UtauTTS Server Console') {
            throw 'Packaged server console is unavailable'
        }
        $voices = Invoke-RestMethod -Uri "$baseUrl/api/voicebanks" -TimeoutSec 5
        $models = Invoke-RestMethod -Uri "$baseUrl/api/models" -TimeoutSec 5
        $renderers = Invoke-RestMethod -Uri "$baseUrl/api/renderers" -TimeoutSec 5
        if (@($voices.voicebanks).Count -lt 1 -or @($models.models).Count -lt 1 -or @($renderers.renderers).Count -lt 1) {
            throw 'Packaged server metadata is incomplete'
        }
        $voicebankId = $voices.voicebanks[0].id
        $analysisBody = @{ text = $smokeText } | ConvertTo-Json -Compress
        $analysis = Invoke-RestMethod -Method Post -Uri "$baseUrl/api/analyze" `
            -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($analysisBody)) -TimeoutSec 15
        if ([string]::IsNullOrWhiteSpace($analysis.reading) -or @($analysis.morae).Count -lt 1) {
            throw 'Packaged server analysis returned no reading'
        }
        $synthesisBody = @{
            text = $smokeText
            voicebank_id = $voicebankId
            renderer = 'waveform'
            mora_duration_ms = 120
        } | ConvertTo-Json -Compress
        $serverWav = Join-Path $workingDirectory 'server-smoke.wav'
        Invoke-WebRequest -UseBasicParsing -Method Post -Uri "$baseUrl/api/synthesize/audio" `
            -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($synthesisBody)) `
            -OutFile $serverWav -TimeoutSec 60 | Out-Null
        Assert-Path $serverWav 'packaged server synthesis output'
        if ((Get-Item -LiteralPath $serverWav).Length -le 44) {
            throw 'Packaged server synthesis output is empty'
        }
        if ($Profile -eq 'Full') {
        $faithfulBody = @{
            text = $smokeText
            voicebank_id = $voicebankId
            model_id = 'frame-intonation-v8'
            renderer = 'openutau-worldline-r-faithful'
            intonation_strength = 1
            apply_pitch = $true
        } | ConvertTo-Json -Compress
        $faithfulServerWav = Join-Path $workingDirectory 'server-faithful-smoke.wav'
        Invoke-WebRequest -UseBasicParsing -Method Post -Uri "$baseUrl/api/synthesize/audio" `
            -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($faithfulBody)) `
            -OutFile $faithfulServerWav -TimeoutSec 120 | Out-Null
        Assert-Path $faithfulServerWav 'packaged server faithful synthesis output'
        if ((Get-Item -LiteralPath $faithfulServerWav).Length -le 44) {
            throw 'Packaged server faithful synthesis output is empty'
        }
        }
        $batchItems = @()
        $batchItems += @{ name = 'first.wav'; request = @{ text = $smokeText; voicebank_id = $voicebankId; renderer = 'waveform' } }
        $singleMora = [string][char]0x3042
        $batchItems += @{ name = 'second.wav'; request = @{ kana = $singleMora; voicebank_id = $voicebankId; renderer = 'waveform' } }
        $batchBody = @{ items = $batchItems } | ConvertTo-Json -Depth 6 -Compress
        $batchZip = Join-Path $workingDirectory 'server-batch.zip'
        Invoke-WebRequest -UseBasicParsing -Method Post -Uri "$baseUrl/api/synthesize/batch" `
            -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($batchBody)) `
            -OutFile $batchZip -TimeoutSec 120 | Out-Null
        Assert-Zip $batchZip
        $reloaded = Invoke-RestMethod -Method Post -Uri "$baseUrl/api/voicebanks/reload" `
            -ContentType 'application/json' -Body '{}' -TimeoutSec 15
        if (@($reloaded.voicebanks).Count -lt 1) {
            throw 'Packaged server voicebank reload returned no voicebank'
        }
    } finally {
        if (-not $process.HasExited) {
            Stop-Process -Id $process.Id -Force
            $process.WaitForExit()
        }
    }
    Write-Host 'Release package smoke test passed'
} finally {
    if (Test-Path -LiteralPath $temporaryRoot) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force
    }
}
exit 0

param(
    [string]$ReleaseRoot = "$(Join-Path $PSScriptRoot '..\release')",
    [ValidateSet('Full', 'Japanese')]
    [string]$Profile = 'Full'
)

$ErrorActionPreference = 'Stop'
$pythonCommand = if ([string]::IsNullOrWhiteSpace($env:PYTHON)) { 'python' } else { $env:PYTHON }
$releaseCheck = Join-Path $PSScriptRoot 'check-release.ps1'
& $releaseCheck
$null = Add-Type -AssemblyName System.IO.Compression.FileSystem
$ReleaseRoot = [IO.Path]::GetFullPath($ReleaseRoot)
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$sourceLicenseScope = [IO.File]::ReadAllText((Join-Path $projectRoot 'LICENSE-SCOPE.md'))
$sourceNotice = [IO.File]::ReadAllText((Join-Path $projectRoot 'THIRD_PARTY_NOTICES.txt'))
$sourceModelReadme = [IO.File]::ReadAllText((Join-Path $projectRoot 'models/README.md'))
$guiZip = Join-Path $ReleaseRoot 'UtauTTS-win-x64.zip'
$serverZip = Join-Path $ReleaseRoot 'UtauTTS-Server-win-x64.zip'
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ('utautts-release-test-' + [Guid]::NewGuid().ToString('N'))

function Assert-Path([string]$Path, [string]$Description) {
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Missing ${Description}: $Path"
    }
}

function Assert-PackagedModelLicenseNotices([string]$PackageRoot) {
    $modelsPath = Join-Path $PackageRoot 'models'
    Assert-Path $modelsPath 'packaged models directory'
    $modelFiles = @(Get-ChildItem -LiteralPath $modelsPath -Filter '*.json' -File)
    if ($modelFiles.Count -eq 0) {
        throw "No packaged model JSON files found: $modelsPath"
    }
    foreach ($modelFile in $modelFiles) {
        try {
            $metadata = Get-Content -LiteralPath $modelFile.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
        } catch {
            throw "Packaged model metadata is not valid JSON: $($modelFile.Name): $($_.Exception.Message)"
        }
        if ([string]::IsNullOrWhiteSpace([string]$metadata.license)) {
            throw "Packaged model has no license description: $($modelFile.Name)"
        }
        $notice = ([string]$metadata.license_notice).Trim()
        if ($notice -notmatch '^licenses/[^/\\]+(?:/[^/\\]+)*$' -or
            $notice -match '(^|/)\.\.?(/|$)' -or
            $notice.Contains(':')) {
            throw "Packaged model has an invalid license_notice path: $($modelFile.Name)"
        }
        $noticePath = Join-Path $PackageRoot ($notice.Replace('/', '\'))
        Assert-Path $noticePath "model license notice for $($modelFile.Name)"
        $sourceNoticePath = Join-Path $projectRoot ($notice.Replace('/', '\'))
        Assert-Path $sourceNoticePath "source model license notice for $($modelFile.Name)"
        if ([IO.File]::ReadAllText($noticePath) -ne [IO.File]::ReadAllText($sourceNoticePath)) {
            throw "Package contains a stale model license notice: $noticePath"
        }
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
        Assert-Path (Join-Path $packageRoot 'LICENSE-SCOPE.md') 'license scope summary'
        Assert-Path (Join-Path $packageRoot 'THIRD_PARTY_NOTICES.txt') 'third-party notices'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/GO-LICENSE.txt') 'Go runtime license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/CMUDICT-LICENSE.txt') 'CMUdict license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/PINYIN-DATA-NOTICE.txt') 'pinyin data provenance notice'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/github_com_ikawaha_kagome_v2-v2.11.0-LICENSE.txt') 'kagome v2 license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/github_com_mozillazg_go-pinyin-v0.21.0-LICENSE.txt') 'go-pinyin license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/github_com_ikawaha_kagome-dict_ipa-v1.2.6-NOTICE.txt') 'kagome IPA notice'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/golang_org_x_text-v0.39.0-PATENTS.txt') 'x/text patents notice'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/gopkg_in_yaml_v3-v3.0.1-LICENSE.txt') 'yaml.v3 license'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/gopkg_in_yaml_v3-v3.0.1-NOTICE.txt') 'yaml.v3 notice'
        Assert-Path (Join-Path $packageRoot 'licenses/Go/github_com_ikawaha_kagome-dict-v1.1.7-LICENSE.txt') 'kagome-dict license'
        Assert-Path (Join-Path $packageRoot 'licenses/OpenJTalk/HTS_ENGINE_API_COPYING.txt') 'hts_engine_API license'
        Assert-Path (Join-Path $packageRoot 'licenses/OpenJTalk/MECAB_COPYING.txt') 'MeCab license'
        Assert-Path (Join-Path $packageRoot 'licenses/OpenJTalk/MECAB_NAIST_JDIC_COPYING.txt') 'MeCab NAIST dictionary license'
        Assert-Path (Join-Path $packageRoot 'licenses/OpenJTalk/OPENJTALK_COPYING.txt') 'Open JTalk license'
        Assert-Path (Join-Path $packageRoot 'runtime/open_jtalk_dic_utf_8-1.11/COPYING') 'Open JTalk dictionary license'
        Assert-Path (Join-Path $packageRoot 'runtime/utautts-worldline-bridge.exe') 'native worldline bridge'
        if ($Profile -eq 'Full') {
            Assert-Path (Join-Path $packageRoot 'runtime/utautts-diffsinger-bridge.exe') 'DiffSinger bridge'
            foreach ($diffSingerLicense in @(
                'ONNXRUNTIME-LICENSE.txt',
                'ONNXRUNTIME-THIRD-PARTY-NOTICES.txt',
                'DIRECTML-LICENSE.txt',
                'DIRECTML-LICENSE-CODE.txt',
                'DIRECTML-THIRD-PARTY-NOTICES.txt',
                'SYSTEM-MEMORY-THIRD-PARTY-NOTICES.txt',
                'SYSTEM-NUMERICS-TENSORS-THIRD-PARTY-NOTICES.txt'
            )) {
                Assert-Path (Join-Path $packageRoot "runtime/licenses/$diffSingerLicense") "DiffSinger dependency license: $diffSingerLicense"
            }
        } else {
            foreach ($optionalRuntime in @('utautts-diffsinger-bridge.exe')) {
                if (Test-Path -LiteralPath (Join-Path $packageRoot "runtime/$optionalRuntime")) {
                    throw "Japanese package contains optional runtime: $optionalRuntime"
                }
            }
        }
        Assert-Path (Join-Path $packageRoot 'runtime/utautts-world-engine.dll') 'UtauTTS WORLD engine'
        Assert-Path (Join-Path $packageRoot 'licenses/WORLD/WORLD-LICENSE.txt') 'official WORLD license'
        Assert-Path (Join-Path $packageRoot 'licenses/WORLD/OOURA-NOTICE.txt') 'Ooura FFT notice'
        Assert-Path (Join-Path $packageRoot 'licenses/WORLD/MACRODEFINITIONS-LICENSE.txt') 'WORLD macro definitions license'
        Assert-Path (Join-Path $packageRoot 'models/README.md') 'model license readme'
        Assert-PackagedModelLicenseNotices $packageRoot
        if ([IO.File]::ReadAllText((Join-Path $packageRoot 'LICENSE-SCOPE.md')) -ne $sourceLicenseScope) {
            throw "Package contains a stale LICENSE-SCOPE.md: $packageRoot"
        }
        if ([IO.File]::ReadAllText((Join-Path $packageRoot 'THIRD_PARTY_NOTICES.txt')) -ne $sourceNotice) {
            throw "Package contains stale THIRD_PARTY_NOTICES.txt: $packageRoot"
        }
        if ([IO.File]::ReadAllText((Join-Path $packageRoot 'models/README.md')) -ne $sourceModelReadme) {
            throw "Package contains a stale models/README.md: $packageRoot"
        }
        $forbiddenBundledData = @(Get-ChildItem -LiteralPath $packageRoot -Recurse -Force -File |
            ForEach-Object {
                $relative = $_.FullName.Substring($packageRoot.Length + 1)
                if ($relative -match '(^|[\\/])(data|out|\.tmp-[^\\/]*)([\\/]|$)') { $_.FullName }
            })
        if ($forbiddenBundledData.Count -ne 0) {
            throw "Release package contains ignored training/build data: $($forbiddenBundledData -join ', ')"
        }
        foreach ($rendererId in @('waveform', 'classic-utau', 'utautts-world-phrase')) {
            Assert-Path (Join-Path $packageRoot "renderer/$rendererId/renderer.json") "renderer manifest $rendererId"
        }
        if ($Profile -eq 'Full') {
            Assert-Path (Join-Path $packageRoot 'renderer/diffsinger/renderer.json') 'DiffSinger renderer manifest'
        } else {
            foreach ($optionalRenderer in @('diffsinger')) {
                if (Test-Path -LiteralPath (Join-Path $packageRoot "renderer/$optionalRenderer/renderer.json")) {
                    throw "Japanese package contains optional renderer: $optionalRenderer"
                }
            }
        }
        Assert-Path (Join-Path $packageRoot 'runtime/licenses/PYTHON_LICENSE.txt') 'Python runtime license'
        Assert-Path (Join-Path $packageRoot 'runtime/licenses/PYINSTALLER_COPYING.txt') 'PyInstaller license'
        foreach ($removedLicense in @(
            'licenses/Go/APACHE-2.0.txt',
            'licenses/Go/github_com_ikawaha_kagome-dict_ipa-v1.2.6-LICENSE.txt',
            'licenses/Go/golang_org_x_text-v0.39.0-LICENSE.txt',
            'licenses/OpenJTalk/DICTIONARY_COPYING.txt',
            'runtime/licenses/OPENSSL-NOTICE.txt',
            'runtime/licenses/OPENSSL-LICENSE.txt',
            'runtime/licenses/ONNXRUNTIME-MANAGED-LICENSE.txt',
            'runtime/licenses/ONNXRUNTIME-MANAGED-THIRD-PARTY-NOTICES.txt'
        )) {
            if (Test-Path -LiteralPath (Join-Path $packageRoot $removedLicense)) {
                throw "Release package contains an obsolete license notice: $removedLicense"
            }
        }
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
        'licenses/Qt/FFmpeg-OPTIONAL.txt',
        'licenses/Qt/Qt-SBOM-MANIFEST.txt',
        'licenses/MinGW/gcc-COPYING',
        'licenses/MinGW/gcc-COPYING.LIB',
        'licenses/MinGW/gcc-COPYING.RUNTIME',
        'licenses/MinGW/mingw-w64-COPYING',
        'licenses/MinGW/mingw-w64-COPYING.MinGW-w64-runtime.txt',
        'licenses/MinGW/mingw-w64-COPYING.MinGW-w64.txt',
        'licenses/MinGW/winpthreads-COPYING'
    )) {
        Assert-Path (Join-Path $guiRoot $asset) "GUI license asset $asset"
    }
    $qtSbomDirectory = Join-Path $guiRoot 'licenses/Qt/sbom'
    if (Test-Path -LiteralPath $qtSbomDirectory) {
        throw 'Raw Qt SBOM JSON must remain outside the release package'
    }
    if (Test-Path -LiteralPath (Join-Path $guiRoot 'licenses/Qt/LGPL-2.1.txt')) {
        throw 'LGPL-2.1 text must not be shipped because FFmpeg is never bundled'
    }
    $ffmpegFiles = @(Get-ChildItem -LiteralPath (Join-Path $guiRoot 'app') -Recurse -File |
        Where-Object { $_.Name -match '^(avcodec|avformat|avutil|swresample|swscale)[-_.].*|ffmpeg' })
    if ($ffmpegFiles.Count -ne 0) {
        throw "Release package contains FFmpeg files: $($ffmpegFiles.Name -join ', ')"
    }

    $serverRuntime = Join-Path $serverRoot 'runtime'
    foreach ($removedRuntime in @('coreclr.dll', 'hostpolicy.dll', 'utautts-worldline-bridge.runtimeconfig.json')) {
        if (Test-Path -LiteralPath (Join-Path $serverRuntime $removedRuntime)) {
            throw "server package still contains an obsolete worldline runtime file: $removedRuntime"
        }
    }
    $unexpectedDebugFiles = @(Get-ChildItem -LiteralPath $guiRoot -Recurse -File |
        Where-Object {
            $_.Extension -in @('.pdb', '.lib', '.exp') -and
            $_.FullName.Substring($guiRoot.Length + 1) -notmatch '^(licenses[\\/])'
        })
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
    if (Test-Path -LiteralPath (Join-Path $guiRoot 'app/translations')) {
        throw 'Release package contains Qt standard translations'
    }
    foreach ($unusedQtStyle in @('FluentWinUI3', 'Imagine', 'Material', 'Universal', 'Windows')) {
        if (Test-Path -LiteralPath (Join-Path $guiRoot "app/qml/QtQuick/Controls/$unusedQtStyle")) {
            throw "Release package contains an unused Qt Quick Controls style: $unusedQtStyle"
        }
    }
    foreach ($unusedQtDialogStyle in @('+Imagine', '+Material', '+Universal')) {
        if (Test-Path -LiteralPath (Join-Path $guiRoot "app/qml/QtQuick/Dialogs/quickimpl/qml/$unusedQtDialogStyle")) {
            throw "Release package contains an unused Qt Quick Dialogs style: $unusedQtDialogStyle"
        }
    }
    foreach ($unusedQtLibrary in @('Qt6QuickEffects.dll', 'Qt6QuickControls2Imagine.dll',
            'Qt6QuickControls2Material.dll', 'Qt6QuickControls2Universal.dll')) {
        if (Test-Path -LiteralPath (Join-Path $guiRoot "app/$unusedQtLibrary")) {
            throw "Release package contains an unused Qt library: $unusedQtLibrary"
        }
    }
    $unusedQtSvgFiles = @(Get-ChildItem -LiteralPath (Join-Path $guiRoot 'app') -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^(?:Qt6?Svg(?:Widgets)?|(?:lib)?qsvg(?:icon)?)\.(?:dll|dylib|so(?:\.\d+)*)$' })
    if ($unusedQtSvgFiles.Count -ne 0) {
        throw "Release package contains unused Qt SVG libraries: $($unusedQtSvgFiles.Name -join ', ')"
    }

    $voicebank = Get-ChildItem -LiteralPath (Join-Path $guiRoot 'voice') -Directory | Select-Object -First 1
    if ($null -eq $voicebank) {
        throw 'GUI release package contains no bundled voicebank'
    }
    $voiceArchives = @(Get-ChildItem -LiteralPath (Join-Path $projectRoot 'voice') -Filter '*.zip' -File)
    if ($voiceArchives.Count -ne 1) {
        throw "Expected exactly one source voicebank archive, found $($voiceArchives.Count)"
    }
    $expectedVoicebankSHA256 = 'B96D1B21145F22E573AFD9EC8AEAAD0EC9CBAEE581C2623C64ADDEB31DE46B3D'
    $actualVoicebankSHA256 = (Get-FileHash -LiteralPath $voiceArchives[0].FullName -Algorithm SHA256).Hash.ToUpperInvariant()
    if ($actualVoicebankSHA256 -ne $expectedVoicebankSHA256) {
        throw "Source voicebank hash mismatch: expected $expectedVoicebankSHA256, got $actualVoicebankSHA256"
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

    $utauTTSWorldWav = Join-Path $workingDirectory 'package-utautts-world-smoke.wav'
    Push-Location $workingDirectory
    try {
        & $cli --voicebank $voicebank.FullName --text $smokeText --prosody frame-intonation-v9-t `
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
        $worldPitchBody = @{
            text = $smokeText
            voicebank_id = $voicebankId
            model_id = 'frame-intonation-v9-t'
            renderer = 'utautts-world-phrase'
            intonation_strength = 1
            apply_pitch = $true
        } | ConvertTo-Json -Compress
        $worldPitchServerWav = Join-Path $workingDirectory 'server-worldPitch-smoke.wav'
        Invoke-WebRequest -UseBasicParsing -Method Post -Uri "$baseUrl/api/synthesize/audio" `
            -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($worldPitchBody)) `
            -OutFile $worldPitchServerWav -TimeoutSec 120 | Out-Null
        Assert-Path $worldPitchServerWav 'packaged server worldPitch synthesis output'
        if ((Get-Item -LiteralPath $worldPitchServerWav).Length -le 44) {
            throw 'Packaged server worldPitch synthesis output is empty'
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

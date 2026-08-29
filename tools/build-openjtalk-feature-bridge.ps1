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
$previousPythonPath = $env:PYTHONPATH
try {
    $env:PYTHONPATH = $pyInstallerRoot
    & $pythonCommand -m PyInstaller --noconfirm --clean --onefile `
        --name utautts-openjtalk-features `
        --paths $inputPath `
        --hidden-import openjtalk `
        --distpath $distPath `
        --workpath $workPath `
        --specpath $specPath `
        (Join-Path $root 'tools/openjtalk-feature-bridge.py')
    if ($LASTEXITCODE -ne 0) {
        throw "PyInstaller failed with exit code $LASTEXITCODE"
    }
} finally {
    $env:PYTHONPATH = $previousPythonPath
}
$helperPath = Join-Path $distPath 'utautts-openjtalk-features.exe'
$verificationCorpus = Join-Path $root 'out/prosody/openjtalk-accent-features-v1.json'
& $pythonCommand (Join-Path $root 'tools/verify-openjtalk-feature-bridge.py') `
    --helper $helperPath --dictionary $dictionaryPath --corpus $verificationCorpus
if ($LASTEXITCODE -ne 0) { throw "Open JTalk helper verification failed with exit code $LASTEXITCODE" }
Get-Item -LiteralPath $helperPath

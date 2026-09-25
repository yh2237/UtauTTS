param(
    [string]$OutputDirectory = (Join-Path (Split-Path $PSScriptRoot -Parent) 'runtime')
)

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$project = Join-Path $PSScriptRoot 'diffsinger-bridge/UtauTTS.DiffSingerBridge.csproj'
$publish = Join-Path $root 'build/diffsinger-bridge/win-x64'
New-Item -ItemType Directory -Force -Path $publish, $OutputDirectory | Out-Null
$nugetRoot = $env:NUGET_PACKAGES
if ([string]::IsNullOrWhiteSpace($nugetRoot)) {
    $nugetRoot = Join-Path $env:USERPROFILE '.nuget/packages'
}
$nugetRoot = [IO.Path]::GetFullPath($nugetRoot)
$env:NUGET_PACKAGES = $nugetRoot

function Copy-RequiredLicense([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "Required DiffSinger license file was not found: $Source"
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

$outputExe = Join-Path $OutputDirectory 'utautts-diffsinger-bridge.exe'
$stampPath = Join-Path $OutputDirectory '.utautts-diffsinger-stamp'
$sourceRoot = Join-Path $PSScriptRoot 'diffsinger-bridge'
$sourceFiles = @(Get-ChildItem -LiteralPath $sourceRoot -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -notmatch '[\\/](bin|obj)[\\/]' } | Sort-Object FullName)
$stampParts = @()
foreach ($sourceFile in $sourceFiles) {
    $relativeName = $sourceFile.FullName.Substring($sourceRoot.Length)
    $sourceHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $sourceFile.FullName).Hash
    $stampParts += ('source:{0}:{1}' -f $relativeName, $sourceHash)
}
$stampParts += 'packages:onnxruntime.DirectML=1.23.0,DirectML=1.15.4,System.Memory=4.5.5,System.Numerics.Tensors=9.0.0'
$stampParts += 'configuration:Release,win-x64,self-contained,singlefile'
$stampText = $stampParts -join "`n"
$stampHasher = [System.Security.Cryptography.SHA256]::Create()
$bridgeStamp = [System.BitConverter]::ToString(
        $stampHasher.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($stampText))).Replace('-', '').ToLowerInvariant()
$bridgeUpToDate = (Test-Path -LiteralPath $outputExe -PathType Leaf) -and
        (Test-Path -LiteralPath $stampPath -PathType Leaf) -and
        ((Get-Content -LiteralPath $stampPath -Raw).Trim() -eq $bridgeStamp)

# dotnet publish is slow, so rebuild only when the sources changed.
if ($bridgeUpToDate) {
    Write-Host 'DiffSinger bridge is up to date.'
} else {
    dotnet publish $project -c Release -r win-x64 --self-contained true `
        -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true `
        -p:PublishTrimmed=false -o $publish
    if ($LASTEXITCODE -ne 0) {
        throw "DiffSinger bridge build failed with exit code $LASTEXITCODE"
    }
    Copy-Item -LiteralPath (Join-Path $publish 'utautts-diffsinger-bridge.exe') -Destination $OutputDirectory -Force
    Set-Content -LiteralPath $stampPath -Value $bridgeStamp -Encoding Ascii
}
$licenses = Join-Path $OutputDirectory 'licenses'
New-Item -ItemType Directory -Force -Path $licenses | Out-Null
foreach ($staleLicense in @(
    'ONNXRUNTIME-MANAGED-LICENSE.txt',
    'ONNXRUNTIME-MANAGED-THIRD-PARTY-NOTICES.txt'
)) {
    $stalePath = Join-Path $licenses $staleLicense
    if (Test-Path -LiteralPath $stalePath -PathType Leaf) {
        Remove-Item -LiteralPath $stalePath -Force
    }
}
$onnxDirectMlRoot = Join-Path (Join-Path $nugetRoot 'microsoft.ml.onnxruntime.directml') '1.23.0'
$onnxManagedRoot = Join-Path (Join-Path $nugetRoot 'microsoft.ml.onnxruntime.managed') '1.23.0'
foreach ($sharedNotice in @(
    @{ DirectMl = 'LICENSE'; Managed = 'LICENSE.txt' },
    @{ DirectMl = 'ThirdPartyNotices.txt'; Managed = 'ThirdPartyNotices.txt' }
)) {
    $directMlHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $onnxDirectMlRoot $sharedNotice.DirectMl)).Hash
    $managedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $onnxManagedRoot $sharedNotice.Managed)).Hash
    if ($directMlHash -ne $managedHash) {
        throw "ONNX Runtime license files differ between DirectML and Managed packages: $($sharedNotice.DirectMl)"
    }
}
$packageFiles = @(
    @{ Package = 'microsoft.ml.onnxruntime.directml'; Version = '1.23.0'; Source = 'LICENSE'; Destination = 'ONNXRUNTIME-LICENSE.txt' },
    @{ Package = 'microsoft.ml.onnxruntime.directml'; Version = '1.23.0'; Source = 'ThirdPartyNotices.txt'; Destination = 'ONNXRUNTIME-THIRD-PARTY-NOTICES.txt' },
    @{ Package = 'microsoft.ai.directml'; Version = '1.15.4'; Source = 'LICENSE.txt'; Destination = 'DIRECTML-LICENSE.txt' },
    @{ Package = 'microsoft.ai.directml'; Version = '1.15.4'; Source = 'LICENSE-CODE.txt'; Destination = 'DIRECTML-LICENSE-CODE.txt' },
    @{ Package = 'microsoft.ai.directml'; Version = '1.15.4'; Source = 'ThirdPartyNotices.txt'; Destination = 'DIRECTML-THIRD-PARTY-NOTICES.txt' },
    @{ Package = 'system.memory'; Version = '4.5.5'; Source = 'THIRD-PARTY-NOTICES.TXT'; Destination = 'SYSTEM-MEMORY-THIRD-PARTY-NOTICES.txt' },
    @{ Package = 'system.numerics.tensors'; Version = '9.0.0'; Source = 'THIRD-PARTY-NOTICES.TXT'; Destination = 'SYSTEM-NUMERICS-TENSORS-THIRD-PARTY-NOTICES.txt' }
)
foreach ($packageFile in $packageFiles) {
    $packageRoot = Join-Path (Join-Path $nugetRoot $packageFile.Package) $packageFile.Version
    Copy-RequiredLicense (Join-Path $packageRoot $packageFile.Source) (Join-Path $licenses $packageFile.Destination)
}
$runtimeRoot = Join-Path $nugetRoot 'microsoft.netcore.app.runtime.win-x64'
$runtimePackage = Get-ChildItem -LiteralPath $runtimeRoot -Directory | Sort-Object { [version]$_.Name } -Descending | Select-Object -First 1
if ($null -eq $runtimePackage) { throw 'The .NET runtime package was not found' }
Copy-RequiredLicense (Join-Path $runtimePackage.FullName 'LICENSE.TXT') (Join-Path $licenses 'DOTNET-RUNTIME-LICENSE.txt')
Copy-RequiredLicense (Join-Path $runtimePackage.FullName 'THIRD-PARTY-NOTICES.TXT') (Join-Path $licenses 'DOTNET-RUNTIME-THIRD-PARTY-NOTICES.txt')

param(
    [string]$OutputDirectory = (Join-Path (Split-Path $PSScriptRoot -Parent) 'runtime')
)

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$project = Join-Path $PSScriptRoot 'diffsinger-bridge/UtauTTS.DiffSingerBridge.csproj'
$publish = Join-Path $root 'build/diffsinger-bridge/win-x64'
New-Item -ItemType Directory -Force -Path $publish, $OutputDirectory | Out-Null

function Copy-RequiredLicense([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "Required DiffSinger license file was not found: $Source"
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

dotnet publish $project -c Release -r win-x64 --self-contained true `
    -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true `
    -p:PublishTrimmed=false -o $publish
if ($LASTEXITCODE -ne 0) {
    throw "DiffSinger bridge build failed with exit code $LASTEXITCODE"
}

Copy-Item -LiteralPath (Join-Path $publish 'utautts-diffsinger-bridge.exe') -Destination $OutputDirectory -Force
$licenses = Join-Path $OutputDirectory 'licenses'
New-Item -ItemType Directory -Force -Path $licenses | Out-Null
$nugetRoot = Join-Path $env:USERPROFILE '.nuget/packages'
$packageFiles = @(
    @{ Package = 'microsoft.ml.onnxruntime.directml'; Version = '1.23.0'; Source = 'LICENSE'; Destination = 'ONNXRUNTIME-LICENSE.txt' },
    @{ Package = 'microsoft.ml.onnxruntime.directml'; Version = '1.23.0'; Source = 'ThirdPartyNotices.txt'; Destination = 'ONNXRUNTIME-THIRD-PARTY-NOTICES.txt' },
    @{ Package = 'microsoft.ml.onnxruntime.managed'; Version = '1.23.0'; Source = 'LICENSE.txt'; Destination = 'ONNXRUNTIME-MANAGED-LICENSE.txt' },
    @{ Package = 'microsoft.ml.onnxruntime.managed'; Version = '1.23.0'; Source = 'ThirdPartyNotices.txt'; Destination = 'ONNXRUNTIME-MANAGED-THIRD-PARTY-NOTICES.txt' },
    @{ Package = 'microsoft.ai.directml'; Version = '1.15.4'; Source = 'LICENSE.txt'; Destination = 'DIRECTML-LICENSE.txt' },
    @{ Package = 'microsoft.ai.directml'; Version = '1.15.4'; Source = 'LICENSE-CODE.txt'; Destination = 'DIRECTML-LICENSE-CODE.txt' },
    @{ Package = 'microsoft.ai.directml'; Version = '1.15.4'; Source = 'ThirdPartyNotices.txt'; Destination = 'DIRECTML-THIRD-PARTY-NOTICES.txt' }
)
foreach ($packageFile in $packageFiles) {
    $packageRoot = Join-Path (Join-Path $nugetRoot $packageFile.Package) $packageFile.Version
    Copy-RequiredLicense (Join-Path $packageRoot $packageFile.Source) (Join-Path $licenses $packageFile.Destination)
}
$runtimeRoot = Join-Path $env:USERPROFILE '.nuget/packages/microsoft.netcore.app.runtime.win-x64'
$runtimePackage = Get-ChildItem -LiteralPath $runtimeRoot -Directory | Sort-Object { [version]$_.Name } -Descending | Select-Object -First 1
if ($null -eq $runtimePackage) { throw 'The .NET runtime package was not found' }
Copy-RequiredLicense (Join-Path $runtimePackage.FullName 'LICENSE.TXT') (Join-Path $licenses 'DOTNET-RUNTIME-LICENSE.txt')
Copy-RequiredLicense (Join-Path $runtimePackage.FullName 'THIRD-PARTY-NOTICES.TXT') (Join-Path $licenses 'DOTNET-RUNTIME-THIRD-PARTY-NOTICES.txt')

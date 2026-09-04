param(
    [string]$OutputDirectory = (Join-Path (Split-Path $PSScriptRoot -Parent) 'runtime')
)

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$project = Join-Path $PSScriptRoot 'diffsinger-bridge/UtauTTS.DiffSingerBridge.csproj'
$publish = Join-Path $root 'build/diffsinger-bridge/win-x64'
New-Item -ItemType Directory -Force -Path $publish, $OutputDirectory | Out-Null

dotnet publish $project -c Release -r win-x64 --self-contained true `
    -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true `
    -p:PublishTrimmed=false -o $publish
if ($LASTEXITCODE -ne 0) {
    throw "DiffSinger bridge build failed with exit code $LASTEXITCODE"
}

Copy-Item -LiteralPath (Join-Path $publish 'utautts-diffsinger-bridge.exe') -Destination $OutputDirectory -Force
$licenses = Join-Path $OutputDirectory 'licenses'
New-Item -ItemType Directory -Force -Path $licenses | Out-Null
$onnxPackage = Join-Path $env:USERPROFILE '.nuget/packages/microsoft.ml.onnxruntime.directml/1.23.0'
Copy-Item -LiteralPath (Join-Path $onnxPackage 'LICENSE') -Destination (Join-Path $licenses 'ONNXRUNTIME-LICENSE.txt') -Force
Copy-Item -LiteralPath (Join-Path $onnxPackage 'ThirdPartyNotices.txt') -Destination (Join-Path $licenses 'ONNXRUNTIME-THIRD-PARTY-NOTICES.txt') -Force
$runtimeRoot = Join-Path $env:USERPROFILE '.nuget/packages/microsoft.netcore.app.runtime.win-x64'
$runtimePackage = Get-ChildItem -LiteralPath $runtimeRoot -Directory | Sort-Object { [version]$_.Name } -Descending | Select-Object -First 1
if ($null -eq $runtimePackage) { throw 'The .NET runtime package was not found' }
Copy-Item -LiteralPath (Join-Path $runtimePackage.FullName 'LICENSE.TXT') -Destination (Join-Path $licenses 'DOTNET-RUNTIME-LICENSE.txt') -Force
Copy-Item -LiteralPath (Join-Path $runtimePackage.FullName 'THIRD-PARTY-NOTICES.TXT') -Destination (Join-Path $licenses 'DOTNET-RUNTIME-THIRD-PARTY-NOTICES.txt') -Force

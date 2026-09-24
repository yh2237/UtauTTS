param(
    [Parameter(Mandatory = $true)]
    [string]$PackageRoot,
    [ValidateSet('windows-gui', 'windows-server', 'linux')]
    [string]$Variant = 'windows-gui',
    [string]$AuditDirectory = ''
)

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$PackageRoot = [IO.Path]::GetFullPath($PackageRoot)
$licenseRoot = Join-Path $PackageRoot 'licenses'

function Copy-Required([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "Required license file was not found: $Source"
    }
    $destinationDirectory = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Force -Path $destinationDirectory | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

function Write-ReleaseText([string]$Path, [string]$Text) {
    $directory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    Set-Content -LiteralPath $Path -Value $Text -Encoding UTF8
}

function Get-CommandOutput([string]$Command, [string[]]$Arguments) {
    $output = & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command failed with exit code $LASTEXITCODE"
    }
    return ($output -join "`n").Trim()
}

function Resolve-QtRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:QT_ROOT)) {
        $candidate = [IO.Path]::GetFullPath($env:QT_ROOT)
        if (Test-Path -LiteralPath (Join-Path $candidate 'lib/cmake/Qt6/Qt6Config.cmake') -PathType Leaf) {
            return $candidate
        }
    }
    $localQtDirectory = Join-Path $root '.qt'
    $kits = @(Get-ChildItem -LiteralPath $localQtDirectory -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        $candidate = Join-Path $_.FullName 'mingw_64'
        if (Test-Path -LiteralPath (Join-Path $candidate 'lib/cmake/Qt6/Qt6Config.cmake') -PathType Leaf) {
            Get-Item -LiteralPath $candidate
        }
    } | Sort-Object { [version]$_.Parent.Name } -Descending)
    if ($kits.Count -eq 0) {
        throw 'Qt license collection requires the Qt kit used for the GUI build'
    }
    return $kits[0].FullName
}

function Copy-GoLicenses {
    $goLicenseRoot = Join-Path $licenseRoot 'Go'
    New-Item -ItemType Directory -Force -Path $goLicenseRoot | Out-Null
    Get-ChildItem -LiteralPath $goLicenseRoot -File -ErrorAction SilentlyContinue |
        Remove-Item -Force
    $goRoot = Get-CommandOutput 'go' @('env', 'GOROOT')
    $goLicensePath = Join-Path $licenseRoot 'Go/GO-LICENSE.txt'
    Copy-Required (Join-Path $goRoot 'LICENSE') $goLicensePath
    Copy-Required (Join-Path $root 'licenses/Go/CMUDICT-LICENSE.txt') (Join-Path $licenseRoot 'Go/CMUDICT-LICENSE.txt')
    Copy-Required (Join-Path $root 'licenses/Go/PINYIN-DATA-NOTICE.txt') (Join-Path $licenseRoot 'Go/PINYIN-DATA-NOTICE.txt')
    $licenseHashes = @{
        (Get-FileHash -Algorithm SHA256 -LiteralPath $goLicensePath).Hash = $goLicensePath
    }

    $modules = @(Get-Content -LiteralPath (Join-Path $PSScriptRoot 'go-license-modules.txt') -Encoding UTF8 |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    foreach ($module in $modules) {
        $moduleInfo = Get-CommandOutput 'go' @('list', '-m', '-f={{.Dir}}|{{.Version}}', $module)
        $parts = $moduleInfo.Split('|', 2)
        if ($parts.Count -ne 2) {
            throw "Could not resolve Go module metadata: $moduleInfo"
        }
        $moduleDirectory = $parts[0]
        $moduleVersion = $parts[1]
        $safeName = $module.Replace('/', '_').Replace('.', '_')
        $licenseFiles = @(Get-ChildItem -LiteralPath $moduleDirectory -Recurse -File |
            Where-Object {
                $_.Name -match '^(LICENSE|COPYING|PATENTS)(\..*)?$' -or
                $_.Name -match '^(NOTICE|THIRD_PARTY_NOTICES|DATA_LICENSES)(\..*)?$'
            } | Sort-Object FullName)
        $primaryLicense = $licenseFiles | Where-Object {
            $_.Name -match '^(LICENSE|COPYING)(\..*)?$'
        } | Select-Object -First 1
        if ($null -eq $primaryLicense) {
            throw "A license file was not found for Go module: $module"
        }
        foreach ($licenseFile in $licenseFiles) {
            $isPrimaryLicense = $licenseFile.Name -match '^(LICENSE|COPYING)(\..*)?$'
            $licenseHash = $null
            if ($isPrimaryLicense) {
                $licenseHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $licenseFile.FullName).Hash
                if ($licenseHashes.ContainsKey($licenseHash)) {
                    continue
                }
            }
            $relativeName = $licenseFile.FullName.Substring($moduleDirectory.Length).TrimStart('\', '/')
            $destinationName = $relativeName.Replace('\', '__').Replace('/', '__')
            if ($relativeName -eq 'LICENSE') { $destinationName = 'LICENSE.txt' }
            if ($relativeName -eq 'NOTICE') { $destinationName = 'NOTICE.txt' }
            if ($relativeName -eq 'PATENTS') { $destinationName = 'PATENTS.txt' }
            $destination = Join-Path $licenseRoot "Go/$safeName-$moduleVersion-$destinationName"
            Copy-Required $licenseFile.FullName $destination
            if ($isPrimaryLicense) {
                $licenseHashes[$licenseHash] = $destination
            }
        }
    }
}

function Copy-OpenJTalkLicenses {
    $source = Join-Path $root 'licenses/openjtalk'
    if (-not (Test-Path -LiteralPath $source -PathType Container)) {
        throw "Open JTalk license sources are missing: $source"
    }
    $staleDestination = Join-Path $licenseRoot 'openjtalk'
    if (Test-Path -LiteralPath $staleDestination) {
        Remove-Item -LiteralPath $staleDestination -Recurse -Force
    }
    $staleDictionaryCopying = Join-Path $licenseRoot 'OpenJTalk/DICTIONARY_COPYING.txt'
    if (Test-Path -LiteralPath $staleDictionaryCopying -PathType Leaf) {
        Remove-Item -LiteralPath $staleDictionaryCopying -Force
    }
    Get-ChildItem -LiteralPath $source -File | ForEach-Object {
        Copy-Required $_.FullName (Join-Path $licenseRoot "OpenJTalk/$($_.Name)")
    }
    $dictionaryCopying = Join-Path $PackageRoot 'runtime/open_jtalk_dic_utf_8-1.11/COPYING'
    if (-not (Test-Path -LiteralPath $dictionaryCopying -PathType Leaf)) {
        throw "Open JTalk dictionary license was not found: $dictionaryCopying"
    }
}

function Copy-IconFontLicenses {
    Copy-Required (Join-Path $root 'licenses/MATERIAL-SYMBOLS.txt') (Join-Path $licenseRoot 'MATERIAL-SYMBOLS.txt')
}

function Copy-WorldLicenses {
    Copy-Required (Join-Path $root 'third_party/world/LICENSE.txt') (Join-Path $licenseRoot 'WORLD/WORLD-LICENSE.txt')
    Copy-Required (Join-Path $root 'third_party/world/OOURA-NOTICE.txt') (Join-Path $licenseRoot 'WORLD/OOURA-NOTICE.txt')
    Copy-Required (Join-Path $root 'third_party/world/MACRODEFINITIONS-LICENSE.txt') (Join-Path $licenseRoot 'WORLD/MACRODEFINITIONS-LICENSE.txt')
}

function Copy-QtLicenses {
    $qtRoot = Resolve-QtRoot
    $qtVersion = (Get-Item -LiteralPath $qtRoot).Parent.Name
    $qtDocSeries = $qtVersion.Substring(0, $qtVersion.LastIndexOf('.'))
    $toolsRoot = [IO.Path]::GetFullPath((Join-Path $qtRoot '../../Tools'))
    $qtLicenseRoot = Join-Path $licenseRoot 'Qt'
    foreach ($staleFile in @('LGPL-2.1.txt', 'FFmpeg-SOURCE-AND-LICENSE.txt', 'FFmpeg-SOURCE-OFFER.txt', 'FFmpeg-OPTIONAL.txt')) {
        $stalePath = Join-Path $qtLicenseRoot $staleFile
        if (Test-Path -LiteralPath $stalePath -PathType Leaf) {
            Remove-Item -LiteralPath $stalePath -Force
        }
    }
    $staleSbomDirectory = Join-Path $qtLicenseRoot 'sbom'
    if (Test-Path -LiteralPath $staleSbomDirectory -PathType Container) {
        Remove-Item -LiteralPath $staleSbomDirectory -Recurse -Force
    }
    $sbomSourceRoot = Join-Path $qtRoot 'sbom'
    $sbomAuditRoot = if ([string]::IsNullOrWhiteSpace($AuditDirectory)) {
        Join-Path $root 'build/license-audit/Qt/windows'
    } else {
        [IO.Path]::GetFullPath($AuditDirectory)
    }
    if (Test-Path -LiteralPath $sbomAuditRoot) {
        Remove-Item -LiteralPath $sbomAuditRoot -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $sbomAuditRoot | Out-Null
    $sbomNames = @(
        "qtbase-$qtVersion.spdx.json",
        "qtdeclarative-$qtVersion.spdx.json",
        "qtmultimedia-$qtVersion.spdx.json"
    )
    $sbomLines = @(
        "Qt SBOM files for Qt $qtVersion",
        "=================================",
        "",
        'Raw SPDX JSON files are kept in the build audit directory and are not included in the release package.',
        'Audit directory: build/license-audit/Qt/windows',
        ''
    )
    foreach ($sbomName in $sbomNames) {
        $sbomSource = Join-Path $sbomSourceRoot $sbomName
        if (-not (Test-Path -LiteralPath $sbomSource -PathType Leaf)) {
            throw "Qt SBOM file was not found: $sbomSource"
        }
        Copy-Required $sbomSource (Join-Path $sbomAuditRoot $sbomName)
        $sbomHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $sbomSource).Hash
        $sbomLines += $sbomName
        $sbomLines += "  SHA-256: $sbomHash"
    }
    $ffmpegDlls = @(Get-ChildItem -LiteralPath (Join-Path $PackageRoot 'app') -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^(avcodec|avformat|avutil|swresample|swscale)[-_.].*|ffmpeg' })
    if ($ffmpegDlls.Count -gt 0) {
        $names = $ffmpegDlls | ForEach-Object { $_.Name }
        throw "FFmpeg files must not be bundled: $($names -join ', ')"
    }
    $sbomLines += @(
        '',
        'Qt Multimedia FFmpeg status:',
        'FFmpeg is not bundled. The Qt Multimedia SBOM may list optional FFmpeg support from the Qt SDK.',
        'FFmpeg package files bundled: false'
    )
    Write-ReleaseText (Join-Path $qtLicenseRoot 'Qt-SBOM-MANIFEST.txt') ($sbomLines -join [Environment]::NewLine)

    $lgpl = Get-ChildItem -LiteralPath $toolsRoot -Recurse -File -Filter 'LGPLv3.txt' -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $lgpl) {
        Copy-Required $lgpl.FullName (Join-Path $licenseRoot 'Qt/LGPL-3.0.txt')
    } else {
        Copy-Required (Join-Path $root 'licenses/Qt/LGPL-3.0.txt') (Join-Path $licenseRoot 'Qt/LGPL-3.0.txt')
    }
    Copy-Required (Join-Path $root 'licenses/Qt/GPL-3.0.txt') (Join-Path $licenseRoot 'Qt/GPL-3.0.txt')
    $qtSourceOffer = @"
Qt source offer
===============

This package contains dynamically linked Qt $qtVersion libraries.
This is a written offer for the complete corresponding source of the LGPL-covered
Qt modules used by this package. For at least three years after this package was
distributed, UtauTTS will provide that source in a machine-readable archive at no
charge other than the reasonable cost of performing the source distribution.

Source requests:
https://github.com/yh2237/UtauTTS/issues/new?title=Qt%20source%20request

Include the UtauTTS release version and the Qt version shown in this file in a
source request. The project repository and its build scripts provide the
corresponding application source and relinking instructions.

The upstream source archives used to prepare the corresponding source archive are:

https://download.qt.io/official_releases/qt/$($qtVersion.Substring(0, $qtVersion.LastIndexOf('.')))/$qtVersion/submodules/
https://code.qt.io/cgit/qt/qt5.git/tag/?h=v$qtVersion

The Qt modules used here include Qt Core, Qt GUI, Qt QML, Qt Quick,
Qt Quick Controls, Qt Multimedia, and Qt Concurrent. This offer covers the
corresponding Qt version used by the build, not an arbitrary later version.
"@
    Write-ReleaseText (Join-Path $licenseRoot 'Qt/Qt-SOURCE-OFFER.txt') $qtSourceOffer

    $qtRelinkInstructions = @"
Qt replacement and relinking information
==========================================

The GUI links dynamically to the Qt DLLs distributed under app/. An end user may
replace those DLLs with compatible modified LGPL-covered Qt builds, subject to
Qt's license terms and ABI compatibility.

To rebuild the application against a modified Qt build:

1. Obtain the UtauTTS source for the same release.
2. Set QT_ROOT to the Qt compiler kit directory containing lib/cmake/Qt6.
3. Build the native application with tools/build-qt.ps1 and package it with
   tools/build-release.ps1 as described in README.md.
4. Deploy the resulting application with the compatible modified Qt DLLs.

The corresponding Qt source offer, LGPLv3 and GPLv3 texts, and third-party attribution
information are included beside this file. Raw Qt SBOM JSON files are kept in
build/license-audit/Qt/windows during the build.
"@
    Write-ReleaseText (Join-Path $licenseRoot 'Qt/Qt-RELINK-INSTRUCTIONS.txt') $qtRelinkInstructions

    $qtAttributions = @"
Qt $qtVersion third-party attributions
======================================

Qt's modules contain third-party components with their own copyright and license
terms. Raw SPDX SBOM files used for this package are kept in the build audit
directory, with SHA-256 values recorded in Qt-SBOM-MANIFEST.txt. The
authoritative attribution list for this Qt version is:

https://doc.qt.io/qt-$qtDocSeries/licenses-used-in-qt.html

Qt Multimedia attribution and optional FFmpeg guidance:
https://doc.qt.io/qt-$qtDocSeries/qtmultimedia-attribution-ffmpeg.html
https://ffmpeg.org/legal.html

The LGPLv3 and GPLv3 texts are included beside this file.
"@
    Write-ReleaseText (Join-Path $licenseRoot 'Qt/Qt-THIRD-PARTY-ATTRIBUTIONS.txt') $qtAttributions

    $mingwRoots = @(Get-ChildItem -LiteralPath $toolsRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'licenses/mingw-w64/COPYING.MinGW-w64-runtime.txt') } |
        Sort-Object Name -Descending)
    if ($mingwRoots.Count -eq 0) {
        throw "The Qt SDK MinGW runtime licenses were not found: $toolsRoot"
    }
    $mingwLicenseRoot = Join-Path $mingwRoots[0].FullName 'licenses'
    $mingwFiles = @(
        'gcc/COPYING',
        'gcc/COPYING.LIB',
        'gcc/COPYING.RUNTIME',
        'mingw-w64/COPYING',
        'mingw-w64/COPYING.MinGW-w64-runtime.txt',
        'mingw-w64/COPYING.MinGW-w64.txt',
        'winpthreads/COPYING'
    )
    foreach ($relativePath in $mingwFiles) {
        $parts = $relativePath -split '[\\/]'
        $destinationName = "$($parts[0])-$($parts[1])"
        Copy-Required (Join-Path $mingwLicenseRoot $relativePath) (Join-Path $licenseRoot "MinGW/$destinationName")
    }
    $ffmpegOptional = @"
Optional FFmpeg runtime
=======================

UtauTTS does not bundle FFmpeg. Qt Multimedia uses its native backend when
available. If an external Qt Multimedia FFmpeg backend is needed, set the path
in Settings or one of these environment variables before the first launch:

UTAUTTS_FFMPEG_PATH
FFMPEG_PATH
FFMPEG_DIR
FFMPEG_ROOT

The path should point to a directory containing the Qt Multimedia FFmpeg plugin
and its codec libraries. A standalone ffmpeg command-line executable is not a
replacement for that plugin. See the Qt Multimedia and FFmpeg documentation:
https://doc.qt.io/qt-6.8/qtmultimedia-index.html
https://ffmpeg.org/legal.html
"@
    Write-ReleaseText (Join-Path $licenseRoot 'Qt/FFmpeg-OPTIONAL.txt') $ffmpegOptional
}
New-Item -ItemType Directory -Force -Path $licenseRoot | Out-Null
Copy-GoLicenses
Copy-OpenJTalkLicenses
Copy-IconFontLicenses
Copy-WorldLicenses

if ($Variant -eq 'windows-gui') {
    Copy-QtLicenses
}

Write-Host "Collected third-party licenses for $Variant at $licenseRoot"

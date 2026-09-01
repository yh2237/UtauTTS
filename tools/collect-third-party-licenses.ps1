param(
    [Parameter(Mandatory = $true)]
    [string]$PackageRoot,
    [ValidateSet('windows-gui', 'windows-server', 'linux')]
    [string]$Variant = 'windows-gui',
    [switch]$CudaIncluded
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
    $goRoot = Get-CommandOutput 'go' @('env', 'GOROOT')
    Copy-Required (Join-Path $goRoot 'LICENSE') (Join-Path $licenseRoot 'Go/GO-LICENSE.txt')
    Copy-Required (Join-Path $root 'licenses/APACHE-2.0.txt') (Join-Path $licenseRoot 'Go/APACHE-2.0.txt')

    $modules = @(
        'golang.org/x/text',
        'github.com/ikawaha/kagome/v2',
        'github.com/ikawaha/kagome-dict',
        'github.com/ikawaha/kagome-dict/ipa',
        'gopkg.in/yaml.v3'
    )
    foreach ($module in $modules) {
        $moduleInfo = Get-CommandOutput 'go' @('list', '-m', '-f={{.Dir}}|{{.Version}}', $module)
        $parts = $moduleInfo.Split('|', 2)
        if ($parts.Count -ne 2) {
            throw "Could not resolve Go module metadata: $moduleInfo"
        }
        $moduleDirectory = $parts[0]
        $moduleVersion = $parts[1]
        $safeName = $module.Replace('/', '_').Replace('.', '_')
        Copy-Required (Join-Path $moduleDirectory 'LICENSE') (Join-Path $licenseRoot "Go/$safeName-$moduleVersion-LICENSE.txt")
        foreach ($noticeName in @('NOTICE', 'NOTICE.txt')) {
            $notice = Join-Path $moduleDirectory $noticeName
            if (Test-Path -LiteralPath $notice -PathType Leaf) {
                Copy-Required $notice (Join-Path $licenseRoot "Go/$safeName-$moduleVersion-NOTICE.txt")
                break
            }
        }
    }
}

function Copy-OpenJTalkLicenses {
    $source = Join-Path $root 'licenses/openjtalk'
    if (-not (Test-Path -LiteralPath $source -PathType Container)) {
        throw "Open JTalk license sources are missing: $source"
    }
    Copy-Item -LiteralPath $source -Destination (Join-Path $licenseRoot 'OpenJTalk') -Recurse -Force
    $dictionaryCopying = Join-Path $PackageRoot 'runtime/open_jtalk_dic_utf_8-1.11/COPYING'
    Copy-Required $dictionaryCopying (Join-Path $licenseRoot 'OpenJTalk/DICTIONARY_COPYING.txt')
}

function Copy-ProsodyDataProvenance {
    Copy-Required (Join-Path $root 'licenses/JSUT-DATA-AND-LABELS.txt') (Join-Path $licenseRoot 'JSUT-DATA-AND-LABELS.txt')
    Copy-Required (Join-Path $root 'licenses/PROSODY-MODELS.txt') (Join-Path $licenseRoot 'PROSODY-MODELS.txt')
}

function Copy-WorldlineLicenses {
    $source = Join-Path $root 'licenses/worldline'
    if (-not (Test-Path -LiteralPath $source -PathType Container)) {
        throw "Worldline license sources are missing: $source"
    }
    Copy-Item -LiteralPath $source -Destination (Join-Path $licenseRoot 'Worldline') -Recurse -Force
    Copy-Required (Join-Path $root 'third_party/world/LICENSE.txt') (Join-Path $licenseRoot 'WORLD/WORLD-LICENSE.txt')
    Copy-Required (Join-Path $root 'third_party/world/OOURA-NOTICE.txt') (Join-Path $licenseRoot 'WORLD/OOURA-NOTICE.txt')
    Copy-Required (Join-Path $root 'third_party/world/MACRODEFINITIONS-LICENSE.txt') (Join-Path $licenseRoot 'WORLD/MACRODEFINITIONS-LICENSE.txt')
}

function Copy-QtLicenses {
    $qtRoot = Resolve-QtRoot
    $qtVersion = (Get-Item -LiteralPath $qtRoot).Parent.Name
    $toolsRoot = [IO.Path]::GetFullPath((Join-Path $qtRoot '../../Tools'))
    $lgpl = Get-ChildItem -LiteralPath $toolsRoot -Recurse -File -Filter 'LGPLv3.txt' -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $lgpl) {
        throw "The Qt SDK did not provide an LGPLv3 license text: $toolsRoot"
    }
    Copy-Required $lgpl.FullName (Join-Path $licenseRoot 'Qt/LGPL-3.0.txt')

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
source request. The request must identify whether the request concerns Qt itself,
Qt Multimedia's FFmpeg deployment, or both. The project repository and its build
scripts provide the corresponding application source and relinking instructions.

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

The corresponding Qt source offer, LGPLv3 text, and Qt third-party attribution
information are included beside this file. The source request procedure is
specified in Qt-SOURCE-OFFER.txt.
"@
    Write-ReleaseText (Join-Path $licenseRoot 'Qt/Qt-RELINK-INSTRUCTIONS.txt') $qtRelinkInstructions

    $qtAttributions = @"
Qt $qtVersion third-party attributions
======================================

Qt's modules contain third-party components with their own copyright and license
terms. The authoritative attribution list for this Qt version is:

https://doc.qt.io/qt-6.8/licenses-used-in-qt.html

Qt Multimedia uses FFmpeg. The Qt Multimedia license and source guidance is:
https://doc.qt.io/qt-6.8/qtmultimedia-index.html
https://ffmpeg.org/legal.html

The Qt source offer and the LGPLv3 text are included beside this file.
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
        Copy-Required (Join-Path $mingwLicenseRoot $relativePath) (Join-Path $licenseRoot "MinGW/$([IO.Path]::GetFileName($relativePath))")
    }

    $ffmpegDlls = @(Get-ChildItem -LiteralPath (Join-Path $PackageRoot 'app') -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^(avcodec|avformat|avutil|swresample|swscale)-\d+\.dll$|ffmpeg' })
    if ($ffmpegDlls.Count -gt 0) {
        $dllList = ($ffmpegDlls | ForEach-Object {
            $relativePath = $_.FullName.Substring($PackageRoot.Length + 1)
            $sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash
            "$relativePath`n  SHA-256: $sha256"
        } | Sort-Object) -join "`n"
        $ffmpegNotice = @"
FFmpeg as deployed by Qt Multimedia
===================================

The GUI package contains the following FFmpeg-related files from the Qt
Multimedia deployment for Qt ${qtVersion}. Each SHA-256 value identifies the
exact binary covered by this notice:

$dllList

Qt's prebuilt FFmpeg configuration and the applicable license/source guidance
are documented by Qt here:
https://doc.qt.io/qt-6.8/qtmultimedia-index.html
https://doc.qt.io/qt-6.8/qtwebengine-3rdparty-ffmpeg.html
https://ffmpeg.org/legal.html

The corresponding Qt and FFmpeg source request procedure is identified in
Qt-SOURCE-OFFER.txt. The FFmpeg source/build must correspond to the exact files
listed above; a generic FFmpeg source tree is not a substitute for the matching
source.
"@
        Write-ReleaseText (Join-Path $licenseRoot 'Qt/FFmpeg-SOURCE-AND-LICENSE.txt') $ffmpegNotice
    }
}

function Copy-CudaLicense {
    $nvcc = (Get-Command nvcc -ErrorAction Stop).Source
    $cudaRoot = Split-Path -Parent (Split-Path -Parent $nvcc)
    Copy-Required (Join-Path $cudaRoot 'EULA.txt') (Join-Path $licenseRoot 'CUDA/CUDA-EULA.txt')
    $cudaLicense = Join-Path $cudaRoot 'LICENSE'
    if (Test-Path -LiteralPath $cudaLicense -PathType Leaf) {
        Copy-Required $cudaLicense (Join-Path $licenseRoot 'CUDA/CUDA-LICENSE.txt')
    }
    $version = (& $nvcc '--version' | Out-String).Trim()
    $cudaNotice = @"
CUDA renderer build provenance
=============================

The optional faithful GPU renderer's CUDA support was built with nvcc at:
$nvcc

nvcc version output:
$version

The renderer uses the statically linked CUDA runtime (-cudart static). The
applicable NVIDIA CUDA Toolkit terms are included in CUDA-EULA.txt. No NVIDIA
GPU driver is distributed by this package.
"@
    Write-ReleaseText (Join-Path $licenseRoot 'CUDA/CUDA-BUILD.txt') $cudaNotice
}

New-Item -ItemType Directory -Force -Path $licenseRoot | Out-Null
Copy-GoLicenses
Copy-OpenJTalkLicenses
Copy-ProsodyDataProvenance
Copy-WorldlineLicenses

if ($Variant -eq 'windows-gui') {
    Copy-QtLicenses
}

if ($CudaIncluded) {
    Copy-CudaLicense
}

$manifest = @(
    'This directory contains license and notice files copied from the exact',
    'SDK/package/toolchain versions used to assemble this release.',
    '',
    'The project-wide summary is ../THIRD_PARTY_NOTICES.txt.'
)
$manifest += @(Get-ChildItem -LiteralPath $licenseRoot -Recurse -File | ForEach-Object {
    $_.FullName.Substring($PackageRoot.Length + 1)
} | Sort-Object)
Write-ReleaseText (Join-Path $licenseRoot 'README.txt') ($manifest -join "`n")

Write-Host "Collected third-party licenses for $Variant at $licenseRoot"

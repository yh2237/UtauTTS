param(
    [string]$DestinationDirectory = (Join-Path (Split-Path $PSScriptRoot -Parent) 'models/english-reference')
)

# 英語の合成で同梱するモデルを、版を固定した配布元から取得してハッシュを確かめる。
$ErrorActionPreference = 'Stop'
$kokoro = 'https://huggingface.co/hexgrad/Kokoro-82M/resolve/f3ff3571791e39611d31c381e3a41a3af07b4987'
$hubert = 'https://huggingface.co/facebook/hubert-base-ls960/resolve/af46f65f540dc3ca7aa59f46c6c3d5dbb4374fa8'
$wavlm = 'https://huggingface.co/microsoft/wavlm-base-plus/resolve/4c66d4806a428f2e922ccfa1a962776e232d487b'
$misaki = 'https://raw.githubusercontent.com/hexgrad/misaki/fba1236595f2d2bf21d414ba6e57d25256afada3/misaki/data'
$files = @(
    @{ Name = 'kokoro-v1_0.pth'; Url = "$kokoro/kokoro-v1_0.pth"; Sha256 = '496dba118d1a58f5f3db2efc88dbdc216e0483fc89fe6e47ee1f2c53f18ad1e4' },
    @{ Name = 'config.json'; Url = "$kokoro/config.json"; Sha256 = '5abb01e2403b072bf03d04fde160443e209d7a0dad49a423be15196b9b43c17f' },
    @{ Name = 'af_heart.pt'; Url = "$kokoro/voices/af_heart.pt"; Sha256 = '0ab5709b8ffab19bfd849cd11d98f75b60af7733253ad0d67b12382a102cb4ff' },
    @{ Name = 'hubert-base-ls960.safetensors'; Url = "$hubert/model.safetensors"; Sha256 = 'ec18ca2453e7cf3de5e4fc5b79e7484b27506ed0ad684282ddd10ecb9492f37f' },
    @{ Name = 'us_gold.json'; Url = "$misaki/us_gold.json"; Sha256 = 'dc414872a49a28ae6c141463d502fd945f3b2fde040484fdc47d00cc4612686f' },
    @{ Name = 'us_silver.json'; Url = "$misaki/us_silver.json"; Sha256 = 'de8f67be911bb6c659187b4a65fd966b6a30e56350e0f790d763210b053ac475' }
)

New-Item -ItemType Directory -Force -Path $DestinationDirectory | Out-Null
foreach ($file in $files) {
    $destination = Join-Path $DestinationDirectory $file.Name
    if ((Test-Path -LiteralPath $destination -PathType Leaf) -and
            (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash -eq $file.Sha256) {
        continue
    }
    $temporary = "$destination.download"
    Write-Host "Downloading $($file.Url)"
    Invoke-WebRequest -Uri $file.Url -OutFile $temporary -UseBasicParsing
    $hash = (Get-FileHash -LiteralPath $temporary -Algorithm SHA256).Hash
    if ($hash -ne $file.Sha256) {
        Remove-Item -LiteralPath $temporary -Force
        throw "SHA256 mismatch for $($file.Name): $hash"
    }
    Move-Item -LiteralPath $temporary -Destination $destination -Force
}

# WavLM is distributed as pytorch_model.bin, so download it and convert it to
# safetensors when the converted file is not there yet.
$wavlmDirectory = Join-Path $DestinationDirectory 'wavlm'
New-Item -ItemType Directory -Force -Path $wavlmDirectory | Out-Null
$safetensors = Join-Path $wavlmDirectory 'wavlm-base-plus.safetensors'
$config = @{ Name = 'config.json'; Url = "$wavlm/config.json"; Sha256 = 'fea6df1c2700a3954fc07e70588aecc9055eeb28db2ff57151a2db0d19180ed4' }
$wavlmFiles = @()
if (-not (Test-Path -LiteralPath $safetensors -PathType Leaf)) {
    $wavlmFiles += @{ Name = 'pytorch_model.bin'; Url = "$wavlm/pytorch_model.bin"; Sha256 = '3bb273a6ace99408b50cfc81afdbb7ef2de02da2eab0234e18db608ce692fe51' }
}
$wavlmFiles += $config
foreach ($file in $wavlmFiles) {
    $destination = Join-Path $wavlmDirectory $file.Name
    if ((Test-Path -LiteralPath $destination -PathType Leaf) -and
            (Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash -eq $file.Sha256) {
        continue
    }
    $temporary = "$destination.download"
    Write-Host "Downloading $($file.Url)"
    Invoke-WebRequest -Uri $file.Url -OutFile $temporary -UseBasicParsing
    $hash = (Get-FileHash -LiteralPath $temporary -Algorithm SHA256).Hash
    if ($hash -ne $file.Sha256) {
        Remove-Item -LiteralPath $temporary -Force
        throw "SHA256 mismatch for $($file.Name): $hash"
    }
    Move-Item -LiteralPath $temporary -Destination $destination -Force
}
$bin = Join-Path $wavlmDirectory 'pytorch_model.bin'
if (-not (Test-Path -LiteralPath $safetensors -PathType Leaf)) {
    Write-Host "Converting $bin to safetensors"
    python -c "import torch; from safetensors.torch import save_file; state=torch.load(r'$bin', map_location='cpu', weights_only=True); save_file({k: v.contiguous() for k, v in state.items()}, r'$safetensors')"
    if ($LASTEXITCODE -ne 0) {
        throw 'WavLM conversion failed: Python with torch, numpy, and safetensors is required.'
    }
}
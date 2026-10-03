$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectRoot
$previousOS = $env:GOOS
$previousArch = $env:GOARCH
try {
    $env:GOOS = 'js'
    $env:GOARCH = 'wasm'
    go build -buildvcs=false -trimpath -ldflags='-s -w' -o web/static/convert.wasm ./cmd/wasm
    if ($LASTEXITCODE -ne 0) { throw 'WASM build failed' }
    $goRoot = go env GOROOT
    $runtime = Join-Path $goRoot 'lib/wasm/wasm_exec.js'
    if (-not (Test-Path -LiteralPath $runtime)) { $runtime = Join-Path $goRoot 'misc/wasm/wasm_exec.js' }
    Copy-Item -LiteralPath $runtime -Destination web/static/wasm_exec.js
    $target = [System.IO.File]::Create((Join-Path $projectRoot 'web/static/convert.wasm.gz'))
    try {
        $gzip = [System.IO.Compression.GZipStream]::new($target, [System.IO.Compression.CompressionLevel]::SmallestSize)
        try {
            $source = [System.IO.File]::OpenRead((Join-Path $projectRoot 'web/static/convert.wasm'))
            try { $source.CopyTo($gzip) } finally { $source.Dispose() }
        } finally { $gzip.Dispose() }
    } finally { $target.Dispose() }
    $env:GOOS = $previousOS
    $env:GOARCH = $previousArch
    go build -trimpath -o armour2bbmodel.exe .
    if ($LASTEXITCODE -ne 0) { throw 'Server build failed' }
    go build -trimpath -o armour-convert.exe ./cmd/convert
    if ($LASTEXITCODE -ne 0) { throw 'CLI build failed' }
    if ($IsWindows -and [string]$env:GOOS -in @('', 'windows')) {
        & (Join-Path $projectRoot 'scripts/set-exe-icon.ps1') -LiteralPath (Join-Path $projectRoot 'armour2bbmodel.exe') -IconPath (Join-Path $projectRoot 'branding/armour2bbmodel.ico')
    }
} finally {
    $env:GOOS = $previousOS
    $env:GOARCH = $previousArch
    Pop-Location
}

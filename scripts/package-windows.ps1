# Copyright 2026 17Artist. Licensed under the project's LICENSE.
#Requires -Version 7.3

[CmdletBinding()]
param(
    [ValidateNotNullOrEmpty()]
    [string]$OutputDirectory = 'dist'
)

$ErrorActionPreference = 'Stop'
if (-not $IsWindows) { throw 'Windows 发布包的 EXE 图标写入需要在 Windows 上执行。' }
$releaseProjectRoot = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$releaseOutputRoot = if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    [System.IO.Path]::GetFullPath($OutputDirectory)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $releaseProjectRoot $OutputDirectory))
}

$releaseStem = 'Armour2BBModel-Windows-x64'
$releaseExePath = Join-Path $releaseOutputRoot "$releaseStem.exe"
$releaseZipPath = Join-Path $releaseOutputRoot "$releaseStem.zip"
$releaseHashPath = Join-Path $releaseOutputRoot "$releaseStem.sha256"
$releaseGuidePath = Join-Path $releaseOutputRoot '使用说明.txt'
$releaseIconPath = Join-Path $releaseProjectRoot 'branding/armour2bbmodel.ico'
$releaseIconScript = Join-Path $releaseProjectRoot 'scripts/set-exe-icon.ps1'

foreach ($requiredIconPath in @($releaseIconPath, $releaseIconScript)) {
    if (-not (Test-Path -LiteralPath $requiredIconPath -PathType Leaf)) {
        throw "缺少发布图标或图标脚本：$requiredIconPath"
    }
}

Get-Command go -ErrorAction Stop | Out-Null
$releaseLicensePath = Join-Path $releaseProjectRoot 'LICENSE'
$releaseNoticesPath = Join-Path $releaseProjectRoot 'THIRD_PARTY_NOTICES.md'
$releaseLicensesRoot = Join-Path $releaseProjectRoot 'licenses'
foreach ($requiredPath in @($releaseLicensePath, $releaseNoticesPath)) {
    if (-not (Test-Path -LiteralPath $requiredPath -PathType Leaf)) {
        throw "缺少发布声明文件：$requiredPath"
    }
}
if (-not (Test-Path -LiteralPath $releaseLicensesRoot -PathType Container)) {
    throw "缺少第三方许可证目录：$releaseLicensesRoot"
}
$releaseLicenseFiles = @(Get-ChildItem -LiteralPath $releaseLicensesRoot -File -Recurse | Sort-Object FullName)
if ($releaseLicenseFiles.Count -eq 0) {
    throw '第三方许可证目录没有文件，不能创建发布包。'
}

$releasePreviousEnvironment = @{}
foreach ($variableName in @('GOOS', 'GOARCH', 'CGO_ENABLED')) {
    $releasePreviousEnvironment[$variableName] = [System.Environment]::GetEnvironmentVariable($variableName, 'Process')
}
$releaseLocationPushed = $false
try {
    Push-Location -LiteralPath $releaseProjectRoot
    $releaseLocationPushed = $true
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'

    # Rebuild the conversion engine and its matching runtime before embedding.
    # Never package an executable copied from a previous build.
    Write-Host '正在刷新 WASM、runtime 和原生程序...'
    & (Join-Path $releaseProjectRoot 'scripts/build.ps1')
    if ($LASTEXITCODE -ne 0) {
        throw '发布前资源构建失败。'
    }
    foreach ($assetName in @('index.html', 'studio.css', 'studio.js', 'preview.js', 'converter-worker.js', 'convert.wasm', 'convert.wasm.gz', 'wasm_exec.js', 'favicon.svg')) {
        $assetPath = Join-Path $releaseProjectRoot "web/static/$assetName"
        if (-not (Test-Path -LiteralPath $assetPath -PathType Leaf) -or (Get-Item -LiteralPath $assetPath).Length -eq 0) {
            throw "发布资源缺失或为空：$assetPath"
        }
    }

    [void][System.IO.Directory]::CreateDirectory($releaseOutputRoot)
    Write-Host '正在构建 Windows x64 独立 EXE...'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    go build -trimpath -buildvcs=false -ldflags='-s -w' -o $releaseExePath .
    if ($LASTEXITCODE -ne 0) {
        throw 'Windows x64 发布程序构建失败。'
    }
    & $releaseIconScript -LiteralPath $releaseExePath -IconPath $releaseIconPath

    $releaseGuide = @'
Armour2BBModel Windows x64 使用说明

1. 将 ZIP 完整解压，双击 Armour2BBModel-Windows-x64.exe。
2. 程序会打开默认浏览器。选择或拖入自己的 .armour / .awsk 文件后转换、预览并下载 .bbmodel。
3. 程序只监听 127.0.0.1。默认使用 8080；默认端口被占用时会换用空闲端口，以控制台显示的地址为准。
4. 使用期间保留控制台窗口；结束时按 Ctrl+C 或关闭窗口。

浏览器未自动打开时，将控制台显示的地址复制到现代浏览器。
关闭自动打开浏览器：Armour2BBModel-Windows-x64.exe -no-browser
指定端口：Armour2BBModel-Windows-x64.exe -port 8081

EXE 内嵌网页、转换引擎和运行时，不需要另外的 web 目录、Go、Node.js 或 Python。
运行和转换可以离线使用，时装内容不会上传。需有现代浏览器用于查看界面；三维预览使用浏览器 WebGL。
程序不附带时装素材，请导入自己的文件。

项目许可和署名见 LICENSE；第三方声明见 THIRD_PARTY_NOTICES.md 及 licenses 目录。
'@
    [System.IO.File]::WriteAllText($releaseGuidePath, $releaseGuide, [System.Text.UTF8Encoding]::new($false))

    $releaseArchiveFiles = @(
        @{ Path = $releaseExePath; Name = "$releaseStem.exe" },
        @{ Path = $releaseGuidePath; Name = '使用说明.txt' },
        @{ Path = $releaseLicensePath; Name = 'LICENSE' },
        @{ Path = $releaseNoticesPath; Name = 'THIRD_PARTY_NOTICES.md' }
    )
    foreach ($licenseFile in $releaseLicenseFiles) {
        $archiveName = [System.IO.Path]::GetRelativePath($releaseProjectRoot, $licenseFile.FullName).Replace('\', '/')
        $releaseArchiveFiles += @{ Path = $licenseFile.FullName; Name = $archiveName }
    }

    # Write the known release files directly. No staging directory or recursive
    # deletion is needed, and the source license files remain byte-for-byte intact.
    $releaseArchiveStream = [System.IO.File]::Open($releaseZipPath, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
    try {
        $releaseArchive = [System.IO.Compression.ZipArchive]::new($releaseArchiveStream, [System.IO.Compression.ZipArchiveMode]::Create, $true)
        try {
            foreach ($archiveFile in $releaseArchiveFiles) {
                $entry = $releaseArchive.CreateEntry($archiveFile.Name, [System.IO.Compression.CompressionLevel]::Optimal)
                $entryStream = $entry.Open()
                try {
                    $inputStream = [System.IO.File]::OpenRead($archiveFile.Path)
                    try { $inputStream.CopyTo($entryStream) } finally { $inputStream.Dispose() }
                } finally { $entryStream.Dispose() }
            }
        } finally { $releaseArchive.Dispose() }
    } finally { $releaseArchiveStream.Dispose() }

    $releaseHashLines = foreach ($artifactPath in @($releaseExePath, $releaseZipPath)) {
        $digest = (Get-FileHash -LiteralPath $artifactPath -Algorithm SHA256).Hash.ToLowerInvariant()
        "$digest  $([System.IO.Path]::GetFileName($artifactPath))"
    }
    [System.IO.File]::WriteAllLines($releaseHashPath, [string[]]$releaseHashLines, [System.Text.UTF8Encoding]::new($false))

    Write-Host 'Windows 离线发布包已生成：'
    Write-Host "EXE    $releaseExePath"
    Write-Host "ZIP    $releaseZipPath"
    Write-Host "SHA256 $releaseHashPath"
    foreach ($hashLine in $releaseHashLines) { Write-Host $hashLine }
} finally {
    foreach ($variableName in @('GOOS', 'GOARCH', 'CGO_ENABLED')) {
        [System.Environment]::SetEnvironmentVariable($variableName, $releasePreviousEnvironment[$variableName], 'Process')
    }
    if ($releaseLocationPushed) { Pop-Location }
}

# Copyright 2026 17Artist. Licensed under the project's LICENSE.
#Requires -Version 7.3

[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateNotNullOrEmpty()]
    [string]$LiteralPath,

    [Parameter(Mandatory)]
    [ValidateNotNullOrEmpty()]
    [string]$IconPath
)

$ErrorActionPreference = 'Stop'
if (-not $IsWindows) { throw 'EXE 图标更新需要 Windows 内置资源 API。' }
if (-not (Test-Path -LiteralPath $LiteralPath -PathType Leaf)) { throw "EXE 不存在：$LiteralPath" }
if (-not (Test-Path -LiteralPath $IconPath -PathType Leaf)) { throw "ICO 不存在：$IconPath" }
$iconTarget = (Get-Item -LiteralPath $LiteralPath).FullName
$iconSource = (Get-Item -LiteralPath $IconPath).FullName
if ([System.IO.Path]::GetExtension($iconTarget) -ine '.exe') { throw '图标目标必须是 .exe 文件。' }

# Reject non-PE files before opening a Windows resource update transaction.
$peStream = [System.IO.File]::OpenRead($iconTarget)
try {
    $dosHeader = [byte[]]::new(64)
    if ($peStream.Read($dosHeader, 0, $dosHeader.Length) -ne 64 -or $dosHeader[0] -ne 0x4D -or $dosHeader[1] -ne 0x5A) {
        throw '目标没有有效的 MZ 可执行文件头。'
    }
    $peOffset = [System.BitConverter]::ToInt32($dosHeader, 60)
    if ($peOffset -lt 64 -or [long]$peOffset + 24 -gt $peStream.Length) { throw '目标 PE 文件头位置无效。' }
    [void]$peStream.Seek($peOffset, [System.IO.SeekOrigin]::Begin)
    $peHeader = [byte[]]::new(24)
    if ($peStream.Read($peHeader, 0, $peHeader.Length) -ne 24 -or $peHeader[0] -ne 0x50 -or $peHeader[1] -ne 0x45 -or $peHeader[2] -ne 0 -or $peHeader[3] -ne 0) {
        throw '目标没有有效的 PE 文件头。'
    }
    $peCharacteristics = [System.BitConverter]::ToUInt16($peHeader, 22)
    if (($peCharacteristics -band 0x0002) -eq 0 -or ($peCharacteristics -band 0x2000) -ne 0) {
        throw '目标必须是 PE 可执行程序，不能是 DLL。'
    }
} finally { $peStream.Dispose() }

$ico = [System.IO.File]::ReadAllBytes($iconSource)
if ($ico.Length -lt 6 -or [System.BitConverter]::ToUInt16($ico, 0) -ne 0 -or [System.BitConverter]::ToUInt16($ico, 2) -ne 1) {
    throw '图标必须是有效的 ICO 文件，不能是 CUR 或其他图片格式。'
}
$imageCount = [int][System.BitConverter]::ToUInt16($ico, 4)
if ($imageCount -lt 1 -or $imageCount -gt 256 -or $ico.Length -lt 6 + 16 * $imageCount) {
    throw 'ICO 图像数量或目录大小无效。'
}
$iconImages = [System.Collections.Generic.List[byte[]]]::new()
$groupStream = [System.IO.MemoryStream]::new()
try {
    $groupWriter = [System.IO.BinaryWriter]::new($groupStream, [System.Text.Encoding]::UTF8, $true)
    try {
        $groupWriter.Write([uint16]0)
        $groupWriter.Write([uint16]1)
        $groupWriter.Write([uint16]$imageCount)
        for ($imageIndex = 0; $imageIndex -lt $imageCount; $imageIndex++) {
            $directoryOffset = 6 + 16 * $imageIndex
            $imageSize = [System.BitConverter]::ToUInt32($ico, $directoryOffset + 8)
            $imageOffset = [System.BitConverter]::ToUInt32($ico, $directoryOffset + 12)
            if ($ico[$directoryOffset + 3] -ne 0 -or $imageSize -eq 0 -or $imageOffset -lt 6 + 16 * $imageCount -or [uint64]$imageOffset + [uint64]$imageSize -gt [uint64]$ico.Length) {
                throw "ICO 第 $($imageIndex + 1) 张图像的目录或数据范围无效。"
            }
            $image = [byte[]]::new([int]$imageSize)
            [System.Buffer]::BlockCopy($ico, [int]$imageOffset, $image, 0, $image.Length)
            $iconImages.Add($image)

            # GRPICONDIRENTRY keeps the ICO dimensions, planes and bit depth;
            # its last field is a WORD resource ID instead of a DWORD offset.
            for ($fieldIndex = 0; $fieldIndex -lt 4; $fieldIndex++) {
                $groupWriter.Write([byte]$ico[$directoryOffset + $fieldIndex])
            }
            $groupWriter.Write([uint16][System.BitConverter]::ToUInt16($ico, $directoryOffset + 4))
            $groupWriter.Write([uint16][System.BitConverter]::ToUInt16($ico, $directoryOffset + 6))
            $groupWriter.Write([uint32]$imageSize)
            $groupWriter.Write([uint16]($imageIndex + 1))
        }
        $groupWriter.Flush()
        $groupData = $groupStream.ToArray()
    } finally { $groupWriter.Dispose() }
} finally { $groupStream.Dispose() }

if (-not ('Armour2BBModel.Packaging.IconResources' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

namespace Armour2BBModel.Packaging {
    public static class IconResources {
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, ExactSpelling = true, SetLastError = true)]
        public static extern IntPtr BeginUpdateResourceW(string fileName,
            [MarshalAs(UnmanagedType.Bool)] bool deleteExistingResources);

        [DllImport("kernel32.dll", ExactSpelling = true, SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool UpdateResourceW(IntPtr updateHandle, IntPtr type, IntPtr name,
            ushort language, [In, MarshalAs(UnmanagedType.LPArray, SizeParamIndex = 5)] byte[] data,
            uint size);

        [DllImport("kernel32.dll", ExactSpelling = true, SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool EndUpdateResourceW(IntPtr updateHandle,
            [MarshalAs(UnmanagedType.Bool)] bool discard);
    }
}
'@
}

# Preserve all existing non-icon resources. On any failed update, discard the
# transaction; only the explicitly supplied executable is ever written.
$updateHandle = [Armour2BBModel.Packaging.IconResources]::BeginUpdateResourceW($iconTarget, $false)
if ($updateHandle -eq [System.IntPtr]::Zero) {
    $errorCode = [System.Runtime.InteropServices.Marshal]::GetLastWin32Error()
    throw [System.ComponentModel.Win32Exception]::new($errorCode, "无法开始 EXE 图标更新：$iconTarget")
}
try {
    for ($imageIndex = 0; $imageIndex -lt $imageCount; $imageIndex++) {
        $image = $iconImages[$imageIndex]
        $ok = [Armour2BBModel.Packaging.IconResources]::UpdateResourceW($updateHandle, [System.IntPtr]::new(3), [System.IntPtr]::new($imageIndex + 1), [uint16]0, $image, [uint32]$image.Length)
        if (-not $ok) {
            $errorCode = [System.Runtime.InteropServices.Marshal]::GetLastWin32Error()
            throw [System.ComponentModel.Win32Exception]::new($errorCode, "写入 RT_ICON $($imageIndex + 1) 失败。")
        }
    }
    $ok = [Armour2BBModel.Packaging.IconResources]::UpdateResourceW($updateHandle, [System.IntPtr]::new(14), [System.IntPtr]::new(1), [uint16]0, $groupData, [uint32]$groupData.Length)
    if (-not $ok) {
        $errorCode = [System.Runtime.InteropServices.Marshal]::GetLastWin32Error()
        throw [System.ComponentModel.Win32Exception]::new($errorCode, '写入主 RT_GROUP_ICON 失败。')
    }
    $ok = [Armour2BBModel.Packaging.IconResources]::EndUpdateResourceW($updateHandle, $false)
    $errorCode = [System.Runtime.InteropServices.Marshal]::GetLastWin32Error()
    $updateHandle = [System.IntPtr]::Zero
    if (-not $ok) { throw [System.ComponentModel.Win32Exception]::new($errorCode, '提交 EXE 图标更新失败。') }
} finally {
    if ($updateHandle -ne [System.IntPtr]::Zero) {
        [void][Armour2BBModel.Packaging.IconResources]::EndUpdateResourceW($updateHandle, $true)
    }
}
Write-Host "已写入 $imageCount 个尺寸的 EXE 图标：$iconTarget"

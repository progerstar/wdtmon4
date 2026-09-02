[CmdletBinding()]
param(
    [string] $DistDir = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
$AppName = "wdtmon4"

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)] [string] $Executable,
        [Parameter(Mandatory = $true)] [string[]] $Arguments,
        [Parameter(Mandatory = $true)] [string] $WorkingDirectory
    )

    Push-Location $WorkingDirectory
    try {
        & $Executable @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "Command failed with exit code ${LASTEXITCODE}: $Executable $($Arguments -join ' ')"
        }
    }
    finally {
        Pop-Location
    }
}

function Get-AppVersion {
    $Source = Get-Content -Raw -LiteralPath (Join-Path $RootDir "main.go")
    $Match = [regex]::Match(
        $Source,
        '(?m)^\s*var\s+VERSION\s*=\s*"([^"]+)"'
    )
    if (-not $Match.Success -or $Match.Groups[1].Value -notmatch '^\d+(\.\d+){1,2}$') {
        throw "Could not read a valid application version from main.go"
    }
    return $Match.Groups[1].Value
}

foreach ($CommandName in @("go", "node", "npm")) {
    if (-not (Get-Command $CommandName -ErrorAction SilentlyContinue)) {
        throw "Required command was not found: $CommandName"
    }
}

$Version = Get-AppVersion
if ([string]::IsNullOrWhiteSpace($DistDir)) {
    $DistDir = Join-Path $RootDir "dist\windows"
}
elseif (-not [IO.Path]::IsPathRooted($DistDir)) {
    $DistDir = Join-Path $RootDir $DistDir
}

if ($env:SKIP_WEB_BUILD -ne "1") {
    Write-Host "==> Building the web interface"
    Invoke-Checked -Executable "npm" -Arguments @("ci", "--no-audit", "--no-fund") `
        -WorkingDirectory (Join-Path $RootDir "web")
    Invoke-Checked -Executable "npm" -Arguments @("run", "build") `
        -WorkingDirectory (Join-Path $RootDir "web")
}

$WebIndex = Join-Path $RootDir "web\build\index.html"
if (-not (Test-Path -LiteralPath $WebIndex -PathType Leaf)) {
    throw "Vite did not create web/build/index.html"
}

$BuildDir = Join-Path $RootDir "build\windows-x86_64"
New-Item -ItemType Directory -Force -Path $BuildDir, $DistDir | Out-Null
$Binary = Join-Path $BuildDir "$AppName.exe"

$PreviousGoos = $env:GOOS
$PreviousGoarch = $env:GOARCH
$PreviousCgoEnabled = $env:CGO_ENABLED
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    Write-Host "==> Building $AppName $Version for windows/amd64"
    Invoke-Checked -Executable "go" -Arguments @(
        "build",
        "-trimpath",
        "-buildvcs=false",
        "-ldflags", "-s -w -X main.VERSION=$Version",
        "-o", $Binary,
        "."
    ) -WorkingDirectory $RootDir
}
finally {
    $env:GOOS = $PreviousGoos
    $env:GOARCH = $PreviousGoarch
    $env:CGO_ENABLED = $PreviousCgoEnabled
}

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    throw "Windows executable was not created: $Binary"
}
$VersionOutput = (& $Binary --version | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $VersionOutput -ne "$AppName v$Version") {
    throw "Unexpected version output: $VersionOutput"
}

$StagingDir = Join-Path ([IO.Path]::GetTempPath()) (
    "wdtmon4-windows-" + [guid]::NewGuid().ToString("N")
)
$ArchiveName = "$AppName-$Version-windows-x86_64.zip"
$ArchivePath = Join-Path $DistDir $ArchiveName
$TemporaryArchive = Join-Path $DistDir ".$ArchiveName"

try {
    New-Item -ItemType Directory -Path $StagingDir | Out-Null
    Copy-Item -LiteralPath $Binary -Destination $StagingDir
    Copy-Item -LiteralPath (Join-Path $RootDir "LICENSE") -Destination $StagingDir
    Copy-Item -LiteralPath (Join-Path $RootDir "README.md") -Destination $StagingDir

    Remove-Item -Force -ErrorAction SilentlyContinue -LiteralPath $TemporaryArchive
    Compress-Archive -Path (Join-Path $StagingDir "*") `
        -DestinationPath $TemporaryArchive -CompressionLevel Optimal
    Move-Item -Force -LiteralPath $TemporaryArchive -Destination $ArchivePath
}
finally {
    if (Test-Path -LiteralPath $StagingDir) {
        Remove-Item -Recurse -Force -LiteralPath $StagingDir
    }
    Remove-Item -Force -ErrorAction SilentlyContinue -LiteralPath $TemporaryArchive
}

$Hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArchivePath).Hash.ToLowerInvariant()
$ChecksumPath = "$ArchivePath.sha256"
$ChecksumLine = "$Hash  $ArchiveName`n"
[IO.File]::WriteAllText(
    $ChecksumPath,
    $ChecksumLine,
    [Text.ASCIIEncoding]::new()
)

Write-Host "Created $ArchivePath"
Write-Host "Created $ChecksumPath"

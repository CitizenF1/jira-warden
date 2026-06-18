param(
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,

        [Parameter(Mandatory = $true)]
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Copy-IfExists {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Source,

        [Parameter(Mandatory = $true)]
        [string]$Destination
    )

    if (Test-Path $Source) {
        Copy-Item $Source $Destination -Force
    }
}

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

Invoke-Native -FilePath "go" -Arguments @("test", "./...")

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; EXT = ".exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; EXT = "" },
    @{ GOOS = "darwin"; GOARCH = "amd64"; EXT = "" },
    @{ GOOS = "darwin"; GOARCH = "arm64"; EXT = "" }
)

New-Item -ItemType Directory -Force -Path "dist" | Out-Null

try {
    foreach ($target in $targets) {
        $name = "jirawarden-$Version-$($target.GOOS)-$($target.GOARCH)"
        $output = "dist/$name/jirawarden$($target.EXT)"

        New-Item -ItemType Directory -Force -Path "dist/$name" | Out-Null

        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        Invoke-Native -FilePath "go" -Arguments @(
            "build",
            "-ldflags", "-s -w -X main.version=$Version",
            "-o", $output,
            "./cmd/jirawarden"
        )

        Copy-Item "README.md" "dist/$name/README.md" -Force
        Copy-IfExists "RELEASE.md" "dist/$name/RELEASE.md"
        Copy-Item ".env.example" "dist/$name/.env.example" -Force

        if ($target.GOOS -eq "windows") {
            Compress-Archive -Path "dist/$name/*" -DestinationPath "dist/$name.zip" -Force
        } else {
            Invoke-Native -FilePath "tar" -Arguments @(
                "-C", "dist/$name",
                "-czf", "dist/$name.tar.gz",
                "."
            )
        }
    }
} finally {
    if ($null -eq $oldGOOS) {
        Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    } else {
        $env:GOOS = $oldGOOS
    }

    if ($null -eq $oldGOARCH) {
        Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    } else {
        $env:GOARCH = $oldGOARCH
    }
}

Write-Host "Release files are in dist/"

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

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

try {
    Invoke-Native -FilePath "go" -Arguments @("test", "./...")

    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
    Invoke-Native -FilePath "go" -Arguments @(
        "build",
        "-ldflags", "-s -w -X main.version=$Version",
        "-o", "bin/jirawarden.exe",
        "./cmd/jirawarden"
    )
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

Write-Host "Built bin/jirawarden.exe"

param(
    [Parameter(Mandatory = $true)]
    [string]$Executable,

    [string]$Description = 'GTK runtime self-test'
)

$ErrorActionPreference = 'Stop'

$resolvedExecutable = (Resolve-Path -LiteralPath $Executable).Path
$temporaryRoot = $env:RUNNER_TEMP
if ([string]::IsNullOrWhiteSpace($temporaryRoot)) {
    $temporaryRoot = [System.IO.Path]::GetTempPath()
}
$runID = [guid]::NewGuid()
$stdout = Join-Path $temporaryRoot "e9s-runtime-self-test-$runID.out"
$stderr = Join-Path $temporaryRoot "e9s-runtime-self-test-$runID.err"

try {
    # e9s-gui.exe uses the Windows GUI subsystem. PowerShell's call operator
    # does not reliably update LASTEXITCODE for GUI-subsystem applications, so
    # wait for the process explicitly and inspect its ExitCode instead.
    $process = Start-Process `
        -FilePath $resolvedExecutable `
        -ArgumentList '--self-test' `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -Wait `
        -PassThru

    if (Test-Path -LiteralPath $stdout) {
        Get-Content -LiteralPath $stdout | Write-Host
    }
    if (Test-Path -LiteralPath $stderr) {
        Get-Content -LiteralPath $stderr | Write-Host
    }
    if ($process.ExitCode -ne 0) {
        throw "$Description failed with exit code $($process.ExitCode)"
    }
} finally {
    Remove-Item -LiteralPath $stdout, $stderr -Force -ErrorAction SilentlyContinue
}

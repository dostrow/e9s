[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $Path,

    [Parameter(Mandatory = $true)]
    [string] $CertificatePath,

    [Parameter(Mandatory = $true)]
    [string] $CertificatePassword,

    [Parameter(Mandatory = $false)]
    [string] $TimestampUrl = 'http://timestamp.digicert.com'
)

$ErrorActionPreference = 'Stop'
$artifact = [System.IO.Path]::GetFullPath($Path)
$pfx = [System.IO.Path]::GetFullPath($CertificatePath)

if (-not (Test-Path -LiteralPath $artifact -PathType Leaf)) {
    throw "Artifact to sign was not found: $artifact"
}
if (-not (Test-Path -LiteralPath $pfx -PathType Leaf)) {
    throw "Code-signing certificate was not found: $pfx"
}

$signTool = Get-Command 'signtool.exe' -ErrorAction SilentlyContinue |
    Select-Object -First 1 -ExpandProperty Source
if ([string]::IsNullOrWhiteSpace($signTool)) {
    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    if (Test-Path -LiteralPath $sdkRoot -PathType Container) {
        $signTool = Get-ChildItem -LiteralPath $sdkRoot -Recurse -Filter 'signtool.exe' |
            Where-Object { $_.FullName -match '\\x64\\signtool\.exe$' } |
            Sort-Object FullName -Descending |
            Select-Object -First 1 -ExpandProperty FullName
    }
}
if ([string]::IsNullOrWhiteSpace($signTool) -or -not (Test-Path -LiteralPath $signTool -PathType Leaf)) {
    throw 'signtool.exe was not found; install the Windows SDK signing tools'
}

$securePassword = ConvertTo-SecureString $CertificatePassword -AsPlainText -Force
$pfxData = Get-PfxData -FilePath $pfx -Password $securePassword
$signingCertificate = $pfxData.EndEntityCertificates |
    Select-Object -First 1
if ($null -eq $signingCertificate) {
    throw 'The PFX does not contain an end-entity certificate'
}

$thumbprint = $signingCertificate.Thumbprint
$storePath = "Cert:\CurrentUser\My\$thumbprint"
$importedForSigning = $false
try {
    if (-not (Test-Path -LiteralPath $storePath)) {
        Import-PfxCertificate -FilePath $pfx -CertStoreLocation 'Cert:\CurrentUser\My' `
            -Password $securePassword | Out-Null
        $importedForSigning = $true
    }
    if (-not (Test-Path -LiteralPath $storePath)) {
        throw "The signing certificate was not imported into the current-user store: $thumbprint"
    }
    $storedCertificate = Get-Item -LiteralPath $storePath
    if (-not $storedCertificate.HasPrivateKey) {
        throw 'The imported end-entity certificate does not have a private key'
    }

    & $signTool sign /s My /sha1 $thumbprint /fd SHA256 /tr $TimestampUrl /td SHA256 /v $artifact
    if ($LASTEXITCODE -ne 0) {
        throw "SignTool failed to sign $artifact with exit code $LASTEXITCODE"
    }

    & $signTool verify /pa /v $artifact
    if ($LASTEXITCODE -ne 0) {
        throw "SignTool could not verify $artifact with exit code $LASTEXITCODE"
    }
} finally {
    if ($importedForSigning -and (Test-Path -LiteralPath $storePath)) {
        Remove-Item -LiteralPath $storePath -Force
    }
}

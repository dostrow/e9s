[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $Version,

    [Parameter(Mandatory = $false)]
    [string] $BundleDirectory,

    [Parameter(Mandatory = $false)]
    [string] $Archive,

    [Parameter(Mandatory = $true)]
    [string] $OutputDirectory,

    [Parameter(Mandatory = $false)]
    [string] $ISCC
)

$ErrorActionPreference = 'Stop'
$scriptDirectory = Split-Path -Parent $MyInvocation.MyCommand.Path
$repositoryDirectory = Resolve-Path (Join-Path $scriptDirectory '..\..')
$output = [System.IO.Path]::GetFullPath($OutputDirectory)
$temporaryBundleRoot = $null

if ([string]::IsNullOrWhiteSpace($BundleDirectory)) {
	if ([string]::IsNullOrWhiteSpace($Archive)) {
		$Archive = Get-ChildItem -LiteralPath $output -Filter 'e9s-gui-*-windows-amd64.zip' |
			Sort-Object LastWriteTime -Descending |
			Select-Object -First 1 -ExpandProperty FullName
	}
	if ([string]::IsNullOrWhiteSpace($Archive) -or -not (Test-Path -LiteralPath $Archive -PathType Leaf)) {
		throw 'A portable Windows bundle directory or archive is required'
	}
	$temporaryBundleRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("e9s-installer-" + [guid]::NewGuid().ToString('N'))
	New-Item -ItemType Directory -Force -Path $temporaryBundleRoot | Out-Null
	Expand-Archive -LiteralPath $Archive -DestinationPath $temporaryBundleRoot
	$BundleDirectory = Get-ChildItem -LiteralPath $temporaryBundleRoot -Directory |
		Select-Object -First 1 -ExpandProperty FullName
}

try {
    $bundle = [System.IO.Path]::GetFullPath($BundleDirectory)
    $executable = Join-Path $bundle 'e9s-gui.exe'
    if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
        throw "Portable application is missing: $executable"
    }

    $icon = Join-Path $repositoryDirectory 'assets\icons\io.github.dostrow.e9s.ico'
    if (-not (Test-Path -LiteralPath $icon -PathType Leaf)) {
        throw "Windows icon is missing: $icon"
    }

    if ([string]::IsNullOrWhiteSpace($ISCC)) {
        $command = Get-Command 'ISCC.exe' -ErrorAction SilentlyContinue
        if ($null -ne $command) {
            $ISCC = $command.Source
        } else {
            $candidates = @(
                (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'),
                (Join-Path $env:ProgramFiles 'Inno Setup 6\ISCC.exe'),
                (Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 7\ISCC.exe'),
                (Join-Path $env:ProgramFiles 'Inno Setup 7\ISCC.exe')
            )
            $ISCC = $candidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
        }
    }
    if ([string]::IsNullOrWhiteSpace($ISCC) -or -not (Test-Path -LiteralPath $ISCC -PathType Leaf)) {
        throw 'Inno Setup compiler ISCC.exe was not found'
    }

    New-Item -ItemType Directory -Force -Path $output | Out-Null
    $artifactVersion = ($Version -replace '^v', '') -replace '[^0-9A-Za-z.+~-]', '-'
    $script = Join-Path $scriptDirectory 'installer.iss'
    $arguments = @(
        "--define=AppVersion=$Version",
        "--define=ArtifactVersion=$artifactVersion",
        "--define=BundleDirectory=$bundle",
        "--define=OutputDirectory=$output",
        "--define=IconFile=$icon",
        $script
    )

    & $ISCC $arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Inno Setup failed with exit code $LASTEXITCODE"
    }

    $installer = Join-Path $output "e9s-gui-$artifactVersion-windows-amd64-setup.exe"
    if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
        throw "Inno Setup did not create the expected installer: $installer"
    }
    Write-Host "Created $installer"
} finally {
    if ($null -ne $temporaryBundleRoot -and (Test-Path -LiteralPath $temporaryBundleRoot)) {
        Remove-Item -LiteralPath $temporaryBundleRoot -Recurse -Force
    }
}

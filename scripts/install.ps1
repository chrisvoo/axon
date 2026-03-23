# Install Axon to %LOCALAPPDATA%\axon\bin (user scope, no admin)
$ErrorActionPreference = "Stop"
$Repo = if ($env:AXON_REPO) { $env:AXON_REPO } else { "chrisvoo/axon" }
$Version = if ($env:AXON_VERSION) { $env:AXON_VERSION } else { "latest" }
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "arm64" }
$Url = "https://github.com/$Repo/releases/download/$Version/axon-windows-$Arch.zip"
$Dest = Join-Path $env:LOCALAPPDATA "axon\bin"
New-Item -ItemType Directory -Force -Path $Dest | Out-Null
$Zip = Join-Path $env:TEMP "axon-windows-$Arch.zip"
Invoke-WebRequest -Uri $Url -OutFile $Zip
Expand-Archive -Path $Zip -DestinationPath $Dest -Force
Remove-Item $Zip
Write-Host "Installed axon.exe to $Dest"
Write-Host "Add this directory to your user PATH if needed."

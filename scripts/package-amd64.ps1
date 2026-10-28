$ErrorActionPreference = "Stop"
$version = "1.8.78"
Write-Host "Syncing runtime assets..."
node scripts/sync-runtime-assets.js

Write-Host "Copying binary..."
Copy-Item "build/bin/TalentLens.exe" "build/bin/TalentLens-$version-amd64.exe" -Force

$repoRoot = (Resolve-Path -LiteralPath ".").Path
$binRoot = [System.IO.Path]::GetFullPath((Join-Path $repoRoot "build/bin"))
$stage = [System.IO.Path]::GetFullPath((Join-Path $binRoot "package-amd64-v$version"))
if (!$stage.StartsWith($binRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Packaging stage escapes build/bin"
}
if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Path $stage -Force | Out-Null
Copy-Item "build/bin/TalentLens.exe" (Join-Path $stage "TalentLens.exe") -Force
Copy-Item "build/bin/scripts" (Join-Path $stage "scripts") -Recurse -Force
Copy-Item "node_modules" (Join-Path $stage "node_modules") -Recurse -Force

if (!(Test-Path -LiteralPath (Join-Path $stage "scripts/51job_application_import.js"))) {
    throw "51job import script missing"
}
if (!(Test-Path -LiteralPath (Join-Path $stage "node_modules/puppeteer-core"))) {
    throw "puppeteer-core missing"
}

Write-Host "Compressing AMD64 archive..."
Remove-Item "build/bin/TalentLens-$version-windows-amd64.zip" -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath "build/bin/TalentLens-$version-windows-amd64.zip" -Force

Write-Host "AMD64 package created successfully"

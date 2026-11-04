$ErrorActionPreference = "Stop"
$version = "1.8.99"
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
Copy-Item "WINDOWS_PACKAGE_README.md" (Join-Path $stage "README.md") -Force
Copy-Item "LICENSE" (Join-Path $stage "LICENSE") -Force

# Bundle the official, pinned Windows CLI without any local OAuth credentials.
$tmeetVersion = "1.1.0"
$tmeetArchive = Join-Path $repoRoot "tencentcloud-tmeet-$tmeetVersion.tgz"
$tmeetBinary = Join-Path $repoRoot "bin/tmeet.exe"
$tmeetLicense = Join-Path $repoRoot "bin/TMEET-LICENSE.txt"
if (!(Test-Path -LiteralPath $tmeetBinary) -or !(Test-Path -LiteralPath $tmeetLicense)) {
    if (!(Test-Path -LiteralPath $tmeetArchive)) {
        npm pack "@tencentcloud/tmeet@$tmeetVersion" --ignore-scripts
        if ($LASTEXITCODE -ne 0) { throw "Could not download official Tencent Meeting CLI $tmeetVersion" }
    }
    $extractDir = Join-Path $binRoot "tmeet-extract-$tmeetVersion"
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
    tar -xzf $tmeetArchive -C $extractDir package/dist/tmeet-Windows-x86_64.exe package/LICENSE
    if ($LASTEXITCODE -ne 0) { throw "Could not extract official Tencent Meeting CLI" }
    New-Item -ItemType Directory -Path (Split-Path -Parent $tmeetBinary) -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $extractDir "package/dist/tmeet-Windows-x86_64.exe") -Destination $tmeetBinary -Force
    Copy-Item -LiteralPath (Join-Path $extractDir "package/LICENSE") -Destination $tmeetLicense -Force
}
New-Item -ItemType Directory -Path (Join-Path $stage "bin") -Force | Out-Null
Copy-Item -LiteralPath $tmeetBinary -Destination (Join-Path $stage "bin/tmeet.exe") -Force
Copy-Item -LiteralPath $tmeetLicense -Destination (Join-Path $stage "bin/TMEET-LICENSE.txt") -Force

if (!(Test-Path -LiteralPath (Join-Path $stage "scripts/51job_application_import.js"))) {
    throw "51job import script missing"
}
if (!(Test-Path -LiteralPath (Join-Path $stage "node_modules/puppeteer-core"))) {
    throw "puppeteer-core missing"
}
if (!(Test-Path -LiteralPath (Join-Path $stage "scripts/51job_conversation.js"))) {
    throw "51job conversation script missing"
}

Write-Host "Compressing AMD64 archive..."
Remove-Item "build/bin/TalentLens-$version-windows-amd64.zip" -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath "build/bin/TalentLens-$version-windows-amd64.zip" -Force

Write-Host "AMD64 package created successfully"

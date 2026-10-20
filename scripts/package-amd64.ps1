$version = "1.8.29"
Write-Host "Syncing runtime assets..."
node scripts/sync-runtime-assets.js

Write-Host "Copying binary..."
Copy-Item "build/bin/TalentLens.exe" "build/bin/TalentLens-$version-amd64.exe" -Force

$stage = "build/bin/package-amd64-v$version"
New-Item -ItemType Directory -Force "$stage" | Out-Null
Copy-Item "build/bin/TalentLens.exe" "$stage/TalentLens.exe" -Force
Copy-Item "build/bin/scripts" "$stage/scripts" -Recurse -Force
Copy-Item "node_modules" "$stage/node_modules" -Recurse -Force

if (!(Test-Path "$stage/scripts/51job_application_import.js")) {
    throw "51job import script missing"
}
if (!(Test-Path "$stage/node_modules/puppeteer-core")) {
    throw "puppeteer-core missing"
}

Write-Host "Compressing AMD64 archive..."
Remove-Item "build/bin/TalentLens-$version-windows-amd64.zip" -Force -ErrorAction SilentlyContinue
Compress-Archive -Path "$stage/*" -DestinationPath "build/bin/TalentLens-$version-windows-amd64.zip" -Force

Write-Host "AMD64 package created successfully"

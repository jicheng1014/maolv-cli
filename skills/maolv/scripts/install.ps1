# First installation only; an existing maolv is never replaced.
$ErrorActionPreference = 'Stop'
$existing = Get-Command maolv -CommandType Application -ErrorAction SilentlyContinue
if ($existing) { & $existing.Source version --json; exit $LASTEXITCODE }
$targetDir = Join-Path $env:LOCALAPPDATA 'Maolv\bin'
$target = Join-Path $targetDir 'maolv.exe'
if (Test-Path -LiteralPath $target) { & $target version --json; exit $LASTEXITCODE }
$version = if ($env:MAOLV_VERSION) { $env:MAOLV_VERSION } else { 'v0.1.0' }
if ($version -cnotmatch '\Av(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\z') { throw 'MAOLV_VERSION must be a release tag such as v0.1.0' }
$nativeArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($nativeArch) { 'AMD64' { 'amd64' }; 'ARM64' { 'arm64' }; default { throw 'Unsupported architecture' } }
$asset = "maolv_${version}_windows_${arch}.exe"
$base = "https://github.com/jicheng1014/maolv-cli/releases/download/$version"
$work = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString('N'))
$staged = $null
[void][IO.Directory]::CreateDirectory($work)
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
function Download([string]$Url, [string]$Path, [long]$Limit) {
    $deadline = [DateTime]::UtcNow.AddSeconds(180)
    for ($redirects = 0; $redirects -le 5; $redirects++) {
        $uri = [Uri]$Url
        if ($uri.Scheme -ne 'https') { throw 'Only HTTPS downloads and redirects are allowed' }
        $request = [Net.HttpWebRequest]::Create($uri)
        $request.AllowAutoRedirect = $false
        $request.Timeout = 15000
        $request.ReadWriteTimeout = 15000
        $response = $request.GetResponse()
        try {
            $status = [int]$response.StatusCode
            if ($status -in 301, 302, 303, 307, 308) {
                if (-not $response.Headers['Location']) { throw 'Redirect has no Location' }
                $Url = ([Uri]::new($uri, $response.Headers['Location'])).AbsoluteUri
                continue
            }
            if ($status -ne 200) { throw "Download HTTP $status" }
            if ($response.ContentLength -gt $Limit) { throw 'Download exceeds size limit' }
            $inputStream = $response.GetResponseStream()
            $outputStream = [IO.File]::Create($Path)
            try {
                $buffer = New-Object byte[] 65536
                [long]$total = 0
                while (($count = $inputStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
                    $total += $count
                    if ($total -gt $Limit) { throw 'Download exceeds size limit' }
                    if ([DateTime]::UtcNow -gt $deadline) { throw 'Download timed out' }
                    $outputStream.Write($buffer, 0, $count)
                }
            } finally { $outputStream.Dispose(); $inputStream.Dispose() }
            return
        } finally { $response.Dispose() }
    }
    throw 'Too many redirects'
}
try {
    $checksums = Join-Path $work 'checksums.txt'
    $binary = Join-Path $work 'maolv.exe'
    Download "$base/checksums.txt" $checksums 65536
    $matchesForAsset = @(Get-Content -LiteralPath $checksums | Where-Object { $_ -cmatch ('^[0-9a-f]{64}  ' + [Regex]::Escape($asset) + '$') })
    if ($matchesForAsset.Count -ne 1) { throw 'Missing or duplicate checksum' }
    $expected = $matchesForAsset[0].Substring(0, 64)
    Download "$base/$asset" $binary 67108864
    if ((Get-FileHash -LiteralPath $binary -Algorithm SHA256).Hash.ToLowerInvariant() -cne $expected) { throw 'Checksum mismatch; installation stopped' }
    $versionOutput = & $binary version --json
    if ($LASTEXITCODE -ne 0) { throw 'Downloaded binary did not run successfully' }
    if (($versionOutput | ConvertFrom-Json).data.version -cne $version) { throw 'Downloaded binary version does not match release tag' }
    [void][IO.Directory]::CreateDirectory($targetDir)
    $staged = Join-Path $targetDir ('.maolv-install.' + [Guid]::NewGuid().ToString('N'))
    [IO.File]::Copy($binary, $staged, $false)
    [IO.File]::Move($staged, $target)
    Write-Output $versionOutput
    Write-Output "Installed $version at $target"
    Write-Output 'For this terminal: $env:Path = "$env:LOCALAPPDATA\Maolv\bin;$env:Path"'
} finally {
    if ($staged -and (Test-Path -LiteralPath $staged)) { Remove-Item -LiteralPath $staged -Force }
    Remove-Item -LiteralPath $work -Recurse -Force
}

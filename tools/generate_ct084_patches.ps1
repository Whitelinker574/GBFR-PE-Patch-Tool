[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string] $InputCT,

    [Parameter(Mandatory = $true)]
    [string] $Output
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Remove-CEComments {
    param([Parameter(Mandatory = $true)][string] $Script)

    $builder = [System.Text.StringBuilder]::new($Script.Length)
    $state = 'normal'
    $blockDepth = 0
    for ($index = 0; $index -lt $Script.Length; $index++) {
        $character = $Script[$index]
        switch ($state) {
            'normal' {
                if ($character -eq '/' -and $index + 1 -lt $Script.Length -and $Script[$index + 1] -eq '/') {
                    [void] $builder.Append(' ')
                    [void] $builder.Append(' ')
                    $index++
                    $state = 'line-comment'
                }
                elseif ($character -eq '{') {
                    [void] $builder.Append(' ')
                    $blockDepth = 1
                    $state = 'block-comment'
                }
                else {
                    [void] $builder.Append($character)
                }
            }
            'line-comment' {
                if ($character -eq "`r" -or $character -eq "`n") {
                    [void] $builder.Append($character)
                    $state = 'normal'
                }
                else {
                    [void] $builder.Append(' ')
                }
            }
            'block-comment' {
                if ($character -eq "`r" -or $character -eq "`n") {
                    [void] $builder.Append($character)
                }
                elseif ($character -eq '{') {
                    [void] $builder.Append(' ')
                    $blockDepth++
                }
                elseif ($character -eq '}') {
                    [void] $builder.Append(' ')
                    $blockDepth--
                    if ($blockDepth -eq 0) {
                        $state = 'normal'
                    }
                }
                else {
                    [void] $builder.Append(' ')
                }
            }
        }
    }
    return $builder.ToString()
}

function ConvertTo-CleanDescription {
    param([AllowEmptyString()][string] $Value)

    $clean = $Value.Trim()
    if ($clean.Length -ge 2 -and $clean[0] -eq '"' -and $clean[$clean.Length - 1] -eq '"') {
        $clean = $clean.Substring(1, $clean.Length - 2).Trim()
    }
    return $clean
}

function ConvertTo-GroupName {
    param([AllowEmptyString()][string] $Value)

    $clean = ConvertTo-CleanDescription $Value
    if ($clean -match '^\[\s*(.*?)\s*\]$') {
        return $Matches[1].Trim()
    }
    return $clean
}

function ConvertFrom-AOBPattern {
    param([Parameter(Mandatory = $true)][string] $Pattern)

    $compact = ($Pattern -replace '\s+', '').ToUpperInvariant()
    if ($compact.Length -eq 0 -or ($compact.Length % 2) -ne 0 -or $compact -notmatch '^[0-9A-FX?]+$') {
        throw 'Invalid AOB pattern in candidate entry.'
    }

    [System.Collections.Generic.List[byte]] $values = @()
    [System.Collections.Generic.List[byte]] $masks = @()
    [System.Collections.Generic.List[string]] $display = @()
    for ($index = 0; $index -lt $compact.Length; $index += 2) {
        $pair = $compact.Substring($index, 2)
        $value = 0
        $mask = 0
        for ($nibbleIndex = 0; $nibbleIndex -lt 2; $nibbleIndex++) {
            $nibble = $pair[$nibbleIndex]
            $shift = if ($nibbleIndex -eq 0) { 4 } else { 0 }
            if ($nibble -ne 'X' -and $nibble -ne '?') {
                $value = $value -bor ([Convert]::ToInt32([string] $nibble, 16) -shl $shift)
                $mask = $mask -bor (0x0F -shl $shift)
            }
        }
        $values.Add([byte] $value)
        $masks.Add([byte] $mask)
        $display.Add($pair.Replace('X', '?'))
    }

    return [pscustomobject]@{
        AOB    = [string]::Join(' ', $display)
        Values = [byte[]] $values.ToArray()
        Masks  = [byte[]] $masks.ToArray()
    }
}

function ConvertFrom-CEInteger {
    param([Parameter(Mandatory = $true)][string] $Value)

    $clean = $Value.Trim()
    if ($clean.StartsWith('#')) {
        return [Convert]::ToInt32($clean.Substring(1), 10)
    }
    if ($clean.StartsWith('0x', [StringComparison]::OrdinalIgnoreCase)) {
        $clean = $clean.Substring(2)
    }
    return [Convert]::ToInt32($clean, 16)
}

function ConvertFrom-DirectByteInstruction {
    param([Parameter(Mandatory = $true)][string] $Line)

    $code = ($Line -split '//', 2)[0].Trim()
    if ($code -match '^(?i:db)\s+(.+?)\s*$') {
        $tokens = $Matches[1] -split '[\s,]+' | Where-Object { $_ -ne '' }
        [System.Collections.Generic.List[byte]] $bytes = @()
        foreach ($token in $tokens) {
            if ($token -notmatch '^[0-9A-Fa-f]{2}$') {
                return $null
            }
            $bytes.Add([Convert]::ToByte($token, 16))
        }
        return [byte[]] $bytes.ToArray()
    }
    if ($code -match '^(?i:nop)(?:\s+([#]?[0-9A-Fa-f]+))?\s*$') {
        $count = if ($Matches[1]) { ConvertFrom-CEInteger $Matches[1] } else { 1 }
        if ($count -le 0) {
            return $null
        }
        return [byte[]] (, [byte] 0x90 * $count)
    }
    return $null
}

function Get-DirectPatchBlocks {
    param(
        [Parameter(Mandatory = $true)][string] $Section,
        [Parameter(Mandatory = $true)][hashtable] $AOBBySymbol
    )

    $lines = $Section -split "`r?`n"
    [System.Collections.Generic.List[object]] $blocks = @()
    for ($lineIndex = 0; $lineIndex -lt $lines.Count; $lineIndex++) {
        $line = ($lines[$lineIndex] -split '//', 2)[0]
        if ($line -notmatch '^\s*([A-Za-z_][A-Za-z0-9_]*)(?:\+([0-9A-Fa-f]+))?\s*:\s*$') {
            continue
        }

        $symbol = $Matches[1]
        if (-not $AOBBySymbol.ContainsKey($symbol)) {
            continue
        }
        $offset = if ($Matches[2]) { ConvertFrom-CEInteger $Matches[2] } else { 0 }
        [System.Collections.Generic.List[byte]] $bytes = @()
        $scanIndex = $lineIndex + 1
        while ($scanIndex -lt $lines.Count) {
            $nextCode = ($lines[$scanIndex] -split '//', 2)[0].Trim()
            if ($nextCode -eq '') {
                $scanIndex++
                continue
            }
            $directBytes = ConvertFrom-DirectByteInstruction $nextCode
            if ($null -eq $directBytes) {
                break
            }
            foreach ($value in $directBytes) {
                $bytes.Add($value)
            }
            $scanIndex++
        }
        if ($bytes.Count -gt 0) {
            $blocks.Add([pscustomobject]@{
                Symbol = $symbol
                Offset = $offset
                Bytes  = [byte[]] $bytes.ToArray()
            })
        }
    }
    return [object[]] $blocks.ToArray()
}

function Get-AncestorNames {
    param([Parameter(Mandatory = $true)][System.Xml.XmlNode] $Entry)

    [System.Collections.Generic.List[string]] $names = @()
    $node = $Entry.ParentNode
    while ($null -ne $node) {
        if ($node.Name -eq 'CheatEntry') {
            $description = $node.SelectSingleNode('Description')
            if ($null -ne $description) {
                $names.Add((ConvertTo-GroupName $description.InnerText))
            }
        }
        $node = $node.ParentNode
    }
    $names.Reverse()
    if ($names.Count -gt 0 -and $names[0] -like 'NidasBot*') {
        $names.RemoveAt(0)
    }
    return [string[]] $names.ToArray()
}

try {
    $settings = [System.Xml.XmlReaderSettings]::new()
    $settings.DtdProcessing = [System.Xml.DtdProcessing]::Prohibit
    $settings.XmlResolver = $null
    $reader = [System.Xml.XmlReader]::Create((Resolve-Path -LiteralPath $InputCT).Path, $settings)
    try {
        $document = [System.Xml.XmlDocument]::new()
        $document.XmlResolver = $null
        $document.Load($reader)
    }
    finally {
        $reader.Dispose()
    }
}
catch {
    throw 'Failed to load CT XML safely.'
}

$knownUnsafeCTIDs = [System.Collections.Generic.HashSet[int]]::new()
foreach ($excludedCTID in @(
        31935, # CT warns that disabling Eugen's instant Detonator can crash.
        33086  # CT marks infinite repeat quest as experimental and potentially buggy.
    )) {
    [void] $knownUnsafeCTIDs.Add($excludedCTID)
}

$unsafeOrUnverifiedCTIDs = [System.Collections.Generic.HashSet[int]]::new()
foreach ($excludedCTID in @(
        31066, # Game 2.0.2 has two matches for NBGFR019B; the intended site is unproven.
        31960  # Game 2.0.2 has three matches for NBGFR040; the intended site is unproven.
    )) {
    [void] $unsafeOrUnverifiedCTIDs.Add($excludedCTID)
}

$alreadyImplementedCTIDs = [System.Collections.Generic.HashSet[int]]::new()
foreach ($excludedCTID in @(
        31060, # Infinite Link Time already has an independently owned implementation.
        31456  # Terminus weapon drop already has a safer independently owned implementation.
    )) {
    [void] $alreadyImplementedCTIDs.Add($excludedCTID)
}

$exclusionEvidence = @{
    31935 = "disabling Eugen's instant Detonator can crash"
    33086 = 'infinite repeat quest is experimental and potentially buggy'
    31066 = 'NBGFR019B has two matches in the locked game 2.0.2 EXE'
    31960 = 'NBGFR040 has three matches in the locked game 2.0.2 EXE'
    31060 = 'Infinite Link Time has an independently owned implementation'
    31456 = 'Terminus weapon drop has a safer independently owned implementation'
}

[System.Collections.Generic.List[object]] $features = @()
foreach ($entry in $document.SelectNodes('//CheatEntry[AssemblerScript]')) {
    $idNode = $entry.SelectSingleNode('ID')
    $nameNode = $entry.SelectSingleNode('Description')
    $scriptNode = $entry.SelectSingleNode('AssemblerScript')
    if ($null -eq $idNode -or $null -eq $nameNode -or $null -eq $scriptNode) {
        continue
    }

    $ctID = 0
    if (-not [int]::TryParse($idNode.InnerText.Trim(), [ref] $ctID)) {
        continue
    }
    $exclusionClass = ''
    if ($knownUnsafeCTIDs.Contains($ctID)) {
        $exclusionClass = 'known unsafe'
    }
    elseif ($unsafeOrUnverifiedCTIDs.Contains($ctID)) {
        $exclusionClass = 'unsafe or unverified'
    }
    elseif ($alreadyImplementedCTIDs.Contains($ctID)) {
        $exclusionClass = 'already implemented'
    }
    if (-not [string]::IsNullOrEmpty($exclusionClass)) {
        $evidence = [string] $exclusionEvidence[$ctID]
        if ([string]::IsNullOrWhiteSpace($evidence)) {
            throw "Excluded CT $ctID has no audit evidence."
        }
        Write-Verbose "Excluded CT $ctID [$exclusionClass]: $evidence"
        continue
    }
    $rawScript = $scriptNode.InnerText
    if ($rawScript -match '(?i)\{\s*\$lua\b|\[\s*lua\s*\]') {
        continue
    }
    $script = Remove-CEComments $rawScript
    if ($script -match '(?i)\bnewmem\b|\balloc\s*\(\s*mem|\blua\b|\bNBLib\b|\bcreateForm\b|\bshowMessage\b|\bdecodeFunction\b') {
        continue
    }

    $sectionMatch = [regex]::Match($script, '(?is)\[\s*ENABLE\s*\](.*?)\[\s*DISABLE\s*\](.*)$')
    if (-not $sectionMatch.Success) {
        continue
    }
    $enableSection = $sectionMatch.Groups[1].Value
    $disableSection = $sectionMatch.Groups[2].Value

    $aobBySymbol = @{}
    foreach ($aobMatch in [regex]::Matches($enableSection, '(?im)^\s*aobscanmodule\s*\(\s*([^,\s]+)\s*,\s*([^,]+?)\s*,\s*([^)]+?)\s*\)')) {
        $symbol = $aobMatch.Groups[1].Value.Trim()
        $aobBySymbol[$symbol] = [pscustomobject]@{
            Module  = $aobMatch.Groups[2].Value.Trim()
            Pattern = ConvertFrom-AOBPattern $aobMatch.Groups[3].Value
        }
    }
    if ($aobBySymbol.Count -eq 0) {
        continue
    }

    $enableBlocks = @(Get-DirectPatchBlocks -Section $enableSection -AOBBySymbol $aobBySymbol)
    if ($enableBlocks.Count -eq 0) {
        continue
    }
    $disableBlocks = @(Get-DirectPatchBlocks -Section $disableSection -AOBBySymbol $aobBySymbol)

    [System.Collections.Generic.List[object]] $sites = @()
    foreach ($block in $enableBlocks) {
        $disableBytes = $null
        foreach ($disableBlock in $disableBlocks) {
            if ($disableBlock.Symbol -eq $block.Symbol -and $disableBlock.Offset -eq $block.Offset) {
                $disableBytes = $disableBlock.Bytes
                break
            }
        }
        $disableOutput = @()
        if ($null -ne $disableBytes) {
            $disableOutput = [byte[]] $disableBytes
        }
        $aobDefinition = $aobBySymbol[$block.Symbol]
        $sites.Add([ordered]@{
            symbol                 = $block.Symbol
            module                 = $aobDefinition.Module
            aob                    = $aobDefinition.Pattern.AOB
            offset                 = $block.Offset
            patternValues          = [byte[]] $aobDefinition.Pattern.Values
            patternMasks           = [byte[]] $aobDefinition.Pattern.Masks
            enableBytes            = [byte[]] $block.Bytes
            disableBytes           = $disableOutput
            requiresRuntimeCapture = ($null -eq $disableBytes)
        })
    }

    $ancestors = @(Get-AncestorNames $entry)
    $characterRootIndex = -1
    for ($ancestorIndex = 0; $ancestorIndex -lt $ancestors.Count; $ancestorIndex++) {
        if ($ancestors[$ancestorIndex] -match '^\u89D2\u8272\u4FEE\u6539$') {
            $characterRootIndex = $ancestorIndex
            break
        }
    }
    $mode = if ($characterRootIndex -ge 0) {
        'characters'
    }
    elseif (@($ancestors | Where-Object { $_ -match '^\u6218\u6597\u529F\u80FD$' }).Count -gt 0) {
        'combat'
    }
    else {
        'quest'
    }
    $group = if ($ancestors.Count -gt 0) { $ancestors[$ancestors.Count - 1] } else { $mode }
    $character = ''
    if ($mode -eq 'characters') {
        $characterIndex = $characterRootIndex + 1
        if ($characterIndex -gt 0 -and $characterIndex -lt $ancestors.Count) {
            $character = $ancestors[$characterIndex]
        }
    }
    $name = ConvertTo-CleanDescription $nameNode.InnerText

    $features.Add([ordered]@{
        id            = "ct084-$ctID"
        ctId          = $ctID
        name          = $name
        displayName   = $name
        mode          = $mode
        category      = $mode
        group         = $group
        groupPath     = [string[]] $ancestors
        character     = $character
        conflicts     = [string[]] @()
        conflictGroup = ''
        sites         = [object[]] $sites.ToArray()
    })
}

$features = [object[]] ($features | Sort-Object @{ Expression = { $_.mode } }, @{ Expression = { $_.ctId } })
$featureByID = @{}
foreach ($feature in $features) {
    $featureByID[$feature.id] = $feature
}
$conflictIDs = [string[]] @('ct084-31967', 'ct084-31979', 'ct084-31995')
foreach ($conflictID in $conflictIDs) {
    if (-not $featureByID.ContainsKey($conflictID)) {
        continue
    }
    $feature = $featureByID[$conflictID]
    $feature.conflictGroup = 'damage-cap-display'
    $feature.conflicts = [string[]] ($conflictIDs | Where-Object { $_ -ne $conflictID -and $featureByID.ContainsKey($_) })
}

$catalog = [ordered]@{
    schemaVersion = 1
    sourceVersion = '0.8.4'
    sourceSha256  = (Get-FileHash -LiteralPath $InputCT -Algorithm SHA256).Hash.ToUpperInvariant()
    features      = $features
}

$outputPath = [System.IO.Path]::GetFullPath($Output)
$outputDirectory = [System.IO.Path]::GetDirectoryName($outputPath)
if (-not [string]::IsNullOrEmpty($outputDirectory)) {
    [System.IO.Directory]::CreateDirectory($outputDirectory) | Out-Null
}
$json = ($catalog | ConvertTo-Json -Depth 20) -replace "`r`n?", "`n"
[System.IO.File]::WriteAllText($outputPath, $json + "`n", [System.Text.UTF8Encoding]::new($false))

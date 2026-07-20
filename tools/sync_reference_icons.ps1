param(
    [string]$ReferenceZip = 'D:\gbf\GBFR-UI-Reference-Library-2.0.2.zip',
    [string]$GameTableZip = 'D:\gbf\GBFR-DLC-shuju-20260716-184413.zip',
    [switch]$SkillsOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$publicRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'frontend\public\loadout-icons'))
$jsonTarget = [IO.Path]::GetFullPath((Join-Path $repoRoot 'frontend\src\gameAssetIcons.json'))
$legacyTraitTarget = [IO.Path]::GetFullPath((Join-Path $repoRoot 'frontend\src\loadoutTraitIcons.json'))
$skillIconTarget = [IO.Path]::GetFullPath((Join-Path $repoRoot 'frontend\src\loadoutSkillIcons.json'))
if (-not $publicRoot.StartsWith($repoRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe asset target: $publicRoot"
}
if (-not (Test-Path -LiteralPath $ReferenceZip -PathType Leaf)) {
    throw "Reference archive not found: $ReferenceZip"
}
if (-not (Test-Path -LiteralPath $GameTableZip -PathType Leaf)) {
    throw "Unpacked 2.0.2 game table archive not found: $GameTableZip"
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($ReferenceZip)
try {
    $catalogEntry = $zip.GetEntry('GBFR-UI-Reference-Library-2.0.2/index-release-v2/semantic-catalog.json')
    if ($null -eq $catalogEntry) { throw 'index-release-v2 semantic catalog is missing' }
    $reader = [IO.StreamReader]::new($catalogEntry.Open(), [Text.Encoding]::UTF8)
    try { $records = ($reader.ReadToEnd() | ConvertFrom-Json).records }
    finally { $reader.Dispose() }

    # Keep the script Windows PowerShell 5.1 compatible: that host treats a
    # UTF-8 file without BOM as the active ANSI code page. Decode the four
    # catalog category names at runtime so the source itself remains ASCII.
    $utf8 = [Text.Encoding]::UTF8
    $categoryTraits = $utf8.GetString([Convert]::FromBase64String('5Zug5a2Q5LiO5oqA6IO95Zu+5qCH'))
    $categoryWeapons = $utf8.GetString([Convert]::FromBase64String('5q2m5Zmo'))
    $categorySummons = $utf8.GetString([Convert]::FromBase64String('5Y+s5ZSk55+z'))
    $categoryItems = $utf8.GetString([Convert]::FromBase64String('54mp5ZOB5Zu+5qCH'))

    # Every alias ends in __<internal game asset>.png.  Index the release-v2
    # entries once so extraction never depends on translated filenames.
    $assetEntries = @{}
    foreach ($entry in $zip.Entries) {
        if ($entry.FullName -notlike '*/index-release-v2/*' -or $entry.Name -notmatch '(?i)\.png$') { continue }
        $marker = $entry.Name.LastIndexOf('__', [StringComparison]::Ordinal)
        $internalFile = if ($marker -ge 0) { $entry.Name.Substring($marker + 2) } else { $entry.Name }
        if (-not $assetEntries.ContainsKey($internalFile)) { $assetEntries[$internalFile] = $entry }
    }

    function Normalize-Hex([object]$value) {
        return ([string]$value).Trim().Replace('0x', '').Replace('0X', '').ToUpperInvariant()
    }

    function Prefer-Record([object[]]$candidates) {
        return $candidates |
            Sort-Object @{ Expression = { if ($_.confidence -eq 'verified-join') { 0 } else { 1 } } }, internal_name |
            Select-Object -First 1
    }

    $recordByKey = @{}
    foreach ($group in ($records | Group-Object { "$($_.category)|$($_.entity_id.ToUpperInvariant())" })) {
        $recordByKey[$group.Name] = Prefer-Record @($group.Group | Where-Object { $_.internal_name -notmatch '_glow$' })
    }

    function Record-For([string]$category, [string]$entityID) {
        $key = "$category|$($entityID.ToUpperInvariant())"
        if ($recordByKey.ContainsKey($key)) { return $recordByKey[$key] }
        return $null
    }

    function Copy-InternalAsset([string]$folder, [string]$internalName) {
        if ([string]::IsNullOrWhiteSpace($internalName)) { return '' }
        # embed.FS is case-sensitive even on Windows.  Keep the generated URL,
        # the on-disk filename and the future Wails embedded path identical.
        $fileName = if ($internalName.EndsWith('.png', [StringComparison]::OrdinalIgnoreCase)) { $internalName } else { "$internalName.png" }
        $fileName = $fileName.ToLowerInvariant()
        if (-not $assetEntries.ContainsKey($fileName)) { return '' }
        $targetDir = [IO.Path]::GetFullPath((Join-Path $publicRoot $folder))
        if (-not $targetDir.StartsWith($publicRoot, [StringComparison]::OrdinalIgnoreCase)) { throw "Unsafe icon folder: $folder" }
        [IO.Directory]::CreateDirectory($targetDir) | Out-Null
        $target = [IO.Path]::GetFullPath((Join-Path $targetDir $fileName))
        $caseVariant = Get-ChildItem -LiteralPath $targetDir -File | Where-Object {
            [string]::Equals($_.Name, $fileName, [StringComparison]::OrdinalIgnoreCase) -and
            -not [string]::Equals($_.Name, $fileName, [StringComparison]::Ordinal)
        } | Select-Object -First 1
        if ($null -ne $caseVariant) {
            $caseTemp = Join-Path $targetDir (".case-normalize-" + [guid]::NewGuid().ToString('N'))
            Move-Item -LiteralPath $caseVariant.FullName -Destination $caseTemp
            Move-Item -LiteralPath $caseTemp -Destination $target
        }
        [IO.Compression.ZipFileExtensions]::ExtractToFile($assetEntries[$fileName], $target, $true)
        return $fileName
    }

    function Read-GameTableBytes([string]$entryName) {
        $tableArchive = [IO.Compression.ZipFile]::OpenRead($GameTableZip)
        try {
            $tableEntry = $tableArchive.Entries | Where-Object { $_.Name -eq $entryName } | Select-Object -First 1
            if ($null -eq $tableEntry) { throw "Unpacked 2.0.2 archive does not contain $entryName" }
            $memory = [IO.MemoryStream]::new()
            try {
                $entryStream = $tableEntry.Open()
                try { $entryStream.CopyTo($memory) }
                finally { $entryStream.Dispose() }
                return ,$memory.ToArray()
            }
            finally { $memory.Dispose() }
        }
        finally { $tableArchive.Dispose() }
    }

    function Read-FixedASCII([byte[]]$bytes, [int]$offset, [int]$length) {
        return [Text.Encoding]::ASCII.GetString($bytes, $offset, $length).TrimEnd([char]0)
    }

    function Add-RecordIcon([hashtable]$map, [string]$key, [object]$record, [string]$folder) {
        if ($null -eq $record -or [string]::IsNullOrWhiteSpace($key)) { return '' }
        $file = Copy-InternalAsset $folder ([string]$record.internal_name)
        if ($file) { $map[$key] = $file }
        return $file
    }

    function Ordered-Map([hashtable]$map) {
        $ordered = [ordered]@{}
        foreach ($key in ($map.Keys | Sort-Object)) { $ordered[$key] = $map[$key] }
        return $ordered
    }

    function Build-ExactPlayableSkillMap {
        # Build active-skill mappings only from the exact 2.0.2 ability row.
        # Translated/community filenames are intentionally not used as a
        # fallback because similarly named and alternate-form skills can point
        # at different game assets.
        $skillCatalog = (Get-Content -LiteralPath (Join-Path $repoRoot 'data\skill_names.json') -Raw -Encoding UTF8 | ConvertFrom-Json).skills
        $abilityTableBytes = [byte[]](Read-GameTableBytes 'ability.tbl')
        $abilityTableRowCount = [BitConverter]::ToInt64($abilityTableBytes, 0)
        $abilityTableRowSize = 96
        if (8 + ($abilityTableRowCount * $abilityTableRowSize) -ne $abilityTableBytes.Length) {
            throw "Unexpected 2.0.2 ability.tbl layout: rows=$abilityTableRowCount bytes=$($abilityTableBytes.Length)"
        }
        $abilityIconByHash = @{}
        for ($row = 0; $row -lt $abilityTableRowCount; $row++) {
            $offset = 8 + ($row * $abilityTableRowSize)
            $keyHash = '{0:X8}' -f [BitConverter]::ToUInt32($abilityTableBytes, $offset + 48)
            $abilityIconByHash[$keyHash] = Read-FixedASCII $abilityTableBytes $offset 24
        }

        $skillMap = @{}
        $missingSkillKeys = [Collections.Generic.List[string]]::new()
        $playableCount = 0
        foreach ($property in $skillCatalog.PSObject.Properties) {
            $skill = $property.Value
            if ([string]$skill.char -notmatch '^PL\d{4}$') { continue }
            $playableCount++
            $key = [string]$skill.key
            $hash = Normalize-Hex $property.Name
            if (-not $abilityIconByHash.ContainsKey($hash)) {
                throw "ability.tbl has no exact row for $key / $hash"
            }
            $iconID = [string]$abilityIconByHash[$hash]
            $file = Copy-InternalAsset 'skills' "cmn_icablt_pl$iconID"
            if (-not $file) {
                $missingSkillKeys.Add($key)
                continue
            }
            $skillMap[$key] = $file
        }

        if ($playableCount -ne 262 -or $skillMap.Count -ne 261) {
            throw "Unexpected playable ability coverage: mapped=$($skillMap.Count) playable=$playableCount missing=$($missingSkillKeys -join ',')"
        }
        if ($missingSkillKeys.Count -ne 1 -or $missingSkillKeys[0] -ne 'AB_PL2400_09') {
            throw "Unexpected missing 2.0.2 ability sprites: $($missingSkillKeys -join ',')"
        }

        return [pscustomobject]@{
            map = $skillMap
            playable = $playableCount
            missing = @($missingSkillKeys)
        }
    }

    if ($SkillsOnly) {
        $skillBuild = Build-ExactPlayableSkillMap
        $skillMap = $skillBuild.map

        $skillJSON = (Ordered-Map $skillMap) | ConvertTo-Json -Depth 2
        $skillJSON = $skillJSON -replace '(?m)^    "', '  "' -replace '":  "', '": "'
        [IO.File]::WriteAllText($skillIconTarget, $skillJSON + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
        Write-Output ([ordered]@{
            playable = $skillBuild.playable
            mapped = $skillMap.Count
            missing = @($skillBuild.missing)
        } | ConvertTo-Json -Compress)
        return
    }

    $traitsByID = @{}
    $traitsByHash = @{}
    $traitsByName = @{}
    $weaponsByID = @{}
    $weaponsByHash = @{}
    $summonsByHash = @{}
    $itemsByHash = @{}
    $charactersByHash = @{}

    # Keep semantic records for the legacy name-only compatibility map. The
    # application ID/hash maps below are rebuilt from skill.tbl instead: a
    # translated name is not an identity, and duplicate display names can use
    # different sprites.
    $traitRecords = @($records | Where-Object { $_.category -eq $categoryTraits -and $_.internal_name -notmatch '_glow$' })
    foreach ($record in $traitRecords) {
        [void](Add-RecordIcon $traitsByID ([string]$record.entity_id) $record 'traits')
    }
    # A few Gran/Djeeta traits share the same translated name but have two
    # official emblems. Resolve name-only compatibility deterministically to
    # the ordinal-first internal asset (IconId1 / Gran), rather than whichever
    # record happened to occur last in the catalog.
    foreach ($nameProperty in @('name_cn', 'name_en')) {
        foreach ($group in @($traitRecords | Where-Object { $_.$nameProperty } | Group-Object -Property $nameProperty)) {
            $record = Prefer-Record @($group.Group)
            $file = Copy-InternalAsset 'traits' ([string]$record.internal_name)
            if ($file) {
                $traitsByName[[string]$group.Name] = $file
            }
        }
    }

    $traitRows = (Get-Content -LiteralPath (Join-Path $repoRoot 'data\traits.json') -Raw -Encoding UTF8 | ConvertFrom-Json).traits
    $skillTableBytes = [byte[]](Read-GameTableBytes 'skill.tbl')
    $skillTableRowCount = [BitConverter]::ToInt64($skillTableBytes, 0)
    $skillTableRowSize = 112
    if (8 + ($skillTableRowCount * $skillTableRowSize) -ne $skillTableBytes.Length) {
        throw "Unexpected 2.0.2 skill.tbl layout: rows=$skillTableRowCount bytes=$($skillTableBytes.Length)"
    }
    $traitIconByHash = @{}
    for ($row = 0; $row -lt $skillTableRowCount; $row++) {
        $offset = 8 + ($row * $skillTableRowSize)
        $keyHash = '{0:X8}' -f [BitConverter]::ToUInt32($skillTableBytes, $offset + 68)
        if ($traitIconByHash.ContainsKey($keyHash)) {
            throw "Duplicate skill.tbl hash: $keyHash"
        }
        # The first fixed 16-byte field is IconId1. A second field exists for
        # the Gran/Djeeta variant, but the application catalog uses IconId1 as
        # its canonical compact trait icon, matching the in-game default.
        $traitIconByHash[$keyHash] = Read-FixedASCII $skillTableBytes $offset 16
    }

    $missingTraitKeys = [Collections.Generic.List[string]]::new()
    foreach ($trait in $traitRows) {
        $id = [string]$trait.internalId
        $hash = Normalize-Hex $trait.hash
        if (-not $traitIconByHash.ContainsKey($hash)) {
            throw "skill.tbl has no exact row for application trait $id / $hash"
        }

        # Remove any semantic ID record before applying the authoritative table
        # join, so a missing/blank official IconId can never retain a guessed
        # alias from an earlier generator stage.
        [void]$traitsByID.Remove($id)
        [void]$traitsByHash.Remove($hash)
        $iconID = [string]$traitIconByHash[$hash]
        if ([string]::IsNullOrWhiteSpace($iconID)) {
            $missingTraitKeys.Add($id)
            continue
        }

        $file = Copy-InternalAsset 'traits' "cmn_icskill_$iconID"
        if (-not $file) {
            $missingTraitKeys.Add($id)
            continue
        }
        $traitsByID[$id] = $file
        $traitsByHash[$hash] = $file
    }
    if ($missingTraitKeys.Count -ne 1 -or $missingTraitKeys[0] -ne 'SKILL_112_00') {
        throw "Unexpected missing 2.0.2 trait sprites: $($missingTraitKeys -join ',')"
    }

    $weaponRecords = @($records | Where-Object { $_.category -eq $categoryWeapons -and $_.internal_name -notmatch '_glow$' })
    foreach ($record in $weaponRecords) {
        $entity = [string]$record.entity_id
        $target = if ($entity -match '^WEP_') { $weaponsByID } else { $weaponsByHash }
        [void](Add-RecordIcon $target $entity $record 'weapons')
    }
    $weaponRows = (Get-Content -LiteralPath (Join-Path $repoRoot 'data\weapons.json') -Raw -Encoding UTF8 | ConvertFrom-Json).weapons
    $weaponTableBytes = [byte[]](Read-GameTableBytes 'weapon.tbl')
    $weaponTableRowCount = [BitConverter]::ToInt64($weaponTableBytes, 0)
    $weaponTableRowSize = 292
    if (8 + ($weaponTableRowCount * $weaponTableRowSize) -ne $weaponTableBytes.Length) {
        throw "Unexpected 2.0.2 weapon.tbl layout: rows=$weaponTableRowCount bytes=$($weaponTableBytes.Length)"
    }
    $weaponIconByHash = @{}
    for ($row = 0; $row -lt $weaponTableRowCount; $row++) {
        $offset = 8 + ($row * $weaponTableRowSize)
        $keyHash = '{0:X8}' -f [BitConverter]::ToUInt32($weaponTableBytes, $offset + 168)
        $weaponIconByHash[$keyHash] = Read-FixedASCII $weaponTableBytes $offset 16
    }
    foreach ($weapon in $weaponRows) {
        $id = [string]$weapon.internalId
        $hash = Normalize-Hex $weapon.hash
        # Exact 2.0.2 table icon IDs take precedence over semantic aliases.
        # In particular, similarly named base/awakening weapons are distinct
        # resources (for example wp2102 and wp2106) and must never be joined by
        # an English-name prefix.
        if ($id) { [void]$weaponsByID.Remove($id) }
        if ($hash) { [void]$weaponsByHash.Remove($hash) }
        if (-not $hash -or -not $weaponIconByHash.ContainsKey($hash)) {
            throw "weapon.tbl has no exact row for application weapon $id / $hash"
        }
        $iconID = [string]$weaponIconByHash[$hash]
        if ($iconID) {
            $file = Copy-InternalAsset 'weapons' "cmn_imgequ_wp$iconID"
            if ($file) {
                if ($id) { $weaponsByID[$id] = $file }
                $weaponsByHash[$hash] = $file
            }
        }
    }

    # These late-game canonical hashes are present in weapon.tbl but are not
    # rows in the curated application weapon list. They are real values seen
    # in runtime/save weapon records, so keep their exact table-backed aliases
    # reproducible instead of relying on a hand-edited generated JSON file.
    $auditedRuntimeWeapons = [ordered]@{
        'WEP_PL2100_07' = [ordered]@{ hash = 'AD915067'; icon = '2106' }
        'WEP_PL2200_07' = [ordered]@{ hash = 'FA5F32D5'; icon = '2206' }
        'WEP_PL2300_07' = [ordered]@{ hash = '4CBA06D8'; icon = '2306' }
    }
    foreach ($pair in $auditedRuntimeWeapons.GetEnumerator()) {
        $id = [string]$pair.Key
        $hash = [string]$pair.Value.hash
        $expectedIconID = [string]$pair.Value.icon
        if (-not $weaponIconByHash.ContainsKey($hash)) {
            throw "weapon.tbl has no exact row for audited runtime weapon $id / $hash"
        }
        $actualIconID = [string]$weaponIconByHash[$hash]
        if ($actualIconID -ne $expectedIconID) {
            throw "Unexpected weapon.tbl icon for $id / ${hash}: expected=$expectedIconID actual=$actualIconID"
        }
        $file = Copy-InternalAsset 'weapons' "cmn_imgequ_wp$actualIconID"
        if (-not $file) {
            throw "Reference archive is missing exact runtime weapon sprite cmn_imgequ_wp$actualIconID"
        }
        $weaponsByID[$id] = $file
        $weaponsByHash[$hash] = $file
    }

    foreach ($record in @($records | Where-Object { $_.category -eq $categorySummons -and $_.internal_name -notmatch '_glow$' })) {
        [void](Add-RecordIcon $summonsByHash (Normalize-Hex $record.entity_id) $record 'summons')
    }
    foreach ($record in @($records | Where-Object { $_.category -eq $categoryItems -and $_.internal_name -notmatch '_glow$' })) {
        [void](Add-RecordIcon $itemsByHash (Normalize-Hex $record.entity_id) $record 'items')
    }

    # Rebuild every ordinary application item through item.tbl. The save item
    # hash is the uint32 at +32 and the official compact sprite ID is the fixed
    # ASCII field at +16. Semantic aliases are useful as an index, but this
    # table join is the authoritative identity proof and prevents similarly
    # named materials from inheriting the wrong icon.
    $itemRows = (Get-Content -LiteralPath (Join-Path $repoRoot 'data\items.json') -Raw -Encoding UTF8 | ConvertFrom-Json).items
    $itemTableBytes = [byte[]](Read-GameTableBytes 'item.tbl')
    $itemRowCount = [BitConverter]::ToInt64($itemTableBytes, 0)
    $itemTableRowSize = 128
    if (8 + ($itemRowCount * $itemTableRowSize) -ne $itemTableBytes.Length) {
        throw "Unexpected 2.0.2 item.tbl layout: rows=$itemRowCount bytes=$($itemTableBytes.Length)"
    }
    $itemIconByHash = @{}
    for ($row = 0; $row -lt $itemRowCount; $row++) {
        $offset = 8 + ($row * $itemTableRowSize)
        $itemHash = '{0:X8}' -f [BitConverter]::ToUInt32($itemTableBytes, $offset + 32)
        $iconID = Read-FixedASCII $itemTableBytes ($offset + 16) 16
        if ($itemIconByHash.ContainsKey($itemHash) -and $itemIconByHash[$itemHash] -ne $iconID) {
            throw "Conflicting item.tbl icon rows for hash ${itemHash}: $($itemIconByHash[$itemHash]) / $iconID"
        }
        $itemIconByHash[$itemHash] = $iconID
    }
    foreach ($item in $itemRows) {
        $hash = Normalize-Hex $item.hash
        if (-not $itemIconByHash.ContainsKey($hash)) { continue }
        [void]$itemsByHash.Remove($hash)
        $iconID = [string]$itemIconByHash[$hash]
        $file = Copy-InternalAsset 'items' "cmn_icitm_$iconID"
        if ($file) { $itemsByHash[$hash] = $file }
    }

    # ITEM_70_0020..0027 use their normal identifier hashes in the save, but
    # the 2.0.2 item table stores eight opaque row keys. Join through the
    # table's exact TXT_ITEM_70_002x name hash instead. All eight rows point to
    # the same shipped cmn_icitm_50_0100 sprite. This is a table-backed alias,
    # not a name/appearance guess.
    $skyMemoryNameHashToItemHash = @{
        '9650A8EF'='6C375D99' # TXT_ITEM_70_0020 -> ITEM_70_0020
        '25574353'='2C0B5A82' # TXT_ITEM_70_0021 -> ITEM_70_0021
        'E1096B8D'='96A00C42' # TXT_ITEM_70_0022 -> ITEM_70_0022
        '5D3D0746'='E3ABC397' # TXT_ITEM_70_0023 -> ITEM_70_0023
        '5FAE3082'='0943913B' # TXT_ITEM_70_0024 -> ITEM_70_0024
        '9827C2DA'='C57A5FE9' # TXT_ITEM_70_0025 -> ITEM_70_0025
        '9D1FA527'='4249436E' # TXT_ITEM_70_0026 -> ITEM_70_0026
        '463611CA'='74201317' # TXT_ITEM_70_0027 -> ITEM_70_0027
    }
    $unresolvedSkyMemoryHashes = @{}
    foreach ($pair in $skyMemoryNameHashToItemHash.GetEnumerator()) {
        $unresolvedSkyMemoryHashes[$pair.Value] = $true
    }
    try {
        for ($row = 0; $row -lt $itemRowCount; $row++) {
            $offset = 8 + ($row * $itemTableRowSize)
            $nameHash = '{0:X8}' -f [BitConverter]::ToUInt32($itemTableBytes, $offset + 36)
            if (-not $skyMemoryNameHashToItemHash.ContainsKey($nameHash)) { continue }

            $iconID = Read-FixedASCII $itemTableBytes ($offset + 16) 16
            if ($iconID -ne '50_0100') {
                throw "Unexpected Sky Memory icon in item.tbl: nameHash=$nameHash icon=$iconID"
            }
            $file = Copy-InternalAsset 'items' "cmn_icitm_$iconID"
            if (-not $file) { throw "Reference archive is missing exact Sky Memory sprite cmn_icitm_$iconID" }
            $itemHash = $skyMemoryNameHashToItemHash[$nameHash]
            $itemsByHash[$itemHash] = $file
            [void]$unresolvedSkyMemoryHashes.Remove($itemHash)
        }
        if ($unresolvedSkyMemoryHashes.Count -ne 0) {
            throw "item.tbl did not prove every Sky Memory alias: $($unresolvedSkyMemoryHashes.Keys -join ', ')"
        }

        $missingItemHashes = @($itemRows |
            Where-Object { -not $itemsByHash.ContainsKey((Normalize-Hex $_.hash)) } |
            ForEach-Object { Normalize-Hex $_.hash } |
            Sort-Object)
        $expectedMissingItemHashes = @(
            '131A4636', '29BBA035', '7F695B76', '9FC6585E', 'CB39E0FC', 'CD6AF550',
            'CE0B379E', 'E600BE75', 'E8C461CA', 'EAC2D7AB', 'F384E322'
        )
        if (($missingItemHashes -join ',') -ne ($expectedMissingItemHashes -join ',')) {
            throw "Unexpected missing 2.0.2 item sprites: $($missingItemHashes -join ',')"
        }
    }
    finally {
        $itemTableBytes = $null
    }

    # Hash-to-player-code truth is shared with the save identity table and the
    # unpacked weapon/skill catalogs. These are the game's own compact roster
    # icons, not generated portraits or community substitutes.
    $characterCodes = [ordered]@{
        '2A26B1B2'='PL0000'; 'A4ACBA76'='PL0100'; '18E2F9F9'='PL0200'; '079DF0CC'='PL0300'
        '4D0A60C3'='PL0400'; 'DD7A151E'='PL0500'; 'C8616284'='PL0600'; '978E4B18'='PL1500'
        'C3FFD418'='PL0700'; '22E437E5'='PL0800'; '2EBE91D5'='PL0900'; 'BDEF7181'='PL1000'
        '627BCB0D'='PL1100'; 'FD3BE362'='PL1200'; 'BAD16E3B'='PL2300'; 'FC6CDF7B'='PL1300'
        'E7053919'='PL1400'; '1BB37EF0'='PL2400'; '0D21B430'='PL1600'; 'A3A3CB2F'='PL1900'
        'F0EB77EF'='PL1700'; 'AA66178A'='PL1800'; '718E1A14'='PL2100'; '296471BE'='PL2200'
        '74DD4C79'='PL2900'; '9A8AF295'='PL2600'; '25D46F4B'='PL2500'; '9B15CFB1'='PL2700'
        '646C3168'='PL2800'
    }
    foreach ($pair in $characterCodes.GetEnumerator()) {
        $file = "cmn_mini_s_$($pair.Value.ToLowerInvariant()).png"
        $copied = Copy-InternalAsset 'characters' $file
        if ($copied) { $charactersByHash[$pair.Key] = $copied }
    }

    # Full and skills-only generation deliberately share one table-exact path.
    # Generated JSON is output only, never an input, so stale or hand-edited
    # keys cannot survive a reproducible full sync.
    $skillBuild = Build-ExactPlayableSkillMap
    $skillMap = $skillBuild.map

    $summonRows = (Get-Content -LiteralPath (Join-Path $repoRoot 'data\summons.json') -Raw -Encoding UTF8 | ConvertFrom-Json).summons
    $coverage = [ordered]@{
        traits = "$(@($traitRows | Where-Object { $traitsByID.ContainsKey([string]$_.internalId) }).Count)/$($traitRows.Count)"
        weapons = "$(@($weaponRows | Where-Object { $weaponsByHash.ContainsKey((Normalize-Hex $_.hash)) }).Count)/$($weaponRows.Count)"
        summons = "$(@($summonRows | Where-Object { $summonsByHash.ContainsKey((Normalize-Hex $_.hash)) }).Count)/$($summonRows.Count)"
        items = "$(@($itemRows | Where-Object { $itemsByHash.ContainsKey((Normalize-Hex $_.hash)) }).Count)/$($itemRows.Count)"
        characters = "$($charactersByHash.Count)/$($characterCodes.Count)"
    }

    $output = [ordered]@{
        schemaVersion = 1
        source = 'GBFR UI Reference Library 2.0.2 / semantic catalog + unpacked 2.0.2 game tables'
        coverage = $coverage
        traits = [ordered]@{ byId = Ordered-Map $traitsByID; byHash = Ordered-Map $traitsByHash; byName = Ordered-Map $traitsByName }
        weapons = [ordered]@{ byId = Ordered-Map $weaponsByID; byHash = Ordered-Map $weaponsByHash }
        summons = [ordered]@{ byHash = Ordered-Map $summonsByHash }
        items = [ordered]@{ byHash = Ordered-Map $itemsByHash }
        characters = [ordered]@{ byHash = Ordered-Map $charactersByHash }
    }
    $json = $output | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText($jsonTarget, $json + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    $legacyTraitJSON = (Ordered-Map $traitsByName) | ConvertTo-Json -Depth 4
    [IO.File]::WriteAllText($legacyTraitTarget, $legacyTraitJSON + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    $skillJSON = (Ordered-Map $skillMap) | ConvertTo-Json -Depth 2
    $skillJSON = $skillJSON -replace '(?m)^    "', '  "' -replace '":  "', '": "'
    [IO.File]::WriteAllText($skillIconTarget, $skillJSON + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    Write-Output ($coverage | ConvertTo-Json -Compress)
}
finally {
    $zip.Dispose()
}

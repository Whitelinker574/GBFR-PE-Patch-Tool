// dllmain.cpp : 定义 DLL 应用程序的入口点。
#include "pch.h"
#include <Windows.h>
#include <cctype>
#include <cstdint>
#include <cstdlib>
#include <libmem/libmem.h>
#include <cstdio>
#include <cstring>
#include <TlHelp32.h>
#include <vector>
#include <array>
#include <algorithm>
#include <atomic>
#include <cmath>
#include <filesystem>
#include <fstream>
#include <mutex>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <utility>

static std::wstring RuntimeDirectory()
{
    wchar_t tempPath[MAX_PATH]{};
    if (!GetTempPathW(_countof(tempPath), tempPath)) return L"";
    std::wstring directory = std::wstring(tempPath) + L"gbfr-player-info-edit\\runtime";
    CreateDirectoryW((std::wstring(tempPath) + L"gbfr-player-info-edit").c_str(), nullptr);
    CreateDirectoryW(directory.c_str(), nullptr);
    return directory;
}

static std::wstring RuntimePath(const wchar_t* name)
{
    std::wstring directory = RuntimeDirectory();
    return directory.empty() ? L"" : directory + L"\\" + name;
}

static float ReadIniFloat(const wchar_t* path, const wchar_t* section, const wchar_t* key, float fallback)
{
    wchar_t value[64]{};
    wchar_t defaultValue[64]{};
    swprintf_s(defaultValue, L"%.9g", fallback);
    GetPrivateProfileStringW(section, key, defaultValue, value, _countof(value), path);
    wchar_t* end = nullptr;
    float parsed = wcstof(value, &end);
    return end != value && std::isfinite(parsed) ? parsed : fallback;
}

static uint64_t CurrentProcessCreationTime()
{
    FILETIME creation{}, exit{}, kernel{}, user{};
    if (!GetProcessTimes(GetCurrentProcess(), &creation, &exit, &kernel, &user)) return 0;
    ULARGE_INTEGER value{};
    value.LowPart = creation.dwLowDateTime;
    value.HighPart = creation.dwHighDateTime;
    return value.QuadPart;
}

static char g_runtimeGeneration[33]{};
static wchar_t g_runtimeGenerationWide[33]{};

static bool ReadCommandValue(const char* content, const char* key, char* value, size_t valueSize)
{
    if (!content || !key || !*key || !value || valueSize < 2) return false;
    const size_t keyLength = strlen(key);
    const char* line = content;
    while (line && *line)
    {
        const char* end = strpbrk(line, "\r\n");
        const size_t lineLength = end ? static_cast<size_t>(end - line) : strlen(line);
        if (lineLength > keyLength && strncmp(line, key, keyLength) == 0 && line[keyLength] == '=')
        {
            const char* source = line + keyLength + 1;
            const size_t length = lineLength - keyLength - 1;
            if (!length || length >= valueSize) return false;
            memcpy(value, source, length);
            value[length] = '\0';
            return true;
        }
        if (!end) break;
        line = end + 1;
        if (*end == '\r' && *line == '\n') ++line;
    }
    return false;
}

static bool IsValidRuntimeGeneration(const char* generation)
{
    if (!generation || strlen(generation) != 32) return false;
    for (const unsigned char* cursor = reinterpret_cast<const unsigned char*>(generation); *cursor; ++cursor)
    {
        if (!std::isxdigit(*cursor)) return false;
    }
    return true;
}

static bool SetRuntimeGeneration(const char* generation)
{
    if (!IsValidRuntimeGeneration(generation)) return false;
    strcpy_s(g_runtimeGeneration, generation);
    for (size_t index = 0; index <= 32; ++index)
        g_runtimeGenerationWide[index] = static_cast<unsigned char>(generation[index]);
    return true;
}

static HANDLE OpenRuntimeOwnerForGeneration(const wchar_t* feature)
{
    if (!feature || !*feature || !IsValidRuntimeGeneration(g_runtimeGeneration)) return INVALID_HANDLE_VALUE;
    std::wstring path = RuntimePath((std::wstring(feature) + L".owner").c_str());
    if (path.empty()) return INVALID_HANDLE_VALUE;
    // Deliberately do not share DELETE. Once this exact owner generation has
    // been validated, the desktop cannot unlink it and publish a successor
    // until the native status update or disposition operation is complete.
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ | DELETE,
        FILE_SHARE_READ | FILE_SHARE_WRITE,
        nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE) return INVALID_HANDLE_VALUE;
    char content[1024]{};
    DWORD read = 0;
    const BOOL ok = ReadFile(file, content, sizeof(content) - 1, &read, nullptr);
    if (!ok || !read)
    {
        CloseHandle(file);
        return INVALID_HANDLE_VALUE;
    }
    content[read] = '\0';
    char generation[33]{};
    if (!ReadCommandValue(content, "generation", generation, sizeof(generation)) ||
        _stricmp(generation, g_runtimeGeneration) != 0)
    {
        CloseHandle(file);
        return INVALID_HANDLE_VALUE;
    }
    return file;
}

class RuntimeOwnerFileGuard
{
public:
    explicit RuntimeOwnerFileGuard(const wchar_t* feature) :
        handle_(OpenRuntimeOwnerForGeneration(feature))
    {
    }

    ~RuntimeOwnerFileGuard()
    {
        if (handle_ != INVALID_HANDLE_VALUE) CloseHandle(handle_);
    }

    RuntimeOwnerFileGuard(const RuntimeOwnerFileGuard&) = delete;
    RuntimeOwnerFileGuard& operator=(const RuntimeOwnerFileGuard&) = delete;

    bool Valid() const
    {
        return handle_ != INVALID_HANDLE_VALUE;
    }

    bool DeleteExactOwner()
    {
        if (!Valid()) return false;
        FILE_DISPOSITION_INFO disposition{};
        disposition.DeleteFile = TRUE;
        return SetFileInformationByHandle(handle_, FileDispositionInfo, &disposition, sizeof(disposition)) != FALSE;
    }

private:
    HANDLE handle_ = INVALID_HANDLE_VALUE;
};

static bool RuntimeOwnerMatchesGeneration(const wchar_t* feature)
{
    RuntimeOwnerFileGuard owner(feature);
    return owner.Valid();
}

static void WriteRuntimeStatus(const wchar_t* feature, const wchar_t* state, const wchar_t* detail)
{
    RuntimeOwnerFileGuard owner(feature);
    if (!owner.Valid()) return;
    std::wstring path = RuntimePath((std::wstring(feature) + L".status").c_str());
    if (path.empty()) return;
    wchar_t content[1024]{};
    swprintf_s(content, L"pid=%lu\r\ncreated=%llu\r\ngeneration=%s\r\nstate=%s\r\ndetail=%s\r\n",
        GetCurrentProcessId(), CurrentProcessCreationTime(), g_runtimeGenerationWide, state, detail ? detail : L"");
    std::wstring temporary = path + L"." + g_runtimeGenerationWide + L".tmp";
    HANDLE file = CreateFileW(temporary.c_str(), GENERIC_WRITE, FILE_SHARE_READ, nullptr, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE) return;
    DWORD written = 0;
    WriteFile(file, content, static_cast<DWORD>(wcslen(content) * sizeof(wchar_t)), &written, nullptr);
    FlushFileBuffers(file);
    CloseHandle(file);
    const DWORD deadline = GetTickCount() + 2000;
    bool published = false;
    while (!(published = MoveFileExW(temporary.c_str(), path.c_str(), MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH) != FALSE))
    {
        const DWORD error = GetLastError();
        if (error != ERROR_SHARING_VIOLATION && error != ERROR_LOCK_VIOLATION && error != ERROR_ACCESS_DENIED) break;
        if (static_cast<LONG>(GetTickCount() - deadline) >= 0) break;
        Sleep(10);
    }
    if (!published) DeleteFileW(temporary.c_str());
}

static void WriteStartupFailureAfterStop(const wchar_t* feature, bool restored, const wchar_t* detail)
{
    // Stop* already published restore_failed when restoration could not be
    // proven. Never overwrite that recovery obligation with a startup error.
    if (restored) WriteRuntimeStatus(feature, L"inactive", detail);
}

static void ReleaseRuntimeOwnerAfterVerifiedStop(const wchar_t* feature, bool restored)
{
    if (!restored) return;
    RuntimeOwnerFileGuard owner(feature);
    if (owner.Valid()) owner.DeleteExactOwner();
}

static void WriteRuntimeInactiveAndReleaseOwner(const wchar_t* feature, const wchar_t* detail)
{
    WriteRuntimeStatus(feature, L"inactive", detail);
    ReleaseRuntimeOwnerAfterVerifiedStop(feature, true);
}

static bool ParseSignature(const char* signature, std::vector<int>& pattern)
{
    pattern.clear();
    const char* cursor = signature;
    while (cursor && *cursor)
    {
        while (*cursor == ' ') ++cursor;
        if (!*cursor) break;
        if (*cursor == '?')
        {
            pattern.push_back(-1);
            while (*cursor == '?') ++cursor;
        }
        else
        {
            if (!std::isxdigit(static_cast<unsigned char>(cursor[0])) ||
                !std::isxdigit(static_cast<unsigned char>(cursor[1]))) return false;
            char token[3]{ cursor[0], cursor[1], 0 };
            pattern.push_back(static_cast<int>(std::strtoul(token, nullptr, 16)));
            cursor += 2;
        }
        if (*cursor && *cursor != ' ') return false;
    }
    return !pattern.empty();
}

static bool IsReadableRegion(const MEMORY_BASIC_INFORMATION& region)
{
    if (region.State != MEM_COMMIT || (region.Protect & (PAGE_GUARD | PAGE_NOACCESS))) return false;
    switch (region.Protect & 0xFF)
    {
    case PAGE_READONLY:
    case PAGE_READWRITE:
    case PAGE_WRITECOPY:
    case PAGE_EXECUTE:
    case PAGE_EXECUTE_READ:
    case PAGE_EXECUTE_READWRITE:
    case PAGE_EXECUTE_WRITECOPY:
        return true;
    default:
        return false;
    }
}

static lm_address_t FindUniqueSignature(const char* signature, const lm_module_t& module)
{
    std::vector<int> pattern;
    if (!ParseSignature(signature, pattern) || module.base == LM_ADDRESS_BAD || module.size < pattern.size()) return LM_ADDRESS_BAD;
    const uintptr_t moduleStart = static_cast<uintptr_t>(module.base);
    if (module.size > UINTPTR_MAX - moduleStart) return LM_ADDRESS_BAD;
    const uintptr_t moduleEnd = moduleStart + module.size;
    lm_address_t unique = LM_ADDRESS_BAD;
    uintptr_t cursor = moduleStart;
    while (cursor < moduleEnd)
    {
        MEMORY_BASIC_INFORMATION region{};
        if (!VirtualQuery(reinterpret_cast<const void*>(cursor), &region, sizeof(region))) break;
        const uintptr_t regionStart = max(cursor, reinterpret_cast<uintptr_t>(region.BaseAddress));
        const uintptr_t rawRegionBase = reinterpret_cast<uintptr_t>(region.BaseAddress);
        const uintptr_t rawRegionEnd = region.RegionSize > UINTPTR_MAX - rawRegionBase ? moduleEnd : rawRegionBase + region.RegionSize;
        const uintptr_t regionEnd = min(moduleEnd, rawRegionEnd);
        if (regionEnd <= cursor) break;
        if (IsReadableRegion(region) && regionEnd - regionStart >= pattern.size())
        {
            const uintptr_t last = regionEnd - pattern.size();
            for (uintptr_t address = regionStart; address <= last; ++address)
            {
                const auto* bytes = reinterpret_cast<const uint8_t*>(address);
                bool matches = true;
                for (size_t index = 0; index < pattern.size(); ++index)
                {
                    if (pattern[index] >= 0 && bytes[index] != static_cast<uint8_t>(pattern[index]))
                    {
                        matches = false;
                        break;
                    }
                }
                if (!matches) continue;
                if (unique != LM_ADDRESS_BAD) return LM_ADDRESS_BAD;
                unique = static_cast<lm_address_t>(address);
            }
        }
        cursor = regionEnd;
    }
    return unique;
}

static LONG g_autoOverdrivePhase = 0;

static constexpr size_t kPlayerPointerCount = 8;

struct PlayerPointerState
{
    volatile LONG index;
    volatile LONG damageEnabled;
    float damageScale;
    uint32_t padding;
    uintptr_t pointers[kPlayerPointerCount];
};

static HANDLE g_playerPointerMapping = nullptr;
static PlayerPointerState* g_playerPointerState = nullptr;
static const wchar_t* kPlayerPointerName = L"Local\\GBFRPlayerInfoEditPlayerPointersV1";

static bool InitPlayerPointers()
{
    if (g_playerPointerState) return true;

    g_playerPointerMapping = CreateFileMappingW(INVALID_HANDLE_VALUE, nullptr, PAGE_READWRITE, 0, sizeof(PlayerPointerState), kPlayerPointerName);
    if (!g_playerPointerMapping) return false;

    g_playerPointerState = reinterpret_cast<PlayerPointerState*>(MapViewOfFile(g_playerPointerMapping, FILE_MAP_ALL_ACCESS, 0, 0, sizeof(PlayerPointerState)));
    if (!g_playerPointerState)
    {
        CloseHandle(g_playerPointerMapping);
        g_playerPointerMapping = nullptr;
        return false;
    }
    return true;
}

static void ClosePlayerPointers()
{
    if (g_playerPointerState)
    {
        UnmapViewOfFile(g_playerPointerState);
        g_playerPointerState = nullptr;
    }
    if (g_playerPointerMapping)
    {
        CloseHandle(g_playerPointerMapping);
        g_playerPointerMapping = nullptr;
    }
}

struct PatchPoint
{
    const char* id;
    const wchar_t* name;
    lm_address_t rva;
    const lm_byte_t* expected;
    lm_size_t size;
    const lm_byte_t* patch;
    bool hook;
};

static const lm_byte_t kMonsterHpExpected[] = { 0x48, 0x8B, 0x41, 0x10, 0x45, 0x31, 0xC9 };
// The shared health-delta helper receives the health component in rcx and the
// already-calculated new health value in rdx.
static const lm_byte_t kMonsterDamageNewExpected[] = {
    0x48, 0x89, 0x51, 0x10, 0xC3,
    0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC,
};
static const lm_byte_t kStunExpected[] = { 0xC5, 0xFA, 0x58, 0x86, 0x60, 0x08, 0x00, 0x00 };
static const lm_byte_t kMonsterDamageExpected[] = { 0x81, 0xBE, 0xD4, 0x00, 0x00, 0x00, 0x00, 0xE1, 0xF5, 0x05 };
static const lm_byte_t kInventorySet45Expected[] = { 0x41, 0x01, 0x76, 0x04, 0x4C, 0x89, 0xE1 };
static const lm_byte_t kOverdriveExpected[] = { 0x8B, 0x46, 0x10, 0x83, 0xF8, 0x03 };
static const lm_byte_t kOdRateExpected[] = {
    0x80, 0x79, 0x50, 0x00, 0x74, 0x13, 0x48, 0x03, 0x51, 0x18,
    0x48, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF, 0x48, 0x0F, 0x43,
    0xC2, 0x48, 0x89, 0x41, 0x18, 0xC3,
};

static const PatchPoint kMonsterPatches[] = {
    { "monster_hp", L"monster hp", 0x1F7A820, kMonsterHpExpected, sizeof(kMonsterHpExpected), nullptr, true },
    { "monster_damage_new", L"monster damage party", 0x1F7A810, kMonsterDamageNewExpected, sizeof(kMonsterDamageNewExpected), nullptr, true },
    { "monster_damage", L"monster damage", 0x1FBDEB4, kMonsterDamageExpected, sizeof(kMonsterDamageExpected), nullptr, true },
    { "monster_stun", L"monster stun", 0xB29128, kStunExpected, sizeof(kStunExpected), nullptr, true },
    { "overdrive_state", L"overdrive state", 0x22CB316, kOverdriveExpected, sizeof(kOverdriveExpected), nullptr, true },
    { "od_rate", L"od gauge rate", 0x22C5E50, kOdRateExpected, sizeof(kOdRateExpected), nullptr, true },
    { "inventory_set_45", L"inventory set 45", 0x356621, kInventorySet45Expected, sizeof(kInventorySet45Expected), nullptr, true },
};

static bool BytesEqual(const lm_byte_t* a, const lm_byte_t* b, lm_size_t size)
{
    for (lm_size_t i = 0; i < size; ++i)
    {
        if (a[i] != b[i]) return false;
    }
    return true;
}

class ScopedOtherThreadSuspension
{
public:
    ScopedOtherThreadSuspension()
    {
        HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0);
        if (snapshot == INVALID_HANDLE_VALUE) return;

        const DWORD processId = GetCurrentProcessId();
        const DWORD currentThreadId = GetCurrentThreadId();
        THREADENTRY32 entry{};
        entry.dwSize = sizeof(entry);
        bool openFailed = false;
        if (!Thread32First(snapshot, &entry))
        {
            CloseHandle(snapshot);
            return;
        }
        do
        {
            if (entry.th32OwnerProcessID != processId || entry.th32ThreadID == currentThreadId) continue;
            HANDLE thread = OpenThread(THREAD_SUSPEND_RESUME | THREAD_GET_CONTEXT | THREAD_QUERY_INFORMATION, FALSE, entry.th32ThreadID);
            if (thread)
            {
                threads_.push_back(thread);
            }
            else if (GetLastError() != ERROR_INVALID_PARAMETER)
            {
                openFailed = true;
                break;
            }
        } while (Thread32Next(snapshot, &entry));
        CloseHandle(snapshot);
        if (openFailed)
        {
            CloseThreads();
            return;
        }

        for (size_t index = 0; index < threads_.size(); ++index)
        {
            if (SuspendThread(threads_[index]) == static_cast<DWORD>(-1))
            {
                for (size_t restore = 0; restore < index; ++restore) ResumeThread(threads_[restore]);
                CloseThreads();
                return;
            }
        }
        active_ = true;
    }

    ~ScopedOtherThreadSuspension()
    {
        if (active_)
        {
            for (HANDLE thread : threads_) ResumeThread(thread);
        }
        CloseThreads();
    }

    bool Active() const { return active_; }

    template <size_t RangeCount>
    bool InstructionPointersOutside(const std::array<std::pair<lm_address_t, lm_size_t>, RangeCount>& ranges) const
    {
        if (!active_) return false;
        for (HANDLE thread : threads_)
        {
            CONTEXT context{};
            context.ContextFlags = CONTEXT_CONTROL;
            if (!GetThreadContext(thread, &context)) return false;
#ifdef _WIN64
            const lm_address_t instruction = static_cast<lm_address_t>(context.Rip);
#else
            const lm_address_t instruction = static_cast<lm_address_t>(context.Eip);
#endif
            for (const auto& range : ranges)
            {
                if (range.first == LM_ADDRESS_BAD || !range.second) continue;
                if (instruction >= range.first && instruction - range.first < range.second) return false;
            }
        }
        return true;
    }

private:
    void CloseThreads()
    {
        for (HANDLE thread : threads_) CloseHandle(thread);
        threads_.clear();
    }

    std::vector<HANDLE> threads_;
    bool active_ = false;
};

static bool PatchBytesWhileSuspended(lm_address_t target, const lm_byte_t* patch, lm_size_t size)
{
    lm_prot_t oldProt{};
    if (!LM_ProtMemory(target, size, LM_PROT_XRW, &oldProt)) return false;

    bool ok = LM_WriteMemory(target, patch, size) == size;
    if (ok)
    {
        std::vector<lm_byte_t> verified(size);
        ok = LM_ReadMemory(target, verified.data(), size) == size && memcmp(verified.data(), patch, size) == 0;
    }
    LM_ProtMemory(target, size, oldProt, nullptr);
    FlushInstructionCache(GetCurrentProcess(), reinterpret_cast<void*>(target), size);
    return ok;
}

static bool PatchBytes(lm_address_t target, const lm_byte_t* patch, lm_size_t size)
{
    // Camera, audio, virtual-sigil and other companions are loaded as separate
    // DLL copies. Their process-watchdog threads can all restore at once when
    // the desktop owner exits. Serialize that cross-module critical section so
    // one copy cannot suspend another halfway through its own restoration.
    wchar_t mutexName[96]{};
    swprintf_s(mutexName, L"Local\\GBFRPatchCoreBytes-%lu", GetCurrentProcessId());
    HANDLE patchMutex = CreateMutexW(nullptr, FALSE, mutexName);
    if (!patchMutex) return false;
    const DWORD wait = WaitForSingleObject(patchMutex, 10000);
    if (wait != WAIT_OBJECT_0 && wait != WAIT_ABANDONED)
    {
        CloseHandle(patchMutex);
        return false;
    }
    ScopedOtherThreadSuspension suspension;
    const bool restored = suspension.Active() && PatchBytesWhileSuspended(target, patch, size);
    ReleaseMutex(patchMutex);
    CloseHandle(patchMutex);
    return restored;
}

static const lm_byte_t kMonsterCaveMarker[] = { 'G', 'B', 'F', 'R', 'M', 'H', '0', '3' };

static lm_address_t MonsterPatchRva203(const char* id)
{
    if (strcmp(id, "monster_hp") == 0) return 0x1F74710;
    if (strcmp(id, "monster_damage_new") == 0) return 0x1F74700;
    if (strcmp(id, "monster_stun") == 0) return 0xB228A8;
    if (strcmp(id, "overdrive_state") == 0) return 0x22C5986;
    if (strcmp(id, "od_rate") == 0) return 0x22C5E50;
    return LM_ADDRESS_BAD;
}

static lm_address_t MonsterPatchRva204(const char* id)
{
    if (strcmp(id, "monster_hp") == 0) return 0x1F756B0;
    if (strcmp(id, "monster_damage_new") == 0) return 0x1F756A0;
    if (strcmp(id, "monster_stun") == 0) return 0xB23848;
    if (strcmp(id, "overdrive_state") == 0) return 0x22C6926;
    if (strcmp(id, "od_rate") == 0) return 0x22C6DF0;
    return LM_ADDRESS_BAD;
}

static lm_size_t MonsterPatchCaveSize(const char* id)
{
    if (strcmp(id, "monster_damage_new") == 0) return 512;
    if (strcmp(id, "monster_damage") == 0) return 192;
    if (strcmp(id, "overdrive_state") == 0) return 128;
    if (strcmp(id, "od_rate") == 0) return 96;
    if (strcmp(id, "inventory_set_45") == 0) return 32;
    if (strcmp(id, "monster_hp") == 0 || strcmp(id, "monster_stun") == 0) return 128;
    return 0;
}

static bool IsMarkedMonsterHook(lm_address_t target, const char* id)
{
    lm_byte_t entry[5]{};
    if (LM_ReadMemory(target, entry, sizeof(entry)) != sizeof(entry) || entry[0] != 0xE9) return false;
    int32_t delta = 0;
    memcpy(&delta, entry + 1, sizeof(delta));
    const int64_t cave64 = static_cast<int64_t>(target) + 5 + delta;
    const lm_size_t caveSize = MonsterPatchCaveSize(id);
    if (cave64 <= 0 || caveSize < sizeof(kMonsterCaveMarker)) return false;
    lm_byte_t marker[sizeof(kMonsterCaveMarker)]{};
    const lm_address_t markerAddress = static_cast<lm_address_t>(cave64) + caveSize - sizeof(marker);
    return LM_ReadMemory(markerAddress, marker, sizeof(marker)) == sizeof(marker) &&
        BytesEqual(marker, kMonsterCaveMarker, sizeof(marker));
}
static HMODULE g_patchCoreModule = nullptr;
static std::atomic<bool> g_patchCoreCanUnload{ true };

static bool RestoreLibmemHookAfterDrain(lm_address_t target, lm_address_t trampoline, lm_size_t* hookSize,
	const lm_byte_t* original, size_t originalCapacity, std::atomic<LONG>& callbacks)
{
	if (!hookSize || !*hookSize) return true;
	lm_size_t size = *hookSize;
	if (size > originalCapacity || !PatchBytes(target, original, size))
	{
		g_patchCoreCanUnload.store(false);
		return false;
	}
	DWORD deadline = GetTickCount() + 5000;
	while (callbacks.load() != 0 && static_cast<LONG>(GetTickCount() - deadline) < 0) Sleep(1);
	if (callbacks.load() != 0 || !LM_UnhookCode(target, trampoline, size))
	{
		g_patchCoreCanUnload.store(false);
		return false;
	}
	*hookSize = 0;
	return true;
}

static bool StampMonsterCave(lm_address_t cave, lm_size_t caveSize, wchar_t* message, size_t messageSize)
{
    if (cave == LM_ADDRESS_BAD || caveSize < sizeof(kMonsterCaveMarker))
    {
        swprintf_s(message, messageSize, L"invalid monster cave marker range");
        return false;
    }
    lm_address_t marker = cave + caveSize - sizeof(kMonsterCaveMarker);
    if (LM_WriteMemory(marker, kMonsterCaveMarker, sizeof(kMonsterCaveMarker)) != sizeof(kMonsterCaveMarker))
    {
        swprintf_s(message, messageSize, L"monster cave marker write failed");
        return false;
    }
    return true;
}

static bool PatchCoreCommandPath(wchar_t* commandPath, size_t commandPathSize)
{
    if (!commandPath || commandPathSize < MAX_PATH) return false;
	DWORD moduleLength = GetModuleFileNameW(g_patchCoreModule, commandPath, static_cast<DWORD>(commandPathSize));
	if (!moduleLength || moduleLength >= commandPathSize - 9) return false;
	wcscat_s(commandPath, commandPathSize, L".command");
    return true;
}

static bool ReadPatchCoreCommand(char* content, DWORD contentSize, DWORD* bytesRead = nullptr)
{
    if (!content || contentSize < 2) return false;
	wchar_t commandPath[MAX_PATH]{};
    if (!PatchCoreCommandPath(commandPath, _countof(commandPath))) return false;

    HANDLE file = CreateFileW(commandPath, GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE, nullptr, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr);
    if (file == INVALID_HANDLE_VALUE) return false;

    DWORD read = 0;
    BOOL ok = ReadFile(file, content, contentSize - 1, &read, nullptr);
    CloseHandle(file);
    if (!ok || read == 0) return false;

    content[read] = '\0';
    if (bytesRead) *bytesRead = read;
    return true;
}

static bool ReadPatchId(char* patchId, DWORD patchIdSize)
{
    DWORD read = 0;
    if (!ReadPatchCoreCommand(patchId, patchIdSize, &read)) return false;
    for (DWORD i = 0; i < read; ++i)
    {
        if (patchId[i] == '\r' || patchId[i] == '\n')
        {
            patchId[i] = '\0';
            break;
        }
    }
    return patchId[0] != '\0';
}

static bool ReadCommandUint64(const char* content, const char* key, uint64_t& value)
{
    if (!content || !key || !*key) return false;
    const size_t keyLength = strlen(key);
    const char* line = content;
    while (line && *line)
    {
        const char* end = strpbrk(line, "\r\n");
        const size_t lineLength = end ? static_cast<size_t>(end - line) : strlen(line);
        if (lineLength > keyLength && strncmp(line, key, keyLength) == 0 && line[keyLength] == '=')
        {
            char* parsedEnd = nullptr;
            unsigned long long parsed = _strtoui64(line + keyLength + 1, &parsedEnd, 10);
            if (parsedEnd == line + keyLength + 1 || parsed == 0) return false;
            value = static_cast<uint64_t>(parsed);
            return true;
        }
        if (!end) break;
        line = end + 1;
        if (*end == '\r' && *line == '\n') ++line;
    }
    return false;
}

class RuntimeOwnerGuard
{
public:
    RuntimeOwnerGuard() = default;
    ~RuntimeOwnerGuard()
    {
        if (process_) CloseHandle(process_);
    }

    RuntimeOwnerGuard(const RuntimeOwnerGuard&) = delete;
    RuntimeOwnerGuard& operator=(const RuntimeOwnerGuard&) = delete;

    bool OpenFromCommand(const wchar_t* feature)
    {
        char content[512]{};
        if (!ReadPatchCoreCommand(content, sizeof(content))) return false;
        uint64_t ownerPid = 0;
        uint64_t ownerCreated = 0;
        char generation[33]{};
        if (!ReadCommandUint64(content, "owner_pid", ownerPid) ||
            !ReadCommandUint64(content, "owner_created", ownerCreated) ||
            !ReadCommandValue(content, "generation", generation, sizeof(generation)) ||
            !SetRuntimeGeneration(generation) ||
            !RuntimeOwnerMatchesGeneration(feature) ||
            ownerPid > MAXDWORD) return false;

        process_ = OpenProcess(SYNCHRONIZE | PROCESS_QUERY_LIMITED_INFORMATION, FALSE, static_cast<DWORD>(ownerPid));
        if (!process_) return false;
        FILETIME creation{}, exit{}, kernel{}, user{};
        if (!GetProcessTimes(process_, &creation, &exit, &kernel, &user))
        {
            CloseHandle(process_);
            process_ = nullptr;
            return false;
        }
        ULARGE_INTEGER actual{};
        actual.LowPart = creation.dwLowDateTime;
        actual.HighPart = creation.dwHighDateTime;
        if (actual.QuadPart != ownerCreated)
        {
            CloseHandle(process_);
            process_ = nullptr;
            return false;
        }
        return Alive() && RuntimeOwnerMatchesGeneration(feature);
    }

    bool Alive() const
    {
        return process_ && WaitForSingleObject(process_, 0) == WAIT_TIMEOUT;
    }

private:
    HANDLE process_ = nullptr;
};

static bool PatchIdEquals(const char* requestedId, const char* pointId)
{
    size_t n = strlen(pointId);
    return strncmp(requestedId, pointId, n) == 0 && (requestedId[n] == '\0' || requestedId[n] == ' ' || requestedId[n] == '\t');
}

static int ReadIntValue(int defaultValue)
{
    char patchId[64]{};
    if (!ReadPatchId(patchId, sizeof(patchId))) return defaultValue;

    char* space = strchr(patchId, ' ');
    if (!space) return defaultValue;
    return atoi(space + 1);
}

static float ReadScale()
{
    char patchId[64]{};
    if (!ReadPatchId(patchId, sizeof(patchId))) return 1.0f;

    char* space = strchr(patchId, ' ');
    if (!space) return 1.0f;
    float scale = static_cast<float>(atof(space + 1));
    if (scale <= 0.0f || scale > 9999.0f) return 1.0f;
    return scale;
}

static bool ConfigurePlayerDamage(wchar_t* message, size_t messageSize)
{
    if (!InitPlayerPointers())
    {
        swprintf_s(message, messageSize, L"player pointer mapping failed");
        return false;
    }

    char patchId[64]{};
    if (!ReadPatchId(patchId, sizeof(patchId)) ||
        (!PatchIdEquals(patchId, "monster_damage") && !PatchIdEquals(patchId, "monster_damage_new")))
    {
        swprintf_s(message, messageSize, L"player damage command missing");
        return false;
    }

    char* space = strchr(patchId, ' ');
    float scale = space ? static_cast<float>(atof(space + 1)) : 0.0f;
    if (scale <= 0.0f || scale > 9999.0f)
    {
        swprintf_s(message, messageSize, L"invalid player damage scale");
        return false;
    }
    g_playerPointerState->damageScale = scale;
    InterlockedExchange(&g_playerPointerState->damageEnabled, 1);
    return true;
}

static lm_address_t AllocNear(lm_address_t target, size_t size)
{
    const uintptr_t granularity = 0x10000;
    const uintptr_t maxDistance = 0x7FFF0000;
    uintptr_t base = target & ~(granularity - 1);

    for (uintptr_t step = 0; step <= maxDistance; step += granularity)
    {
        uintptr_t candidates[2]{};
        int count = 0;
        if (base >= step) candidates[count++] = base - step;
        if (base <= UINTPTR_MAX - step) candidates[count++] = base + step;

        for (int i = 0; i < count; ++i)
        {
            void* ptr = VirtualAlloc(reinterpret_cast<void*>(candidates[i]), size, MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);
            if (!ptr) continue;

            int64_t delta = static_cast<int64_t>(reinterpret_cast<uintptr_t>(ptr)) - static_cast<int64_t>(target + 5);
            if (delta >= INT32_MIN && delta <= INT32_MAX)
            {
                return reinterpret_cast<lm_address_t>(ptr);
            }
            VirtualFree(ptr, 0, MEM_RELEASE);
        }
    }

    return LM_ADDRESS_BAD;
}

static bool PatchDamageHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    float scale = ReadScale();
    lm_address_t cave = AllocNear(target, 128);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: monster hp");
        return false;
    }

    lm_byte_t code[96]{};
    size_t i = 0;
    code[i++] = 0x41; code[i++] = 0x52;                                                             // push r10
    code[i++] = 0x48; code[i++] = 0x83; code[i++] = 0xEC; code[i++] = 0x10;                         // sub rsp,10
    code[i++] = 0x0F; code[i++] = 0x11; code[i++] = 0x04; code[i++] = 0x24;                         // movups [rsp],xmm0
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x2A; code[i++] = 0xC2;                         // cvtsi2ss xmm0,edx
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x59; code[i++] = 0x05;                         // mulss xmm0,[rip+disp32]
    size_t scaleDisp = i; i += 4;
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x2C; code[i++] = 0xD0;                         // cvttss2si edx,xmm0
    code[i++] = 0x85; code[i++] = 0xD2;                                                             // test edx,edx
    code[i++] = 0x7F; size_t jgScaled = i++;                                                        // jg scaled
    code[i++] = 0xBA; code[i++] = 0x01; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;       // mov edx,1
    size_t scaledOffset = i;
    code[i++] = 0x0F; code[i++] = 0x10; code[i++] = 0x04; code[i++] = 0x24;                         // movups xmm0,[rsp]
    code[i++] = 0x48; code[i++] = 0x83; code[i++] = 0xC4; code[i++] = 0x10;                         // add rsp,10
    code[i++] = 0x41; code[i++] = 0x5A;                                                             // pop r10
    code[i++] = 0x48; code[i++] = 0x8B; code[i++] = 0x41; code[i++] = 0x10;                         // mov rax,[rcx+10]
    code[i++] = 0x45; code[i++] = 0x31; code[i++] = 0xC9;                                           // xor r9d,r9d
    code[i++] = 0xE9;                                                                               // jmp return
    size_t jmpBackDisp = i; i += 4;
    size_t scaleOffset = i;
    memcpy(code + i, &scale, sizeof(scale)); i += sizeof(scale);

    code[jgScaled] = static_cast<lm_byte_t>(scaledOffset - (jgScaled + 1));

    int64_t scaleDelta = static_cast<int64_t>(cave + scaleOffset) - static_cast<int64_t>(cave + scaleDisp + 4);
    int64_t backDelta = static_cast<int64_t>(target + 7) - static_cast<int64_t>(cave + jmpBackDisp + 4);
    if (scaleDelta < INT32_MIN || scaleDelta > INT32_MAX || backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"jump out of range: monster hp");
        return false;
    }
    int32_t relScale = static_cast<int32_t>(scaleDelta);
    int32_t relBack = static_cast<int32_t>(backDelta);
    memcpy(code + scaleDisp, &relScale, sizeof(relScale));
    memcpy(code + jmpBackDisp, &relBack, sizeof(relBack));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: monster hp");
        return false;
    }
    if (!StampMonsterCave(cave, 128, message, messageSize)) return false;

    lm_byte_t jmp[7]{ 0xE9 };
    memset(jmp + 5, 0x90, sizeof(jmp) - 5);
    int32_t rel = static_cast<int32_t>(cave - (target + 5));
    memcpy(jmp + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jmp, sizeof(jmp)))
    {
        swprintf_s(message, messageSize, L"hook write failed: monster hp");
        return false;
    }
    return true;
}

static bool InstallPlayerPointerHook(const lm_module_t& module, wchar_t* message, size_t messageSize)
{
    if (!g_playerPointerState)
    {
        swprintf_s(message, messageSize, L"player pointer mapping is unavailable");
        return false;
    }

    lm_address_t match = LM_SigScan("FF 90 ?? ?? 00 00 ?? ?? ?? ?? ?? ?? ?? ?? 8B ?? ?? ?? 00 00 48 81 C1 ?? ?? 00 00 FF ?? ?? ?? 00 00 ?? 39", module.base, module.size);
    if (match == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"player pointer signature not found");
        return false;
    }

    lm_address_t target = match + 0x14;
    lm_byte_t original[7]{};
    if (LM_ReadMemory(target, original, sizeof(original)) != sizeof(original))
    {
        swprintf_s(message, messageSize, L"read failed: player pointer instruction");
        return false;
    }
    if (original[0] != 0x48 || original[1] != 0x81 || original[2] != 0xC1)
    {
        swprintf_s(message, messageSize, L"unexpected player pointer instruction");
        return false;
    }

    lm_address_t cave = AllocNear(target, 96);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: player pointers");
        return false;
    }

    lm_byte_t code[96]{};
    size_t i = 0;
    code[i++] = 0x50;
    code[i++] = 0x52;
    code[i++] = 0x48; code[i++] = 0xBA;
    uintptr_t stateAddr = reinterpret_cast<uintptr_t>(g_playerPointerState);
    memcpy(code + i, &stateAddr, sizeof(stateAddr)); i += sizeof(stateAddr);
    code[i++] = 0x48; code[i++] = 0x85; code[i++] = 0xD2;
    code[i++] = 0x74; size_t jzSkipCapture = i++;
    code[i++] = 0x8B; code[i++] = 0x02;
    code[i++] = 0xFF; code[i++] = 0x02;
    code[i++] = 0x83; code[i++] = 0xE0; code[i++] = 0x07;
    code[i++] = 0x48; code[i++] = 0x89; code[i++] = 0x4C; code[i++] = 0xC2; code[i++] = 0x10;
    size_t skipCaptureOffset = i;
    code[jzSkipCapture] = static_cast<lm_byte_t>(skipCaptureOffset - (jzSkipCapture + 1));
    code[i++] = 0x5A;
    code[i++] = 0x58;
    memcpy(code + i, original, sizeof(original)); i += sizeof(original);
    code[i++] = 0xE9;
    size_t returnDisp = i; i += 4;

    int64_t backDelta = static_cast<int64_t>(target + sizeof(original)) - static_cast<int64_t>(cave + returnDisp + 4);
    if (backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"return jump out of range: player pointers");
        return false;
    }
    int32_t relBack = static_cast<int32_t>(backDelta);
    memcpy(code + returnDisp, &relBack, sizeof(relBack));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: player pointers");
        return false;
    }
    if (!StampMonsterCave(cave, 96, message, messageSize)) return false;

    lm_byte_t jump[7]{ 0xE9 };
    memset(jump + 5, 0x90, sizeof(jump) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook jump out of range: player pointers");
        return false;
    }
    int32_t rel = static_cast<int32_t>(hookDelta);
    memcpy(jump + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jump, sizeof(jump)))
    {
        swprintf_s(message, messageSize, L"hook write failed: player pointers");
        return false;
    }
    return true;
}

static bool PatchMonsterDamageNewHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    lm_address_t cave = AllocNear(target, 512);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: monster damage party");
        return false;
    }

    lm_byte_t code[512]{};
    size_t i = 0;
    size_t playerJumps[kPlayerPointerCount]{};
    code[i++] = 0x50;                                                                               // push rax
    code[i++] = 0x41; code[i++] = 0x50;                                                             // push r8
    code[i++] = 0x41; code[i++] = 0x51;                                                             // push r9
    code[i++] = 0x41; code[i++] = 0x52;                                                             // push r10
    code[i++] = 0x41; code[i++] = 0x53;                                                             // push r11
    code[i++] = 0x48; code[i++] = 0x83; code[i++] = 0xEC; code[i++] = 0x10;                         // sub rsp,10
    code[i++] = 0x0F; code[i++] = 0x11; code[i++] = 0x04; code[i++] = 0x24;                         // movups [rsp],xmm0
    code[i++] = 0x49; code[i++] = 0xBA;
    uintptr_t stateAddr = reinterpret_cast<uintptr_t>(g_playerPointerState);
    memcpy(code + i, &stateAddr, sizeof(stateAddr)); i += sizeof(stateAddr);
    code[i++] = 0x4D; code[i++] = 0x85; code[i++] = 0xD2;                                           // test r10,r10
    code[i++] = 0x74; size_t jzRestore = i++;                                                       // je restore
    code[i++] = 0x41; code[i++] = 0x83; code[i++] = 0x7A; code[i++] = 0x04; code[i++] = 0x00;       // cmp dword ptr [r10+4],0
    code[i++] = 0x74; size_t jzDisabled = i++;                                                      // je restore
    code[i++] = 0x4C; code[i++] = 0x8D; code[i++] = 0x99; code[i++] = 0xB0; code[i++] = 0xFE; code[i++] = 0xFF; code[i++] = 0xFF; // lea r11,[rcx-150]
    for (size_t player = 0; player < kPlayerPointerCount; ++player)
    {
        code[i++] = 0x4D; code[i++] = 0x3B; code[i++] = 0x5A; code[i++] = static_cast<lm_byte_t>(0x10 + player * 8); // cmp r11,[r10+player]
        code[i++] = 0x74; playerJumps[player] = i++;                                                // je scale
    }
    code[i++] = 0xEB; size_t jmpRestore = i++;                                                      // jmp restore
    size_t scaleOffset = i;
    code[i++] = 0x4C; code[i++] = 0x8B; code[i++] = 0x59; code[i++] = 0x10;                         // mov r11,[rcx+10]
    code[i++] = 0x4D; code[i++] = 0x89; code[i++] = 0xD9;                                           // mov r9,r11
    code[i++] = 0x49; code[i++] = 0x39; code[i++] = 0xD3;                                           // cmp r11,rdx
    code[i++] = 0x72; size_t jbRestore = i++;                                                       // jb restore (healing)
    code[i++] = 0x49; code[i++] = 0x29; code[i++] = 0xD3;                                           // sub r11,rdx
    code[i++] = 0xF3; code[i++] = 0x49; code[i++] = 0x0F; code[i++] = 0x2A; code[i++] = 0xC3;       // cvtsi2ss xmm0,r11
    code[i++] = 0xF3; code[i++] = 0x41; code[i++] = 0x0F; code[i++] = 0x59; code[i++] = 0x42; code[i++] = 0x08; // mulss xmm0,[r10+8]
    code[i++] = 0xF3; code[i++] = 0x48; code[i++] = 0x0F; code[i++] = 0x2C; code[i++] = 0xC0;       // cvttss2si rax,xmm0
    code[i++] = 0x49; code[i++] = 0x39; code[i++] = 0xC1;                                           // cmp r9,rax
    code[i++] = 0x4C; code[i++] = 0x0F; code[i++] = 0x42; code[i++] = 0xC8;                         // cmovb r9,rax
    code[i++] = 0x49; code[i++] = 0x29; code[i++] = 0xC1;                                           // sub r9,rax
    code[i++] = 0x4C; code[i++] = 0x89; code[i++] = 0xCA;                                           // mov rdx,r9
    size_t restoreOffset = i;
    code[i++] = 0x0F; code[i++] = 0x10; code[i++] = 0x04; code[i++] = 0x24;                         // movups xmm0,[rsp]
    code[i++] = 0x48; code[i++] = 0x83; code[i++] = 0xC4; code[i++] = 0x10;                         // add rsp,10
    code[i++] = 0x41; code[i++] = 0x5B;                                                             // pop r11
    code[i++] = 0x41; code[i++] = 0x5A;                                                             // pop r10
    code[i++] = 0x41; code[i++] = 0x59;                                                             // pop r9
    code[i++] = 0x41; code[i++] = 0x58;                                                             // pop r8
    code[i++] = 0x58;                                                                               // pop rax
    code[i++] = 0x48; code[i++] = 0x89; code[i++] = 0x51; code[i++] = 0x10;                         // mov [rcx+10],rdx
    code[i++] = 0xC3;                                                                               // ret

    for (size_t player = 0; player < kPlayerPointerCount; ++player)
    {
        code[playerJumps[player]] = static_cast<lm_byte_t>(scaleOffset - (playerJumps[player] + 1));
    }
    code[jzRestore] = static_cast<lm_byte_t>(restoreOffset - (jzRestore + 1));
    code[jzDisabled] = static_cast<lm_byte_t>(restoreOffset - (jzDisabled + 1));
    code[jmpRestore] = static_cast<lm_byte_t>(restoreOffset - (jmpRestore + 1));
    code[jbRestore] = static_cast<lm_byte_t>(restoreOffset - (jbRestore + 1));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: monster damage party");
        return false;
    }
    if (!StampMonsterCave(cave, 512, message, messageSize)) return false;

    lm_byte_t jump[sizeof(kMonsterDamageNewExpected)]{ 0xE9 };
    memset(jump + 5, 0x90, sizeof(jump) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook jump out of range: monster damage party");
        return false;
    }
    int32_t rel = static_cast<int32_t>(hookDelta);
    memcpy(jump + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jump, sizeof(jump)))
    {
        swprintf_s(message, messageSize, L"hook write failed: monster damage party");
        return false;
    }
    return true;
}

static bool PatchMonsterDamageHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    lm_address_t cave = AllocNear(target, 192);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: monster damage");
        return false;
    }

    lm_byte_t code[256]{};
    size_t i = 0;
    size_t playerJumps[kPlayerPointerCount]{};
    code[i++] = 0x50;
    code[i++] = 0x52;
    code[i++] = 0x48; code[i++] = 0xBA;
    uintptr_t stateAddr = reinterpret_cast<uintptr_t>(g_playerPointerState);
    memcpy(code + i, &stateAddr, sizeof(stateAddr)); i += sizeof(stateAddr);
    code[i++] = 0x48; code[i++] = 0x85; code[i++] = 0xD2;
    code[i++] = 0x74; size_t jzSkipPlayerCheck = i++;
    code[i++] = 0x83; code[i++] = 0x7A; code[i++] = 0x04; code[i++] = 0x00;
    code[i++] = 0x74; size_t jzDisabled = i++;
    for (size_t player = 0; player < kPlayerPointerCount; ++player)
    {
        code[i++] = 0x4C; code[i++] = 0x3B; code[i++] = 0x72; code[i++] = static_cast<lm_byte_t>(0x10 + player * 8);
        code[i++] = 0x74; playerJumps[player] = i++;
    }
    size_t skipPlayerCheckOffset = i;
    code[jzSkipPlayerCheck] = static_cast<lm_byte_t>(skipPlayerCheckOffset - (jzSkipPlayerCheck + 1));
    code[jzDisabled] = static_cast<lm_byte_t>(skipPlayerCheckOffset - (jzDisabled + 1));
    code[i++] = 0x5A;
    code[i++] = 0x58;
    code[i++] = 0xEB; size_t jmpSkipScale = i++;
    size_t scalePlayerDamageOffset = i;
    code[i++] = 0x48; code[i++] = 0x83; code[i++] = 0xEC; code[i++] = 0x10;
    code[i++] = 0x0F; code[i++] = 0x11; code[i++] = 0x04; code[i++] = 0x24;
    code[i++] = 0x8B; code[i++] = 0x86; code[i++] = 0xD4; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x2A; code[i++] = 0xC0;
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x59; code[i++] = 0x42; code[i++] = 0x08;
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x2C; code[i++] = 0xC0;
    code[i++] = 0x85; code[i++] = 0xC0;
    code[i++] = 0x7F; size_t jgScaled = i++;
    code[i++] = 0xB8; code[i++] = 0x01; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
    size_t scaledOffset = i;
    code[i++] = 0x89; code[i++] = 0x86; code[i++] = 0xD4; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
    code[i++] = 0x0F; code[i++] = 0x10; code[i++] = 0x04; code[i++] = 0x24;
    code[i++] = 0x48; code[i++] = 0x83; code[i++] = 0xC4; code[i++] = 0x10;
    code[i++] = 0x5A;
    code[i++] = 0x58;
    size_t compareOffset = i;
    code[jmpSkipScale] = static_cast<lm_byte_t>(compareOffset - (jmpSkipScale + 1));
    for (size_t player = 0; player < kPlayerPointerCount; ++player)
    {
        code[playerJumps[player]] = static_cast<lm_byte_t>(scalePlayerDamageOffset - (playerJumps[player] + 1));
    }
    code[i++] = 0x81; code[i++] = 0xBE; code[i++] = 0xD4; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
    code[i++] = 0x00; code[i++] = 0xE1; code[i++] = 0xF5; code[i++] = 0x05;
    code[i++] = 0xE9;
    size_t jmpBackDisp = i; i += 4;
    code[jgScaled] = static_cast<lm_byte_t>(scaledOffset - (jgScaled + 1));

    int64_t backDelta = static_cast<int64_t>(target + sizeof(kMonsterDamageExpected)) - static_cast<int64_t>(cave + jmpBackDisp + 4);
    if (backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"return jump out of range: monster damage");
        return false;
    }
    int32_t relBack = static_cast<int32_t>(backDelta);
    memcpy(code + jmpBackDisp, &relBack, sizeof(relBack));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: monster damage");
        return false;
    }
    if (!StampMonsterCave(cave, 192, message, messageSize)) return false;

    lm_byte_t jmp[sizeof(kMonsterDamageExpected)]{ 0xE9 };
    memset(jmp + 5, 0x90, sizeof(jmp) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook jump out of range: monster damage");
        return false;
    }
    int32_t rel = static_cast<int32_t>(hookDelta);
    memcpy(jmp + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jmp, sizeof(jmp)))
    {
        swprintf_s(message, messageSize, L"hook write failed: monster damage");
        return false;
    }
    return true;
}

static bool PatchOverdriveHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    int requested = ReadIntValue(3);
    bool autoMode = requested == 9;
    if (requested != 0 && requested != 3 && !autoMode) requested = 3;

    lm_address_t cave = AllocNear(target, 128);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: overdrive state");
        return false;
    }

    lm_byte_t code[96]{};
    size_t i = 0;
    if (autoMode)
    {
        code[i++] = 0x41; code[i++] = 0x53; // push r11
        code[i++] = 0x8B; code[i++] = 0x46; code[i++] = 0x10;
        code[i++] = 0x49; code[i++] = 0xBB;
        uintptr_t phaseAddr = reinterpret_cast<uintptr_t>(&g_autoOverdrivePhase);
        memcpy(code + i, &phaseAddr, sizeof(phaseAddr)); i += sizeof(phaseAddr);
        code[i++] = 0x83; code[i++] = 0xF8; code[i++] = 0x02;
        code[i++] = 0x75; size_t jneNotActive = i++;
        code[i++] = 0x41; code[i++] = 0xC7; code[i++] = 0x03; code[i++] = 0x02; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
        code[i++] = 0xEB; size_t jmpReadActive = i++;
        size_t notActiveOffset = i;
        code[i++] = 0x41; code[i++] = 0x83; code[i++] = 0x3B; code[i++] = 0x02;
        code[i++] = 0x74; size_t jeRefill = i++;
        code[i++] = 0x41; code[i++] = 0x83; code[i++] = 0x3B; code[i++] = 0x00;
        code[i++] = 0x75; size_t jneReadWaiting = i++;
        size_t refillOffset = i;
        code[i++] = 0xC7; code[i++] = 0x46; code[i++] = 0x10; code[i++] = 0x03; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
        code[i++] = 0x41; code[i++] = 0xC7; code[i++] = 0x03; code[i++] = 0x01; code[i++] = 0x00; code[i++] = 0x00; code[i++] = 0x00;
        size_t readOffset = i;
        code[jneNotActive] = static_cast<lm_byte_t>(notActiveOffset - (jneNotActive + 1));
        code[jmpReadActive] = static_cast<lm_byte_t>(readOffset - (jmpReadActive + 1));
        code[jeRefill] = static_cast<lm_byte_t>(refillOffset - (jeRefill + 1));
        code[jneReadWaiting] = static_cast<lm_byte_t>(readOffset - (jneReadWaiting + 1));
        code[i++] = 0x41; code[i++] = 0x5B; // pop r11
    }
    else
    {
        code[i++] = 0xC7; code[i++] = 0x46; code[i++] = 0x10;
        memcpy(code + i, &requested, sizeof(requested)); i += sizeof(requested);
    }
    code[i++] = 0x8B; code[i++] = 0x46; code[i++] = 0x10;
    code[i++] = 0x83; code[i++] = 0xF8; code[i++] = 0x03;
    code[i++] = 0xE9;
    size_t backDisp = i; i += 4;

    int64_t backDelta = static_cast<int64_t>(target + 6) - static_cast<int64_t>(cave + backDisp + 4);
    if (backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"return jump out of range: overdrive state");
        return false;
    }
    int32_t backRel = static_cast<int32_t>(backDelta);
    memcpy(code + backDisp, &backRel, sizeof(backRel));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: overdrive state");
        return false;
    }
    if (!StampMonsterCave(cave, 128, message, messageSize)) return false;

    lm_byte_t jmp[6]{ 0xE9 };
    memset(jmp + 5, 0x90, sizeof(jmp) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook jump out of range: overdrive state");
        return false;
    }
    int32_t rel = static_cast<int32_t>(hookDelta);
    memcpy(jmp + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jmp, sizeof(jmp)))
    {
        swprintf_s(message, messageSize, L"hook write failed: overdrive state");
        return false;
    }
    return true;
}

static bool PatchOdRateHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    const float scale = ReadScale();
    lm_address_t cave = AllocNear(target, 96);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: od gauge rate");
        return false;
    }

    lm_byte_t code[64]{};
    size_t i = 0;
    code[i++] = 0x80; code[i++] = 0x79; code[i++] = 0x50; code[i++] = 0x00;
    size_t jzDisp = i; code[i++] = 0x74; code[i++] = 0x00;
    code[i++] = 0xF3; code[i++] = 0x48; code[i++] = 0x0F; code[i++] = 0x2A; code[i++] = 0xC2;
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x59; code[i++] = 0x05;
    size_t scaleDisp = i; i += 4;
    code[i++] = 0xF3; code[i++] = 0x48; code[i++] = 0x0F; code[i++] = 0x2C; code[i++] = 0xD0;
    code[i++] = 0x48; code[i++] = 0x03; code[i++] = 0x51; code[i++] = 0x18;
    code[i++] = 0x48; code[i++] = 0xC7; code[i++] = 0xC0; code[i++] = 0xFF; code[i++] = 0xFF; code[i++] = 0xFF; code[i++] = 0xFF;
    code[i++] = 0x48; code[i++] = 0x0F; code[i++] = 0x43; code[i++] = 0xC2;
    code[i++] = 0x48; code[i++] = 0x89; code[i++] = 0x41; code[i++] = 0x18;
    size_t retOffset = i;
    code[i++] = 0xC3;
    size_t scaleOffset = i;
    memcpy(code + i, &scale, sizeof(scale)); i += sizeof(scale);
    code[jzDisp + 1] = static_cast<lm_byte_t>(retOffset - (jzDisp + 2));

    int64_t scaleDelta = static_cast<int64_t>(cave + scaleOffset) - static_cast<int64_t>(cave + scaleDisp + 4);
    if (scaleDelta < INT32_MIN || scaleDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"scale out of range: od gauge rate");
		VirtualFree(reinterpret_cast<LPVOID>(cave), 0, MEM_RELEASE);
        return false;
    }
    int32_t relScale = static_cast<int32_t>(scaleDelta);
    memcpy(code + scaleDisp, &relScale, sizeof(relScale));
    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: od gauge rate");
		VirtualFree(reinterpret_cast<LPVOID>(cave), 0, MEM_RELEASE);
        return false;
    }
    if (!StampMonsterCave(cave, 96, message, messageSize))
	{
		VirtualFree(reinterpret_cast<LPVOID>(cave), 0, MEM_RELEASE);
		return false;
	}

    lm_byte_t jump[sizeof(kOdRateExpected)]{ 0xE9 };
    memset(jump + 5, 0x90, sizeof(jump) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook out of range: od gauge rate");
		VirtualFree(reinterpret_cast<LPVOID>(cave), 0, MEM_RELEASE);
        return false;
    }
    int32_t rel = static_cast<int32_t>(hookDelta);
    memcpy(jump + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jump, sizeof(jump)))
    {
        swprintf_s(message, messageSize, L"hook write failed: od gauge rate");
		// PatchBytes can fail after the write when its readback or protection
		// restore fails. Never release a cave while the entry may already jump
		// into it. First prove the original entry, or restore and re-read it.
		lm_byte_t actual[sizeof(jump)]{};
		bool originalProven = LM_ReadMemory(target, actual, sizeof(actual)) == sizeof(actual) &&
			BytesEqual(actual, kOdRateExpected, sizeof(actual));
		if (!originalProven && BytesEqual(actual, jump, sizeof(actual)))
		{
			PatchBytes(target, kOdRateExpected, sizeof(kOdRateExpected));
			originalProven = LM_ReadMemory(target, actual, sizeof(actual)) == sizeof(actual) &&
				BytesEqual(actual, kOdRateExpected, sizeof(actual));
		}
		if (originalProven) VirtualFree(reinterpret_cast<LPVOID>(cave), 0, MEM_RELEASE);
        return false;
    }
    return true;
}

static bool PatchInventorySetQuantityHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    int quantity = ReadIntValue(45);
    if (quantity < 1 || quantity > 9999) quantity = 45;

    lm_address_t cave = AllocNear(target, 32);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: inventory quantity");
        return false;
    }

    lm_byte_t code[24]{};
    size_t i = 0;
    code[i++] = 0x41; code[i++] = 0xC7; code[i++] = 0x46; code[i++] = 0x04;                         // mov dword ptr [r14+04],quantity
    memcpy(code + i, &quantity, sizeof(quantity)); i += sizeof(quantity);
    code[i++] = 0x4C; code[i++] = 0x89; code[i++] = 0xE1;                                           // mov rcx,r12
    code[i++] = 0xE9;
    size_t jmpBackDisp = i; i += 4;

    int64_t backDelta = static_cast<int64_t>(target + 7) - static_cast<int64_t>(cave + jmpBackDisp + 4);
    if (backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"return jump out of range: inventory quantity");
        return false;
    }
    int32_t relBack = static_cast<int32_t>(backDelta);
    memcpy(code + jmpBackDisp, &relBack, sizeof(relBack));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: inventory quantity");
        return false;
    }
    if (!StampMonsterCave(cave, 32, message, messageSize)) return false;

    lm_byte_t jmp[7]{ 0xE9 };
    memset(jmp + 5, 0x90, sizeof(jmp) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook jump out of range: inventory quantity");
        return false;
    }
    int32_t relHook = static_cast<int32_t>(hookDelta);
    memcpy(jmp + 1, &relHook, sizeof(relHook));
    if (!PatchBytes(target, jmp, sizeof(jmp)))
    {
        swprintf_s(message, messageSize, L"hook write failed: inventory quantity");
        return false;
    }
    return true;
}

static bool PatchStunHook(lm_address_t target, wchar_t* message, size_t messageSize)
{
    float scale = ReadScale();
    lm_address_t cave = AllocNear(target, 128);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"alloc near failed: monster stun");
        return false;
    }

    lm_byte_t code[64]{};
    size_t i = 0;
    code[i++] = 0x50;                                                                               // push rax
    code[i++] = 0x48; code[i++] = 0x8D; code[i++] = 0x86; code[i++] = 0x60; code[i++] = 0x08; code[i++] = 0x00; code[i++] = 0x00; // lea rax,[rsi+860]
    code[i++] = 0xF3; code[i++] = 0x0F; code[i++] = 0x59; code[i++] = 0x05;                         // mulss xmm0,[rip+disp32]
    size_t scaleDisp = i; i += 4;
    code[i++] = 0xC5; code[i++] = 0xFA; code[i++] = 0x58; code[i++] = 0x00;                         // vaddss xmm0,xmm0,[rax]
    code[i++] = 0x58;                                                                               // pop rax
    code[i++] = 0xE9;                                                                               // jmp return
    size_t jmpBackDisp = i; i += 4;
    size_t scaleOffset = i;
    memcpy(code + i, &scale, sizeof(scale)); i += sizeof(scale);

    int64_t scaleDelta = static_cast<int64_t>(cave + scaleOffset) - static_cast<int64_t>(cave + scaleDisp + 4);
    if (scaleDelta < INT32_MIN || scaleDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"scale jump out of range: monster stun");
        return false;
    }
    int32_t relScale = static_cast<int32_t>(scaleDelta);
    memcpy(code + scaleDisp, &relScale, sizeof(relScale));

    int64_t backDelta = static_cast<int64_t>(target + sizeof(kStunExpected)) - static_cast<int64_t>(cave + jmpBackDisp + 4);
    if (backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"return jump out of range: monster stun");
        return false;
    }
    int32_t relBack = static_cast<int32_t>(backDelta);
    memcpy(code + jmpBackDisp, &relBack, sizeof(relBack));

    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"cave write failed: monster stun");
        return false;
    }
    if (!StampMonsterCave(cave, 128, message, messageSize)) return false;

    lm_byte_t jmp[sizeof(kStunExpected)]{ 0xE9 };
    memset(jmp + 5, 0x90, sizeof(jmp) - 5);
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"hook jump out of range: monster stun");
        return false;
    }
    int32_t rel = static_cast<int32_t>(hookDelta);
    memcpy(jmp + 1, &rel, sizeof(rel));
    if (!PatchBytes(target, jmp, sizeof(jmp)))
    {
        swprintf_s(message, messageSize, L"hook write failed: monster stun");
        return false;
    }
    return true;
}

static lm_address_t ResolveRIPGlobal(lm_address_t instruction, size_t displacementOffset, size_t instructionSize);

// Camera runtime. This is deliberately independent from any external mod loader:
// the desktop app injects this DLL and owns the configuration/status channel.
using CameraInitFunction = uintptr_t(*)();
static CameraInitFunction g_cameraOriginalInit = nullptr;
static lm_address_t g_cameraInitTarget = LM_ADDRESS_BAD;
static lm_size_t g_cameraInitHookSize = 0;
static lm_byte_t g_cameraInitOriginal[32]{};
static lm_address_t g_cameraDecreaseTarget = LM_ADDRESS_BAD;
static lm_address_t g_cameraIncreaseTarget = LM_ADDRESS_BAD;
static lm_address_t g_cameraDecreaseCave = LM_ADDRESS_BAD;
static lm_address_t g_cameraIncreaseCave = LM_ADDRESS_BAD;
static lm_address_t g_cameraSettingsGlobal = LM_ADDRESS_BAD;
static lm_byte_t g_cameraDecreaseOriginal[8]{};
static lm_byte_t g_cameraIncreaseOriginal[8]{};
static std::atomic<LONG> g_cameraCallbacks{ 0 };
static SRWLOCK g_cameraLock = SRWLOCK_INIT;
static uintptr_t g_cameraSettings = 0;
static float g_cameraOriginalDistance = 0.0f;
static float g_cameraOriginalHeight = 0.0f;
static float g_cameraAppliedDistance = 0.0f;
static float g_cameraAppliedHeight = 0.0f;
static float g_cameraDistance = 6.0f;
static float g_cameraHeight = 1.8f;
static float g_cameraIncreaseStep = 0.02f;
static float g_cameraDecreaseStep = -0.02f;
static bool g_cameraHasApplied = false;
static std::atomic<bool> g_cameraStopping{ false };

static bool MemoryRegionAllows(uintptr_t address, size_t size, bool write)
{
    if (!address || !size || address + size < address) return false;
    MEMORY_BASIC_INFORMATION info{};
    if (!VirtualQuery(reinterpret_cast<void*>(address), &info, sizeof(info))) return false;
    if (info.State != MEM_COMMIT || (info.Protect & PAGE_GUARD) || (info.Protect & PAGE_NOACCESS)) return false;
    uintptr_t regionEnd = reinterpret_cast<uintptr_t>(info.BaseAddress) + info.RegionSize;
    if (address + size > regionEnd) return false;
    if (!write) return true;
    DWORD protection = info.Protect & 0xFF;
    return protection == PAGE_READWRITE || protection == PAGE_WRITECOPY || protection == PAGE_EXECUTE_READWRITE || protection == PAGE_EXECUTE_WRITECOPY;
}

static bool CameraValuePlausible(float value)
{
    return std::isfinite(value) && std::fabs(value) <= 10000.0f;
}

static void RestoreCameraValuesLocked()
{
    if (!g_cameraSettings || !g_cameraHasApplied || !MemoryRegionAllows(g_cameraSettings + 0x34, sizeof(float), true) ||
        !MemoryRegionAllows(g_cameraSettings + 0x54, sizeof(float), true)) return;
    float* distance = reinterpret_cast<float*>(g_cameraSettings + 0x34);
    float* height = reinterpret_cast<float*>(g_cameraSettings + 0x54);
    if (*distance == g_cameraAppliedDistance && *height == g_cameraAppliedHeight)
    {
        *distance = g_cameraOriginalDistance;
        *height = g_cameraOriginalHeight;
    }
    g_cameraHasApplied = false;
}

static void ApplyCameraValuesLocked(uintptr_t settings)
{
    if (!settings || g_cameraStopping.load() || !MemoryRegionAllows(settings + 0x34, sizeof(float), true) ||
        !MemoryRegionAllows(settings + 0x54, sizeof(float), true)) return;
    if (settings != g_cameraSettings)
    {
        RestoreCameraValuesLocked();
        float distance = *reinterpret_cast<float*>(settings + 0x34);
        float height = *reinterpret_cast<float*>(settings + 0x54);
        if (!CameraValuePlausible(distance) || !CameraValuePlausible(height)) return;
        g_cameraSettings = settings;
        g_cameraOriginalDistance = distance;
        g_cameraOriginalHeight = height;
        g_cameraHasApplied = false;
    }
    float* distance = reinterpret_cast<float*>(settings + 0x34);
    float* height = reinterpret_cast<float*>(settings + 0x54);
    if (g_cameraHasApplied && (*distance != g_cameraAppliedDistance || *height != g_cameraAppliedHeight))
    {
        g_cameraSettings = 0;
        g_cameraHasApplied = false;
        return;
    }
    *distance = g_cameraDistance;
    *height = g_cameraHeight;
    if (*distance != g_cameraDistance || *height != g_cameraHeight)
    {
        *distance = g_cameraOriginalDistance;
        *height = g_cameraOriginalHeight;
        g_cameraSettings = 0;
        g_cameraHasApplied = false;
        return;
    }
    g_cameraAppliedDistance = g_cameraDistance;
    g_cameraAppliedHeight = g_cameraHeight;
    g_cameraHasApplied = true;
}

static void RefreshCameraSettingsFromGlobal()
{
    if (g_cameraSettingsGlobal == LM_ADDRESS_BAD || g_cameraStopping.load() ||
        !MemoryRegionAllows(g_cameraSettingsGlobal, sizeof(uintptr_t), false)) return;
    const uintptr_t settings = *reinterpret_cast<const uintptr_t*>(g_cameraSettingsGlobal);
    AcquireSRWLockExclusive(&g_cameraLock);
    ApplyCameraValuesLocked(settings);
    ReleaseSRWLockExclusive(&g_cameraLock);
}

static uintptr_t CameraInitDetour()
{
    g_cameraCallbacks.fetch_add(1);
    uintptr_t settings = g_cameraOriginalInit ? g_cameraOriginalInit() : 0;
    AcquireSRWLockExclusive(&g_cameraLock);
    ApplyCameraValuesLocked(settings);
    ReleaseSRWLockExclusive(&g_cameraLock);
    g_cameraCallbacks.fetch_sub(1);
    return settings;
}

static bool InstallCameraStepHook(lm_address_t target, float step, lm_address_t* caveOut, lm_byte_t* original, wchar_t* message, size_t messageSize)
{
    if (LM_ReadMemory(target, original, 8) != 8 || original[0] != 0xC5 || original[1] != 0xFA || original[2] != 0x10 || original[3] != 0x05)
    {
        swprintf_s(message, messageSize, L"camera zoom preflight failed");
        return false;
    }
	lm_address_t cave = AllocNear(target, 64);
    if (cave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"camera zoom cave allocation failed");
        return false;
    }
	lm_byte_t code[24]{};
	size_t i = 0;
	code[i++] = 0xC5; code[i++] = 0xFA; code[i++] = 0x10; code[i++] = 0x05; // vmovss xmm0,[rip+disp32]
	int32_t valueDisplacement = 8;
	memcpy(code + i, &valueDisplacement, sizeof(valueDisplacement)); i += sizeof(valueDisplacement);
	code[i++] = 0xE9;
	int64_t backDelta = static_cast<int64_t>(target + 8) - static_cast<int64_t>(cave + i + 4);
    if (backDelta < INT32_MIN || backDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"camera zoom return is out of range");
        return false;
    }
	int32_t back = static_cast<int32_t>(backDelta);
	memcpy(code + i, &back, sizeof(back)); i += sizeof(back);
	while (i < 16) code[i++] = 0x90;
	memcpy(code + i, &step, sizeof(step)); i += sizeof(step);
    if (LM_WriteMemory(cave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"camera zoom cave write failed");
        return false;
    }
    lm_byte_t jump[8]{ 0xE9, 0, 0, 0, 0, 0x90, 0x90, 0x90 };
    int64_t hookDelta = static_cast<int64_t>(cave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"camera zoom hook is out of range");
        return false;
    }
    int32_t hook = static_cast<int32_t>(hookDelta);
    memcpy(jump + 1, &hook, sizeof(hook));
	if (!PatchBytes(target, jump, sizeof(jump)))
    {
        swprintf_s(message, messageSize, L"camera zoom hook write failed");
		return false;
	}
	*caveOut = cave;
	return true;
}

static bool LoadCameraConfig(const wchar_t* path, bool requireEnabled)
{
    bool enabled = GetPrivateProfileIntW(L"camera", L"enabled", 0, path) == 1;
    if (requireEnabled && !enabled) return false;
    float distance = ReadIniFloat(path, L"camera", L"maxDistance", g_cameraDistance);
    float height = ReadIniFloat(path, L"camera", L"targetHeight", g_cameraHeight);
    float step = ReadIniFloat(path, L"camera", L"zoomStep", g_cameraIncreaseStep);
    if (distance < 0.5f || distance > 30.0f || height < 0.0f || height > 5.0f || step < 0.001f || step > 1.0f) return false;
    AcquireSRWLockExclusive(&g_cameraLock);
    g_cameraDistance = distance;
    g_cameraHeight = height;
	g_cameraIncreaseStep = step;
	g_cameraDecreaseStep = -step;
	if (g_cameraIncreaseCave != LM_ADDRESS_BAD) LM_WriteMemory(g_cameraIncreaseCave + 16, reinterpret_cast<lm_bytearray_t>(&g_cameraIncreaseStep), sizeof(g_cameraIncreaseStep));
	if (g_cameraDecreaseCave != LM_ADDRESS_BAD) LM_WriteMemory(g_cameraDecreaseCave + 16, reinterpret_cast<lm_bytearray_t>(&g_cameraDecreaseStep), sizeof(g_cameraDecreaseStep));
    if (g_cameraSettings) ApplyCameraValuesLocked(g_cameraSettings);
    ReleaseSRWLockExclusive(&g_cameraLock);
    return enabled;
}

static bool StopCameraRuntime()
{
	g_cameraStopping.store(true);
	bool manualRestored = true;
	if (g_cameraDecreaseCave != LM_ADDRESS_BAD) manualRestored = PatchBytes(g_cameraDecreaseTarget, g_cameraDecreaseOriginal, sizeof(g_cameraDecreaseOriginal)) && manualRestored;
	if (g_cameraIncreaseCave != LM_ADDRESS_BAD) manualRestored = PatchBytes(g_cameraIncreaseTarget, g_cameraIncreaseOriginal, sizeof(g_cameraIncreaseOriginal)) && manualRestored;
	bool hookRestored = g_cameraInitTarget == LM_ADDRESS_BAD || RestoreLibmemHookAfterDrain(g_cameraInitTarget,
		reinterpret_cast<lm_address_t>(g_cameraOriginalInit), &g_cameraInitHookSize, g_cameraInitOriginal, sizeof(g_cameraInitOriginal), g_cameraCallbacks);
	hookRestored = hookRestored && manualRestored;
	if (!hookRestored) g_patchCoreCanUnload.store(false);
	AcquireSRWLockExclusive(&g_cameraLock);
	RestoreCameraValuesLocked();
	ReleaseSRWLockExclusive(&g_cameraLock);
	WriteRuntimeStatus(L"camera", hookRestored ? L"inactive" : L"restore_failed",
		hookRestored ? L"hooks and owned camera values restored" : L"camera entry restoration could not be proven; module kept loaded");
    ReleaseRuntimeOwnerAfterVerifiedStop(L"camera", hookRestored);
	return hookRestored;
}

static DWORD RunCameraRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"camera"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"camera", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    std::wstring configPath = RuntimePath(L"camera.ini");
    if (configPath.empty() || !LoadCameraConfig(configPath.c_str(), true))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"camera", L"camera configuration is missing or invalid");
        return 1;
    }
    lm_module_t module{};
    if (!LM_FindModule("granblue_fantasy_relink.exe", &module))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"camera", L"game module is unavailable");
        return 1;
    }
    const char* initSignature = "56 48 83 EC ?? 8B 05 ?? ?? ?? ?? 65 48 8B 0C 25 ?? ?? ?? ?? 48 8B 04 C1 48 8B 88 ?? ?? ?? ?? 48 8B 81 ?? ?? ?? ?? 48 8B 70 ?? 48 85 F6 0F 84 ?? ?? ?? ?? 89 F2 83 E2 ?? 0F 85 ?? ?? ?? ?? FF 40 ?? 48 8B 0E 48 89 48 ?? C5 F8 57 C0 C5 F8 29 46 ?? C5 F8 29 46";
    const char* settingsGlobalSignature = "48 89 35 ?? ?? ?? ?? C5 F8 29 06 48 C7 46 10 00 00 00 00 48 8D 05";
    const char* decreaseSignature = "C5 FA 10 05 ?? ?? ?? ?? EB ?? C5 FA 10 05 ?? ?? ?? ?? C5 FA 58 05";
    const char* increaseSignature = "C5 FA 10 05 ?? ?? ?? ?? C5 FA 58 05 ?? ?? ?? ?? C5 FA 5D 05";
    g_cameraInitTarget = FindUniqueSignature(initSignature, module);
    const lm_address_t settingsGlobalInstruction = FindUniqueSignature(settingsGlobalSignature, module);
    g_cameraSettingsGlobal = ResolveRIPGlobal(settingsGlobalInstruction, 3, 7);
    g_cameraDecreaseTarget = FindUniqueSignature(decreaseSignature, module);
    g_cameraIncreaseTarget = FindUniqueSignature(increaseSignature, module);
    if (g_cameraInitTarget == LM_ADDRESS_BAD || g_cameraSettingsGlobal == LM_ADDRESS_BAD ||
        g_cameraDecreaseTarget == LM_ADDRESS_BAD || g_cameraIncreaseTarget == LM_ADDRESS_BAD)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"camera", L"camera signature preflight failed or was ambiguous");
        return 1;
    }
    wchar_t message[256]{};
	if (LM_ReadMemory(g_cameraInitTarget, g_cameraInitOriginal, sizeof(g_cameraInitOriginal)) != sizeof(g_cameraInitOriginal))
	{
		WriteRuntimeInactiveAndReleaseOwner(L"camera", L"camera entry preflight read failed");
		return 1;
	}
	g_cameraInitHookSize = LM_HookCode(g_cameraInitTarget, reinterpret_cast<lm_address_t>(&CameraInitDetour), reinterpret_cast<lm_address_t*>(&g_cameraOriginalInit));
	if (!g_cameraInitHookSize || !InstallCameraStepHook(g_cameraDecreaseTarget, g_cameraDecreaseStep, &g_cameraDecreaseCave, g_cameraDecreaseOriginal, message, _countof(message)) ||
		!InstallCameraStepHook(g_cameraIncreaseTarget, g_cameraIncreaseStep, &g_cameraIncreaseCave, g_cameraIncreaseOriginal, message, _countof(message)))
    {
        const bool restored = StopCameraRuntime();
        WriteStartupFailureAfterStop(L"camera", restored, message[0] ? message : L"camera hook installation failed");
        return 1;
    }
    RefreshCameraSettingsFromGlobal();
    WriteRuntimeStatus(L"camera", L"active", L"native camera runtime is active");
    FILETIME previous{};
    while (owner.Alive())
    {
        WIN32_FILE_ATTRIBUTE_DATA data{};
        if (!GetFileAttributesExW(configPath.c_str(), GetFileExInfoStandard, &data) || GetPrivateProfileIntW(L"camera", L"enabled", 0, configPath.c_str()) != 1) break;
        if (CompareFileTime(&data.ftLastWriteTime, &previous) != 0)
        {
            previous = data.ftLastWriteTime;
            LoadCameraConfig(configPath.c_str(), false);
        }
        RefreshCameraSettingsFromGlobal();
        Sleep(250);
    }
    StopCameraRuntime();
    return 0;
}

#pragma pack(push, 1)
struct VirtualSigilConfigHeader
{
    char magic[8];
    uint32_t schema;
    uint32_t enabled;
    uint32_t slotCount;
    uint32_t entryCount;
};

struct VirtualSigilConfigEntry
{
    uint32_t characterHash;
    uint32_t slotIndex;
    uint32_t slotId;
    uint32_t gemId;
    uint32_t trait1;
    int32_t trait1Level;
    uint32_t trait2;
    int32_t trait2Level;
    int32_t sigilLevel;
};

struct RuntimeGemData
{
    uint32_t trait1;
    int32_t trait1Level;
    uint32_t trait2;
    int32_t trait2Level;
    uint32_t gemId;
    uint32_t wornBy;
    int32_t sigilLevel;
    uint32_t slotId;
    uint32_t flags;
};
#pragma pack(pop)

using GetGemDataFunction = uint8_t(*)(uintptr_t, int, uintptr_t);
struct VirtualSigilRuntimeLayout
{
    const wchar_t* version;
    lm_address_t traitApplyLoopRva;
    lm_address_t traitCategoryLoopRva;
    lm_address_t traitFetchRva;
    lm_address_t traitFetchCallRva;
    lm_address_t getGemDataRva;
    lm_address_t systemDataGlobalRva;
    lm_address_t statusManagerGlobalRva;
};
static constexpr VirtualSigilRuntimeLayout kVirtualSigilRuntimeLayouts[] = {
    { L"2.0.2", 0x00A25484, 0x00A26096, 0x00A260AE, 0x00A260F0, 0x00A2C610, 0x07C20940, 0x07C24980 },
    { L"2.0.3", 0x00A1EBE4, 0x00A1F7F6, 0x00A1F80E, 0x00A1F850, 0x00A25D70, 0x07C1D900, 0x07C21940 },
    { L"2.0.4", 0x00A1FB84, 0x00A20796, 0x00A207AE, 0x00A207F0, 0x00A26D10, 0x07C1EB80, 0x07C22BC0 },
};
static constexpr int kNativeSigilSlots = 13;
static constexpr int kMainGemCapacity = 5100;
static constexpr int kMainGemArrayOffset = 0x25D0;
static constexpr uint32_t kUnwornCharacterHash = 0x887AE0B0;
static constexpr uint32_t kLocalPlayerStatusKey = 0xDBD9A18D;
static const lm_byte_t kTraitFetchOriginal[] = { 0x84, 0xDB, 0x74, 0x3E, 0x49, 0x8B, 0x87, 0x80, 0x5E, 0x00, 0x00 };
static const lm_byte_t kGetterOriginal[] = { 0x55, 0x41, 0x57, 0x41, 0x56, 0x56, 0x57, 0x53, 0x48, 0x83, 0xEC, 0x28 };
static SRWLOCK g_virtualSigilLock = SRWLOCK_INIT;
static std::unordered_map<uint32_t, std::array<VirtualSigilConfigEntry, 8>> g_virtualSigils;
static uint32_t g_virtualSlotCount = 4;
static const VirtualSigilRuntimeLayout* g_virtualLayout = nullptr;
static lm_address_t g_virtualModuleBase = 0;
static GetGemDataFunction g_originalGetGemData = nullptr;
static lm_size_t g_getGemHookSize = 0;
static lm_byte_t g_getGemOriginal[32]{};
static lm_address_t g_traitFetchCave = LM_ADDRESS_BAD;
static bool g_traitFetchInstalled = false;
static std::atomic<LONG> g_virtualCallbacks{ 0 };
static std::atomic<bool> g_virtualStopping{ false };

static bool ReadSharedFile(const std::wstring& path, std::vector<uint8_t>& output, size_t maximumSize)
{
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ,
        FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, nullptr, OPEN_EXISTING,
        FILE_ATTRIBUTE_NORMAL | FILE_FLAG_SEQUENTIAL_SCAN, nullptr);
    if (file == INVALID_HANDLE_VALUE) return false;
    LARGE_INTEGER length{};
    bool valid = GetFileSizeEx(file, &length) && length.QuadPart >= 0 &&
        static_cast<uint64_t>(length.QuadPart) <= maximumSize && length.QuadPart <= MAXDWORD;
    if (!valid)
    {
        CloseHandle(file);
        return false;
    }
    output.resize(static_cast<size_t>(length.QuadPart));
    DWORD read = 0;
    valid = output.empty() || (ReadFile(file, output.data(), static_cast<DWORD>(output.size()), &read, nullptr) && read == output.size());
    CloseHandle(file);
    return valid;
}

static bool ReadVirtualSigilConfig(const std::wstring& path, bool requireEnabled, uint32_t requiredSlotCount = 0)
{
    std::vector<uint8_t> data;
    if (!ReadSharedFile(path, data, 1024 * 1024) || data.size() < sizeof(VirtualSigilConfigHeader)) return false;
    VirtualSigilConfigHeader header{};
    memcpy(&header, data.data(), sizeof(header));
    if (memcmp(header.magic, "GBFRVS02", 8) != 0 || header.schema != 2 || header.slotCount < 1 || header.slotCount > 8 || header.entryCount > 232 ||
        data.size() != sizeof(header) + static_cast<size_t>(header.entryCount) * sizeof(VirtualSigilConfigEntry)) return false;
    if (requireEnabled && header.enabled != 1) return false;
    if (requiredSlotCount && header.slotCount != requiredSlotCount) return false;
    std::unordered_map<uint32_t, std::array<VirtualSigilConfigEntry, 8>> next;
    std::unordered_set<uint32_t> physicalOwners;
    for (uint32_t index = 0; index < header.entryCount; ++index)
    {
        VirtualSigilConfigEntry entry{};
        memcpy(&entry, data.data() + sizeof(header) + static_cast<size_t>(index) * sizeof(entry), sizeof(entry));
        if (!entry.characterHash || entry.slotIndex >= header.slotCount || !entry.slotId || !entry.gemId || !entry.trait1 || entry.trait1Level <= 0 || entry.sigilLevel <= 0 ||
            !physicalOwners.insert(entry.slotId).second) return false;
        next[entry.characterHash][entry.slotIndex] = entry;
    }
    AcquireSRWLockExclusive(&g_virtualSigilLock);
    g_virtualSlotCount = header.slotCount;
    g_virtualSigils.swap(next);
    ReleaseSRWLockExclusive(&g_virtualSigilLock);
    return header.enabled == 1;
}

static bool ResolveOwnedStatusUnsafe(uintptr_t status, uint32_t characterHash, int contextMode)
{
    if (!g_virtualLayout) return false;
    uintptr_t global = g_virtualModuleBase + g_virtualLayout->statusManagerGlobalRva;
    if (!MemoryRegionAllows(global, sizeof(uintptr_t), false)) return false;
    uintptr_t manager = *reinterpret_cast<uintptr_t*>(global);
    if (!manager || !MemoryRegionAllows(manager + 0xA58, 4, false) || !MemoryRegionAllows(manager + 0xA40, sizeof(uintptr_t), false) ||
        !MemoryRegionAllows(manager + 0xA30, sizeof(uintptr_t), false)) return false;
    uint32_t mask = *reinterpret_cast<uint32_t*>(manager + 0xA58);
    if (mask > 0xFFFF || ((mask + 1) & mask) != 0) return false;
    uintptr_t table = *reinterpret_cast<uintptr_t*>(manager + 0xA40);
    uintptr_t sentinel = *reinterpret_cast<uintptr_t*>(manager + 0xA30);
    uint32_t key = contextMode == 1 ? kLocalPlayerStatusKey : characterHash;
    uintptr_t bucket = table + static_cast<uintptr_t>((key & mask) * 0x10u);
    if (!table || !sentinel || !MemoryRegionAllows(bucket, 0x10, false)) return false;
    uintptr_t last = *reinterpret_cast<uintptr_t*>(bucket);
    uintptr_t node = *reinterpret_cast<uintptr_t*>(bucket + 8);
    for (int step = 0; step < 256 && node && node != sentinel; ++step)
    {
        if (!MemoryRegionAllows(node, 0x38, false)) return false;
        if (*reinterpret_cast<uint32_t*>(node + 0x10) == key) return *reinterpret_cast<uintptr_t*>(node + 0x30) == status;
        if (node == last) break;
        node = *reinterpret_cast<uintptr_t*>(node + 8);
    }
    return false;
}

static bool ResolveOwnedStatus(uintptr_t status, uint32_t characterHash, int contextMode)
{
    __try
    {
        return ResolveOwnedStatusUnsafe(status, characterHash, contextMode);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

static bool FindInventoryGemUnsafe(uint32_t slotId, RuntimeGemData& output)
{
    if (!g_virtualLayout) return false;
    uintptr_t global = g_virtualModuleBase + g_virtualLayout->systemDataGlobalRva;
    if (!MemoryRegionAllows(global, sizeof(uintptr_t), false)) return false;
    uintptr_t systemData = *reinterpret_cast<uintptr_t*>(global);
    uintptr_t start = systemData + kMainGemArrayOffset;
    size_t bytes = kMainGemCapacity * sizeof(RuntimeGemData);
    if (!systemData || !MemoryRegionAllows(start, bytes, false)) return false;
    const RuntimeGemData* gems = reinterpret_cast<const RuntimeGemData*>(start);
    for (int index = 0; index < kMainGemCapacity; ++index)
    {
        if (gems[index].slotId == slotId)
        {
            output = gems[index];
            return true;
        }
    }
    return false;
}

static bool FindInventoryGem(uint32_t slotId, RuntimeGemData& output)
{
    __try
    {
        return FindInventoryGemUnsafe(slotId, output);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

static bool ReadVirtualSigilStatusFields(uintptr_t status, uint32_t& characterHash, int& contextMode)
{
    __try
    {
        if (!status || !MemoryRegionAllows(status + 0x5EA8, 8, false)) return false;
        characterHash = *reinterpret_cast<uint32_t*>(status + 0x5EA8);
        contextMode = *reinterpret_cast<int*>(status + 0x5EAC);
        return true;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

static bool WriteVirtualSigilOutput(uintptr_t output, const RuntimeGemData& gem)
{
    __try
    {
        if (!output || !MemoryRegionAllows(output, sizeof(RuntimeGemData), true)) return false;
        memcpy(reinterpret_cast<void*>(output), &gem, sizeof(gem));
        return true;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

static uint8_t GetGemDataDetour(uintptr_t status, int slotIndex, uintptr_t output)
{
    g_virtualCallbacks.fetch_add(1);
    auto finish = [](uint8_t result) { g_virtualCallbacks.fetch_sub(1); return result; };
    uint32_t slotCount = g_virtualSlotCount;
    if (slotIndex < kNativeSigilSlots || slotIndex >= kNativeSigilSlots + static_cast<int>(slotCount) || g_virtualStopping.load())
        return finish(g_originalGetGemData ? g_originalGetGemData(status, slotIndex, output) : 0);
    uint32_t characterHash = 0;
    int contextMode = -1;
    if (!ReadVirtualSigilStatusFields(status, characterHash, contextMode) || !output) return finish(0);
    if (!characterHash || contextMode < 0 || contextMode > 2 || !ResolveOwnedStatus(status, characterHash, contextMode)) return finish(0);
    VirtualSigilConfigEntry selection{};
    AcquireSRWLockShared(&g_virtualSigilLock);
    auto character = g_virtualSigils.find(characterHash);
    if (character != g_virtualSigils.end()) selection = character->second[slotIndex - kNativeSigilSlots];
    ReleaseSRWLockShared(&g_virtualSigilLock);
    if (!selection.slotId) return finish(0);
    RuntimeGemData gem{};
    if (!FindInventoryGem(selection.slotId, gem) || gem.slotId != selection.slotId || gem.gemId != selection.gemId || gem.trait1 != selection.trait1 ||
        gem.trait1Level != selection.trait1Level || gem.trait2 != selection.trait2 || gem.trait2Level != selection.trait2Level || gem.sigilLevel != selection.sigilLevel ||
        gem.wornBy != kUnwornCharacterHash || (gem.flags & 0x10) != 0) return finish(0);
    return finish(WriteVirtualSigilOutput(output, gem) ? 1 : 0);
}

static bool InstallVirtualTraitFetchHook(lm_address_t target, lm_address_t callPath, uint8_t expandedSlots, wchar_t* message, size_t messageSize)
{
    lm_byte_t current[sizeof(kTraitFetchOriginal)]{};
    if (LM_ReadMemory(target, current, sizeof(current)) != sizeof(current) || !BytesEqual(current, kTraitFetchOriginal, sizeof(current)))
    {
        swprintf_s(message, messageSize, L"virtual sigil trait-fetch preflight failed");
        return false;
    }
    g_traitFetchCave = AllocNear(target, 96);
    if (g_traitFetchCave == LM_ADDRESS_BAD)
    {
        swprintf_s(message, messageSize, L"virtual sigil trait-fetch cave allocation failed");
        return false;
    }
    lm_byte_t code[96]{};
    size_t i = 0;
    code[i++] = 0x41; code[i++] = 0x83; code[i++] = 0xFD; code[i++] = kNativeSigilSlots;
    code[i++] = 0x72; size_t jbOriginal = i++;
    code[i++] = 0x41; code[i++] = 0x83; code[i++] = 0xFD; code[i++] = expandedSlots;
    code[i++] = 0x73; size_t jaeOriginal = i++;
    code[i++] = 0x48; code[i++] = 0xB8;
    memcpy(code + i, &callPath, sizeof(callPath)); i += sizeof(callPath);
    code[i++] = 0xFF; code[i++] = 0xE0;
    // Relocate the original control flow, not its bytes. kTraitFetchOriginal
    // contains `je +0x3e`; copying that short relative branch into this cave
    // makes it jump into the zero-filled tail and crash when bl == 0.
    size_t nativeOffset = i;
    code[i++] = 0x84; code[i++] = 0xDB; // test bl, bl
    code[i++] = 0x75; size_t jneNativeNonzero = i++; // jne native-nonzero
    code[i++] = 0x48; code[i++] = 0xB8;
    memcpy(code + i, &callPath, sizeof(callPath)); i += sizeof(callPath);
    code[i++] = 0xFF; code[i++] = 0xE0; // jmp callPath for native bl == 0
    size_t nativeNonzeroOffset = i;
    code[i++] = 0x49; code[i++] = 0x8B; code[i++] = 0x87;
    code[i++] = 0x80; code[i++] = 0x5E; code[i++] = 0x00; code[i++] = 0x00; // mov rax,[r15+0x5e80]
    const lm_address_t returnPath = target + sizeof(kTraitFetchOriginal);
    code[i++] = 0x49; code[i++] = 0xBB;
    memcpy(code + i, &returnPath, sizeof(returnPath)); i += sizeof(returnPath);
    code[i++] = 0x41; code[i++] = 0xFF; code[i++] = 0xE3; // jmp r11
    if (nativeOffset - (jbOriginal + 1) > 127 || nativeOffset - (jaeOriginal + 1) > 127 ||
        nativeNonzeroOffset - (jneNativeNonzero + 1) > 127)
    {
        swprintf_s(message, messageSize, L"virtual sigil trait-fetch jump is out of range");
        return false;
    }
    code[jbOriginal] = static_cast<lm_byte_t>(nativeOffset - (jbOriginal + 1));
    code[jaeOriginal] = static_cast<lm_byte_t>(nativeOffset - (jaeOriginal + 1));
    code[jneNativeNonzero] = static_cast<lm_byte_t>(nativeNonzeroOffset - (jneNativeNonzero + 1));
    if (LM_WriteMemory(g_traitFetchCave, code, i) != i)
    {
        swprintf_s(message, messageSize, L"virtual sigil trait-fetch cave write failed");
        return false;
    }
    lm_byte_t jump[sizeof(kTraitFetchOriginal)]{ 0xE9 };
    memset(jump + 5, 0x90, sizeof(jump) - 5);
    int64_t hookDelta = static_cast<int64_t>(g_traitFetchCave) - static_cast<int64_t>(target + 5);
    if (hookDelta < INT32_MIN || hookDelta > INT32_MAX)
    {
        swprintf_s(message, messageSize, L"virtual sigil trait-fetch hook is out of range");
        return false;
    }
    int32_t hook = static_cast<int32_t>(hookDelta);
    memcpy(jump + 1, &hook, sizeof(hook));
	g_traitFetchInstalled = PatchBytes(target, jump, sizeof(jump));
	return g_traitFetchInstalled;
}

static bool StopVirtualSigilRuntime()
{
	g_virtualStopping.store(true);
	if (!g_virtualLayout || !g_virtualModuleBase)
	{
		WriteRuntimeStatus(L"virtual-sigils", L"inactive", L"no virtual sigil layout was installed");
		ReleaseRuntimeOwnerAfterVerifiedStop(L"virtual-sigils", true);
		return true;
	}
	lm_address_t getter = g_virtualModuleBase + g_virtualLayout->getGemDataRva;
	bool nativeRestored = true;
	if (g_traitFetchInstalled) nativeRestored = PatchBytes(g_virtualModuleBase + g_virtualLayout->traitFetchRva, kTraitFetchOriginal, sizeof(kTraitFetchOriginal));
	bool hookRestored = RestoreLibmemHookAfterDrain(getter, reinterpret_cast<lm_address_t>(g_originalGetGemData),
		&g_getGemHookSize, g_getGemOriginal, sizeof(g_getGemOriginal), g_virtualCallbacks);
	uint8_t current = 0;
    lm_address_t category = g_virtualModuleBase + g_virtualLayout->traitCategoryLoopRva;
    lm_address_t apply = g_virtualModuleBase + g_virtualLayout->traitApplyLoopRva;
	lm_size_t categoryRead = LM_ReadMemory(category, &current, 1);
	if (categoryRead == 1 && current == kNativeSigilSlots + g_virtualSlotCount)
	{
		uint8_t native = kNativeSigilSlots;
		nativeRestored = PatchBytes(category, &native, 1) && nativeRestored;
	}
	else if (categoryRead != 1 || current != kNativeSigilSlots) nativeRestored = false;
	lm_size_t applyRead = LM_ReadMemory(apply, &current, 1);
	if (applyRead == 1 && current == kNativeSigilSlots + g_virtualSlotCount)
	{
		uint8_t native = kNativeSigilSlots;
		nativeRestored = PatchBytes(apply, &native, 1) && nativeRestored;
	}
	else if (applyRead != 1 || current != kNativeSigilSlots) nativeRestored = false;
	hookRestored = hookRestored && nativeRestored;
	if (!hookRestored) g_patchCoreCanUnload.store(false);
	WriteRuntimeStatus(L"virtual-sigils", hookRestored ? L"inactive" : L"restore_failed",
		hookRestored ? L"hooks and native loop limits restored" : L"virtual sigil restoration could not be proven; module kept loaded");
    ReleaseRuntimeOwnerAfterVerifiedStop(L"virtual-sigils", hookRestored);
	return hookRestored;
}

static DWORD RunVirtualSigilRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"virtual-sigils"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"virtual-sigils", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    constexpr bool kStableReleaseVirtualSigilsEnabled = true;
    if (!kStableReleaseVirtualSigilsEnabled)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"virtual-sigils", L"virtual sigils are disabled in the stable build pending field acceptance");
        return 1;
    }
    std::wstring configPath = RuntimePath(L"virtual-sigils.bin");
    if (configPath.empty() || !ReadVirtualSigilConfig(configPath, true))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"virtual-sigils", L"virtual sigil configuration is missing or invalid");
        return 1;
    }
    lm_module_t module{};
    if (!LM_FindModule("granblue_fantasy_relink.exe", &module))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"virtual-sigils", L"game module is unavailable");
        return 1;
    }
    g_virtualModuleBase = module.base;
	const VirtualSigilRuntimeLayout* layout = nullptr;
	lm_byte_t getter[sizeof(kGetterOriginal)]{};
	lm_byte_t applyPreflight[16]{};
	lm_byte_t categoryPreflight[13]{};
    const lm_byte_t expectedApply[] = { 0xFF, 0xC7, 0x83, 0xFF, 0x0D, 0x0F, 0x84, 0xB7, 0x00, 0x00, 0x00, 0xC5, 0xF8, 0x11, 0x75, 0xF0 };
    const lm_byte_t expectedCategory[] = { 0x49, 0xFF, 0xC5, 0x49, 0x83, 0xFD, 0x0D, 0x0F, 0x84, 0xE4, 0x00, 0x00, 0x00 };
	for (const auto& candidate : kVirtualSigilRuntimeLayouts)
	{
		if (LM_ReadMemory(module.base + candidate.getGemDataRva, getter, sizeof(getter)) != sizeof(getter) || !BytesEqual(getter, kGetterOriginal, sizeof(getter))) continue;
		if (LM_ReadMemory(module.base + candidate.traitApplyLoopRva - 4, applyPreflight, sizeof(applyPreflight)) != sizeof(applyPreflight) || !BytesEqual(applyPreflight, expectedApply, sizeof(expectedApply))) continue;
		if (LM_ReadMemory(module.base + candidate.traitCategoryLoopRva - 6, categoryPreflight, sizeof(categoryPreflight)) != sizeof(categoryPreflight) || !BytesEqual(categoryPreflight, expectedCategory, sizeof(expectedCategory))) continue;
		if (layout) {
			layout = nullptr;
			break;
		}
		layout = &candidate;
	}
	if (!layout)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"virtual-sigils", L"virtual sigil executable preflight failed");
        return 1;
    }
	g_virtualLayout = layout;
    wchar_t message[256]{};
	lm_address_t getterTarget = module.base + g_virtualLayout->getGemDataRva;
	if (LM_ReadMemory(getterTarget, g_getGemOriginal, sizeof(g_getGemOriginal)) != sizeof(g_getGemOriginal))
	{
		WriteRuntimeInactiveAndReleaseOwner(L"virtual-sigils", L"virtual sigil getter preflight read failed");
		return 1;
	}
	g_virtualStopping.store(false);
	g_getGemHookSize = LM_HookCode(getterTarget, reinterpret_cast<lm_address_t>(&GetGemDataDetour), reinterpret_cast<lm_address_t*>(&g_originalGetGemData));
    uint8_t expanded = static_cast<uint8_t>(kNativeSigilSlots + g_virtualSlotCount);
	if (!g_getGemHookSize || !InstallVirtualTraitFetchHook(module.base + g_virtualLayout->traitFetchRva, module.base + g_virtualLayout->traitFetchCallRva, expanded, message, _countof(message)))
    {
        const bool restored = StopVirtualSigilRuntime();
        WriteStartupFailureAfterStop(L"virtual-sigils", restored, message[0] ? message : L"virtual sigil hook installation failed");
        return 1;
    }
    uint8_t native = kNativeSigilSlots;
    uint8_t current = 0;
    if (LM_ReadMemory(module.base + g_virtualLayout->traitApplyLoopRva, &current, 1) != 1 || current != native || !PatchBytes(module.base + g_virtualLayout->traitApplyLoopRva, &expanded, 1) ||
        LM_ReadMemory(module.base + g_virtualLayout->traitCategoryLoopRva, &current, 1) != 1 || current != native || !PatchBytes(module.base + g_virtualLayout->traitCategoryLoopRva, &expanded, 1))
    {
        const bool restored = StopVirtualSigilRuntime();
        WriteStartupFailureAfterStop(L"virtual-sigils", restored, L"virtual sigil loop-limit patch failed");
        return 1;
    }
    WriteRuntimeStatus(L"virtual-sigils", L"active", L"native virtual sigil runtime is active");
    FILETIME previous{};
    while (owner.Alive())
    {
        WIN32_FILE_ATTRIBUTE_DATA data{};
        if (!GetFileAttributesExW(configPath.c_str(), GetFileExInfoStandard, &data)) break;
        std::vector<uint8_t> headerData;
        VirtualSigilConfigHeader header{};
        if (!ReadSharedFile(configPath, headerData, 1024 * 1024) || headerData.size() < sizeof(header)) break;
        memcpy(&header, headerData.data(), sizeof(header));
        if (header.enabled != 1) break;
        if (CompareFileTime(&data.ftLastWriteTime, &previous) != 0)
        {
            previous = data.ftLastWriteTime;
            if (!ReadVirtualSigilConfig(configPath, false, g_virtualSlotCount))
            {
                WriteRuntimeStatus(L"virtual-sigils", L"active", L"hot configuration was rejected; keeping the active mapping");
            }
        }
        Sleep(250);
    }
    StopVirtualSigilRuntime();
    return 0;
}

#pragma pack(push, 1)
struct WeaponRuntimeConfigHeader
{
    char magic[8];
    uint32_t schema;
    uint32_t enabled;
    int32_t weaponSlot;
    uint32_t weaponId;
    uint32_t entryCount;
};

struct WeaponRuntimeSkillEntry
{
    uint32_t hash;
    uint32_t level;
};
#pragma pack(pop)

static_assert(sizeof(WeaponRuntimeConfigHeader) == 28);
static_assert(sizeof(WeaponRuntimeSkillEntry) == 8);

using WeaponTraitAggregationFunction = void(*)(uintptr_t, uintptr_t, uintptr_t);
using ApplyWeaponTraitFunction = void(*)(uint32_t, uintptr_t, uint32_t, uint32_t, uint32_t);
static constexpr uint32_t kWeaponRuntimeMaxEntries = 2048;
static constexpr lm_address_t kWeaponRuntimeStatusManagerRvas[] = { 0x07C21940, 0x07C22BC0 };
static SRWLOCK g_weaponRuntimeLock = SRWLOCK_INIT;
static std::array<WeaponRuntimeSkillEntry, kWeaponRuntimeMaxEntries> g_weaponRuntimeSkills{};
static size_t g_weaponRuntimeSkillCount = 0;
static int32_t g_weaponRuntimeSlot = -1;
static uint32_t g_weaponRuntimeId = 0;
static lm_address_t g_weaponRuntimeModuleBase = 0;
static lm_address_t g_weaponAggregationTarget = LM_ADDRESS_BAD;
static WeaponTraitAggregationFunction g_originalWeaponAggregation = nullptr;
static ApplyWeaponTraitFunction g_applyWeaponTrait = nullptr;
static lm_size_t g_weaponAggregationHookSize = 0;
static lm_byte_t g_weaponAggregationOriginal[32]{};
static lm_byte_t g_weaponAggregationInstalled[32]{};
static lm_size_t g_weaponAggregationInstalledSize = 0;
static std::atomic<LONG> g_weaponRuntimeCallbacks{ 0 };
static std::atomic<LONG> g_weaponRuntimeCleanRebuilds{ 0 };
static std::atomic<bool> g_weaponRuntimeStopping{ false };

class WeaponRuntimeCallbackGuard
{
public:
    WeaponRuntimeCallbackGuard() { g_weaponRuntimeCallbacks.fetch_add(1); }
    ~WeaponRuntimeCallbackGuard() { g_weaponRuntimeCallbacks.fetch_sub(1); }
    WeaponRuntimeCallbackGuard(const WeaponRuntimeCallbackGuard&) = delete;
    WeaponRuntimeCallbackGuard& operator=(const WeaponRuntimeCallbackGuard&) = delete;
};

static bool ReadWeaponRuntimeConfig(const std::wstring& path, bool requireEnabled)
{
    std::vector<uint8_t> data;
    const size_t maximum = sizeof(WeaponRuntimeConfigHeader) +
        static_cast<size_t>(kWeaponRuntimeMaxEntries) * sizeof(WeaponRuntimeSkillEntry);
    if (!ReadSharedFile(path, data, maximum) || data.size() < sizeof(WeaponRuntimeConfigHeader)) return false;
    WeaponRuntimeConfigHeader header{};
    memcpy(&header, data.data(), sizeof(header));
    if (memcmp(header.magic, "GBFRWK01", 8) != 0 || header.schema != 1 || header.weaponSlot < 0 ||
        !header.weaponId || header.weaponId == kUnwornCharacterHash || !header.entryCount ||
        header.entryCount > kWeaponRuntimeMaxEntries ||
        data.size() != sizeof(header) + static_cast<size_t>(header.entryCount) * sizeof(WeaponRuntimeSkillEntry)) return false;
    if (requireEnabled && header.enabled != 1) return false;
    std::vector<WeaponRuntimeSkillEntry> next(header.entryCount);
    memcpy(next.data(), data.data() + sizeof(header), next.size() * sizeof(WeaponRuntimeSkillEntry));
    for (const auto& skill : next)
    {
        if (!skill.hash || skill.hash == kUnwornCharacterHash || !skill.level || skill.level > 0x7FFFFFFFu) return false;
    }
    AcquireSRWLockExclusive(&g_weaponRuntimeLock);
    g_weaponRuntimeSlot = header.weaponSlot;
    g_weaponRuntimeId = header.weaponId;
    g_weaponRuntimeSkillCount = next.size();
    std::copy(next.begin(), next.end(), g_weaponRuntimeSkills.begin());
    ReleaseSRWLockExclusive(&g_weaponRuntimeLock);
    return header.enabled == 1;
}

static bool ResolveLocalWeaponStatusUnsafe(uintptr_t status, uintptr_t weapon)
{
    if (!g_weaponRuntimeModuleBase || !status || !weapon ||
        !MemoryRegionAllows(status + 0x5B60, sizeof(uint32_t), false) ||
        !MemoryRegionAllows(status + 0x5EAC, sizeof(int32_t), false) ||
        !MemoryRegionAllows(weapon, 8, false) || *reinterpret_cast<int32_t*>(status + 0x5EAC) != 1) return false;
    for (const lm_address_t managerRva : kWeaponRuntimeStatusManagerRvas)
    {
        uintptr_t global = g_weaponRuntimeModuleBase + managerRva;
        if (!MemoryRegionAllows(global, sizeof(uintptr_t), false)) continue;
        uintptr_t manager = *reinterpret_cast<uintptr_t*>(global);
        if (!manager || !MemoryRegionAllows(manager + 0xA58, 4, false) ||
            !MemoryRegionAllows(manager + 0xA40, sizeof(uintptr_t), false) ||
            !MemoryRegionAllows(manager + 0xA30, sizeof(uintptr_t), false)) continue;
        uint32_t mask = *reinterpret_cast<uint32_t*>(manager + 0xA58);
        if (mask > 0xFFFF || ((mask + 1) & mask) != 0) continue;
        uintptr_t table = *reinterpret_cast<uintptr_t*>(manager + 0xA40);
        uintptr_t sentinel = *reinterpret_cast<uintptr_t*>(manager + 0xA30);
        if (!table || !sentinel) continue;
        uintptr_t bucket = table + static_cast<uintptr_t>((kLocalPlayerStatusKey & mask) * 0x10u);
        if (!MemoryRegionAllows(bucket, 0x10, false)) continue;
        uintptr_t last = *reinterpret_cast<uintptr_t*>(bucket);
        uintptr_t node = *reinterpret_cast<uintptr_t*>(bucket + 8);
        for (int step = 0; step < 256 && node && node != sentinel; ++step)
        {
            if (!MemoryRegionAllows(node, 0x38, false)) break;
            if (*reinterpret_cast<uint32_t*>(node + 0x10) == kLocalPlayerStatusKey)
            {
                if (*reinterpret_cast<uintptr_t*>(node + 0x30) == status) return true;
                break;
            }
            if (node == last) break;
            node = *reinterpret_cast<uintptr_t*>(node + 8);
        }
    }
    return false;
}

static bool ResolveLocalWeaponStatus(uintptr_t status, uintptr_t weapon)
{
    __try
    {
        return ResolveLocalWeaponStatusUnsafe(status, weapon);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

static void ApplyExtraWeaponSkillsUnsafe(uintptr_t status, uintptr_t accumulator, uintptr_t weapon,
    int32_t targetSlot, uint32_t targetId, const WeaponRuntimeSkillEntry* skills, size_t count)
{
    __try
    {
        if (!skills || *reinterpret_cast<int32_t*>(weapon) != targetSlot ||
            *reinterpret_cast<uint32_t*>(weapon + 4) != targetId ||
            !MemoryRegionAllows(status + 0x5B60, sizeof(uint32_t), false)) return;
        const uint32_t statusValue = *reinterpret_cast<uint32_t*>(status + 0x5B60);
        for (size_t index = 0; index < count; ++index)
        {
            if (g_weaponRuntimeStopping.load()) break;
            g_applyWeaponTrait(statusValue, accumulator, skills[index].hash, skills[index].level, 0);
        }
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
}

static void WeaponTraitAggregationDetour(uintptr_t status, uintptr_t accumulator, uintptr_t weapon)
{
    WeaponRuntimeCallbackGuard callbackGuard;
    if (g_originalWeaponAggregation) g_originalWeaponAggregation(status, accumulator, weapon);
    if (g_weaponRuntimeStopping.load())
    {
        // This callback ran the complete native aggregator after additions were
        // disabled. It is the only evidence that the cached status was rebuilt
        // without the runtime-only weapon skills.
        g_weaponRuntimeCleanRebuilds.fetch_add(1);
        return;
    }
    if (accumulator && ResolveLocalWeaponStatus(status, weapon))
    {
        std::array<WeaponRuntimeSkillEntry, kWeaponRuntimeMaxEntries> skills{};
        size_t skillCount = 0;
        int32_t targetSlot = -1;
        uint32_t targetId = 0;
        AcquireSRWLockShared(&g_weaponRuntimeLock);
        targetSlot = g_weaponRuntimeSlot;
        targetId = g_weaponRuntimeId;
        skillCount = g_weaponRuntimeSkillCount;
        std::copy_n(g_weaponRuntimeSkills.begin(), skillCount, skills.begin());
        ReleaseSRWLockShared(&g_weaponRuntimeLock);
        ApplyExtraWeaponSkillsUnsafe(status, accumulator, weapon, targetSlot, targetId, skills.data(), skillCount);
    }
}

static bool StopWeaponSkillsRuntime()
{
    g_weaponRuntimeCleanRebuilds.store(0);
    g_weaponRuntimeStopping.store(true);
    const DWORD rebuildDeadline = GetTickCount() + 500;
    while (g_weaponRuntimeCleanRebuilds.load() == 0 &&
        static_cast<LONG>(GetTickCount() - rebuildDeadline) < 0) Sleep(10);
    const bool cleanRebuildObserved = g_weaponRuntimeCleanRebuilds.load() != 0;
    bool ownedEntry = g_weaponAggregationTarget == LM_ADDRESS_BAD || g_weaponAggregationHookSize == 0;
    if (!ownedEntry && g_weaponAggregationInstalledSize == g_weaponAggregationHookSize &&
        g_weaponAggregationInstalledSize <= sizeof(g_weaponAggregationInstalled))
    {
        lm_byte_t current[sizeof(g_weaponAggregationInstalled)]{};
        ownedEntry = LM_ReadMemory(g_weaponAggregationTarget, current, g_weaponAggregationInstalledSize) == g_weaponAggregationInstalledSize &&
            memcmp(current, g_weaponAggregationInstalled, g_weaponAggregationInstalledSize) == 0;
    }
    const bool restored = ownedEntry && (g_weaponAggregationTarget == LM_ADDRESS_BAD ||
        RestoreLibmemHookAfterDrain(g_weaponAggregationTarget,
            reinterpret_cast<lm_address_t>(g_originalWeaponAggregation), &g_weaponAggregationHookSize,
            g_weaponAggregationOriginal, sizeof(g_weaponAggregationOriginal), g_weaponRuntimeCallbacks));
    if (!restored) g_patchCoreCanUnload.store(false);
    const wchar_t* state = !restored ? L"restore_failed" :
        cleanRebuildObserved ? L"inactive" : L"inactive_pending_refresh";
    const wchar_t* detail = !restored ? L"weapon skill hook ownership/restoration could not be proven; module kept loaded" :
        cleanRebuildObserved ? L"weapon aggregation hook restored after a clean native status rebuild" :
        L"weapon aggregation hook restored; cached status still needs the next native rebuild";
    WriteRuntimeStatus(L"weapon-skills", state, detail);
    ReleaseRuntimeOwnerAfterVerifiedStop(L"weapon-skills", restored);
    if (restored) g_weaponAggregationInstalledSize = 0;
    return restored;
}

static DWORD RunWeaponSkillsRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"weapon-skills"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    const std::wstring configPath = RuntimePath(L"weapon-skills.bin");
    if (configPath.empty() || !ReadWeaponRuntimeConfig(configPath, true))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"weapon skill configuration is missing or invalid");
        return 1;
    }
    lm_module_t module{};
    if (!LM_FindModule("granblue_fantasy_relink.exe", &module))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"game module is unavailable");
        return 1;
    }
    const char* aggregationSignature = "56 57 53 48 83 EC 30 4C 89 C3 48 89 D6 48 89 CF 45 8B 80 A4 00 00 00 44 8B 8B A8 00 00 00 8B 89 60 5B 00 00 C7 44 24 20";
    const char* applySignature = "55 41 57 41 56 41 55 41 54 56 57 53 48 83 EC 58 48 8D 6C 24 50 48 C7 45 00 FE FF FF FF 41 81 F8 B0 E0 7A 88 0F 84 63 03 00 00 44 89 C7 48 89 D6";
    g_weaponAggregationTarget = FindUniqueSignature(aggregationSignature, module);
    const lm_address_t applyTarget = FindUniqueSignature(applySignature, module);
    if (g_weaponAggregationTarget == LM_ADDRESS_BAD || applyTarget == LM_ADDRESS_BAD)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"weapon skill signatures were missing or ambiguous");
        return 1;
    }
    lm_byte_t call[5]{};
    int32_t displacement = 0;
    if (LM_ReadMemory(g_weaponAggregationTarget + 0x2C, call, sizeof(call)) != sizeof(call) || call[0] != 0xE8)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"native weapon trait call preflight failed");
        return 1;
    }
    memcpy(&displacement, call + 1, sizeof(displacement));
    const lm_address_t resolvedApply = g_weaponAggregationTarget + 0x31 + displacement;
    if (resolvedApply != applyTarget)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"native weapon trait call target did not match the guarded aggregator");
        return 1;
    }
    g_weaponRuntimeModuleBase = module.base;
    g_applyWeaponTrait = reinterpret_cast<ApplyWeaponTraitFunction>(applyTarget);
    if (LM_ReadMemory(g_weaponAggregationTarget, g_weaponAggregationOriginal, sizeof(g_weaponAggregationOriginal)) != sizeof(g_weaponAggregationOriginal))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"weapon-skills", L"weapon aggregation entry preflight read failed");
        return 1;
    }
    g_weaponRuntimeStopping.store(false);
    g_weaponRuntimeCleanRebuilds.store(0);
    g_weaponAggregationHookSize = LM_HookCode(g_weaponAggregationTarget,
        reinterpret_cast<lm_address_t>(&WeaponTraitAggregationDetour),
        reinterpret_cast<lm_address_t*>(&g_originalWeaponAggregation));
    if (!g_weaponAggregationHookSize)
    {
        const bool restored = StopWeaponSkillsRuntime();
        WriteStartupFailureAfterStop(L"weapon-skills", restored, L"weapon aggregation hook installation failed");
        return 1;
    }
    if (g_weaponAggregationHookSize > sizeof(g_weaponAggregationInstalled) ||
        LM_ReadMemory(g_weaponAggregationTarget, g_weaponAggregationInstalled, g_weaponAggregationHookSize) != g_weaponAggregationHookSize)
    {
        // This follows immediately after our successful install, before the
        // runtime is published. Restore synchronously rather than retaining an
        // unidentifiable Hook ownership state.
        const bool restored = RestoreLibmemHookAfterDrain(g_weaponAggregationTarget,
            reinterpret_cast<lm_address_t>(g_originalWeaponAggregation), &g_weaponAggregationHookSize,
            g_weaponAggregationOriginal, sizeof(g_weaponAggregationOriginal), g_weaponRuntimeCallbacks);
        if (!restored) g_patchCoreCanUnload.store(false);
        WriteStartupFailureAfterStop(L"weapon-skills", restored, L"weapon aggregation hook ownership readback failed");
        return 1;
    }
    g_weaponAggregationInstalledSize = g_weaponAggregationHookSize;
    WriteRuntimeStatus(L"weapon-skills", L"active", L"extra weapon skills are using the native trait aggregator");
    FILETIME previous{};
    while (owner.Alive())
    {
        WIN32_FILE_ATTRIBUTE_DATA data{};
        if (!GetFileAttributesExW(configPath.c_str(), GetFileExInfoStandard, &data)) break;
        std::vector<uint8_t> headerData;
        WeaponRuntimeConfigHeader header{};
        if (!ReadSharedFile(configPath, headerData, sizeof(header) +
            static_cast<size_t>(kWeaponRuntimeMaxEntries) * sizeof(WeaponRuntimeSkillEntry)) ||
            headerData.size() < sizeof(header)) break;
        memcpy(&header, headerData.data(), sizeof(header));
        if (header.enabled != 1) break;
        if (CompareFileTime(&data.ftLastWriteTime, &previous) != 0)
        {
            previous = data.ftLastWriteTime;
            if (!ReadWeaponRuntimeConfig(configPath, false))
                WriteRuntimeStatus(L"weapon-skills", L"active", L"hot configuration was rejected; keeping the active weapon skills");
        }
        Sleep(250);
    }
    StopWeaponSkillsRuntime();
    return 0;
}

static constexpr const char* kAudioCharacterIds[] = {
    "NP0000", "NP0100", "NP0200", "PL0000", "PL0100", "PL0200", "PL0300", "PL0400", "PL0500", "PL0600", "PL0700",
    "PL0800", "PL0900", "PL1000", "PL1100", "PL1200", "PL1300", "PL1400", "PL1500", "PL1600", "PL1700", "PL1800",
    "PL1900", "PL2100", "PL2200", "PL2300", "PL2400", "PL2500", "PL2600", "PL2700", "PL2800", "PL2900"
};
static constexpr size_t kAudioCharacterCount = _countof(kAudioCharacterIds);
using PostEventFunction = uint32_t(*)(uint32_t, uint64_t, uint32_t, uintptr_t, uintptr_t, uint32_t, uintptr_t, uint32_t);
using GetIdFunction = uint32_t(*)(const char*);
using GetRtpcFunction = int(*)(uint32_t, uint64_t, uint32_t, float*, int*);
using SetRtpcFunction = int(*)(uint32_t, float, uint32_t, int, int, uint8_t);
static PostEventFunction g_originalPostEvent = nullptr;
static GetRtpcFunction g_getRtpcValue = nullptr;
static SetRtpcFunction g_setRtpcValue = nullptr;
static lm_address_t g_postEventTarget = LM_ADDRESS_BAD;
static lm_size_t g_postEventHookSize = 0;
static lm_byte_t g_postEventOriginal[32]{};
static uint32_t g_voiceRtpcId = 0;
static uint32_t g_seRtpcId = 0;
static std::unordered_map<uint32_t, size_t> g_audioEventOwners;
static std::unordered_set<uint32_t> g_audioUIEvents;
static std::array<std::atomic<int>, kAudioCharacterCount> g_audioVolumes;
static std::atomic<int> g_audioUIVolume{ 100 };
static std::atomic<bool> g_audioDiagnostic{ false };
static std::atomic<bool> g_audioStopping{ false };
static std::atomic<LONG> g_audioCallbacks{ 0 };

static int AudioCharacterIndex(const std::string& id)
{
    for (size_t index = 0; index < kAudioCharacterCount; ++index)
        if (_stricmp(id.c_str(), kAudioCharacterIds[index]) == 0) return static_cast<int>(index);
    return -1;
}

static uint32_t ReadLittle32(const uint8_t* value)
{
    return static_cast<uint32_t>(value[0]) | (static_cast<uint32_t>(value[1]) << 8) | (static_cast<uint32_t>(value[2]) << 16) | (static_cast<uint32_t>(value[3]) << 24);
}

static void ReadVoiceBankEvents(const std::filesystem::path& path, std::vector<uint32_t>& events)
{
    std::ifstream stream(path, std::ios::binary);
    if (!stream) return;
    stream.seekg(0, std::ios::end);
    std::streamoff length = stream.tellg();
    stream.seekg(0, std::ios::beg);
    while (stream && stream.tellg() >= 0 && stream.tellg() + std::streamoff(8) <= length)
    {
        uint8_t header[8]{};
        stream.read(reinterpret_cast<char*>(header), sizeof(header));
        uint32_t chunkLength = ReadLittle32(header + 4);
        std::streamoff chunkEnd = stream.tellg() + static_cast<std::streamoff>(chunkLength);
        if (!stream || chunkEnd > length) return;
        if (memcmp(header, "HIRC", 4) != 0)
        {
            stream.seekg(chunkEnd);
            continue;
        }
        uint8_t countBuffer[4]{};
        stream.read(reinterpret_cast<char*>(countBuffer), 4);
        uint32_t objectCount = ReadLittle32(countBuffer);
        for (uint32_t index = 0; index < objectCount && stream; ++index)
        {
            uint8_t objectHeader[5]{};
            stream.read(reinterpret_cast<char*>(objectHeader), sizeof(objectHeader));
            uint32_t objectLength = ReadLittle32(objectHeader + 1);
            std::streamoff objectEnd = stream.tellg() + static_cast<std::streamoff>(objectLength);
            if (!stream || objectLength < 4 || objectEnd > chunkEnd) return;
            uint8_t idBuffer[4]{};
            stream.read(reinterpret_cast<char*>(idBuffer), 4);
            if (objectHeader[0] == 4) events.push_back(ReadLittle32(idBuffer));
            stream.seekg(objectEnd);
        }
        stream.seekg(chunkEnd);
    }
}

static bool BuildAudioEventOwners()
{
    wchar_t executable[MAX_PATH]{};
    if (!GetModuleFileNameW(nullptr, executable, _countof(executable))) return false;
    std::filesystem::path sound = std::filesystem::path(executable).parent_path() / L"data" / L"sound";
    std::error_code error;
    if (!std::filesystem::is_directory(sound, error)) return false;
    std::unordered_map<uint32_t, size_t> owners;
    std::unordered_set<uint32_t> uiEvents;
    std::unordered_set<uint32_t> ambiguous;
    for (std::filesystem::recursive_directory_iterator it(sound, std::filesystem::directory_options::skip_permission_denied, error), end; it != end; it.increment(error))
    {
        if (error) { error.clear(); continue; }
        if (!it->is_regular_file(error) || _wcsicmp(it->path().extension().c_str(), L".bnk") != 0) continue;
        std::string stem = it->path().stem().string();
        if (_strnicmp(stem.c_str(), "ui", 2) == 0 && (stem.size() == 2 || stem[2] == '_'))
        {
            std::vector<uint32_t> events;
            ReadVoiceBankEvents(it->path(), events);
            uiEvents.insert(events.begin(), events.end());
            continue;
        }
        if (stem.size() < 9 || _strnicmp(stem.c_str(), "vo_", 3) != 0 || (stem.size() > 9 && stem[9] != '_')) continue;
        std::string id = stem.substr(3, 6);
        for (char& value : id) value = static_cast<char>(toupper(static_cast<unsigned char>(value)));
        int owner = AudioCharacterIndex(id);
        if (owner < 0) continue;
        std::vector<uint32_t> events;
        ReadVoiceBankEvents(it->path(), events);
        for (uint32_t eventId : events)
        {
            if (ambiguous.contains(eventId)) continue;
            auto current = owners.find(eventId);
            if (current != owners.end() && current->second != static_cast<size_t>(owner))
            {
                owners.erase(current);
                ambiguous.insert(eventId);
            }
            else
            {
                owners[eventId] = static_cast<size_t>(owner);
            }
        }
    }
    for (const auto& [eventId, owner] : owners) uiEvents.erase(eventId);
    g_audioEventOwners.swap(owners);
    g_audioUIEvents.swap(uiEvents);
    return !g_audioEventOwners.empty() && !g_audioUIEvents.empty();
}

static bool LoadAudioConfig(const wchar_t* path, bool requireEnabled)
{
    bool enabled = GetPrivateProfileIntW(L"audio", L"enabled", 0, path) == 1;
    if (requireEnabled && !enabled) return false;
    g_audioDiagnostic.store(GetPrivateProfileIntW(L"audio", L"diagnostic", 0, path) == 1);
    int uiVolume = GetPrivateProfileIntW(L"ui", L"volume", 100, path);
    if (uiVolume < 0 || uiVolume > 100) return false;
    g_audioUIVolume.store(uiVolume);
    for (size_t index = 0; index < kAudioCharacterCount; ++index)
    {
        wchar_t id[16]{};
        MultiByteToWideChar(CP_ACP, 0, kAudioCharacterIds[index], -1, id, _countof(id));
        int volume = GetPrivateProfileIntW(L"volumes", id, 100, path);
        if (volume < 0 || volume > 100) return false;
        g_audioVolumes[index].store(volume);
    }
    return enabled;
}

static uint32_t PostEventDetour(uint32_t eventId, uint64_t gameObjectId, uint32_t callbackFlags, uintptr_t callback, uintptr_t cookie,
    uint32_t sourceCount, uintptr_t sources, uint32_t playingId)
{
    g_audioCallbacks.fetch_add(1);
    uint32_t result = g_originalPostEvent ? g_originalPostEvent(eventId, gameObjectId, callbackFlags, callback, cookie, sourceCount, sources, playingId) : 0;
    if (!g_audioStopping.load() && result)
    {
        auto owner = g_audioEventOwners.find(eventId);
        if (g_audioDiagnostic.load())
        {
            char message[192]{};
            const int ownerIndex = owner != g_audioEventOwners.end() ? static_cast<int>(owner->second) : -1;
            const char* category = ownerIndex >= 0 ? "character" : g_audioUIEvents.contains(eventId) ? "ui" : "unmapped";
            sprintf_s(message, "GBFR audio event=%u playing=%u object=%llu category=%s owner=%d\n",
                eventId, result, static_cast<unsigned long long>(gameObjectId), category, ownerIndex);
            OutputDebugStringA(message);
        }
        if (owner != g_audioEventOwners.end())
        {
            int volume = g_audioVolumes[owner->second].load();
            if (volume < 100)
            {
                int valueType = 1;
                float master = 0.0f;
                if (g_getRtpcValue && g_setRtpcValue && g_getRtpcValue(g_voiceRtpcId, UINT64_MAX, 0, &master, &valueType) == 1 && std::isfinite(master))
                    g_setRtpcValue(g_voiceRtpcId, master * static_cast<float>(volume) / 100.0f, result, 0, 4, 0);
            }
        }
        else if (g_audioUIEvents.contains(eventId))
        {
            int volume = g_audioUIVolume.load();
            if (volume < 100)
            {
                int valueType = 1;
                float master = 0.0f;
                if (g_getRtpcValue && g_setRtpcValue && g_getRtpcValue(g_seRtpcId, UINT64_MAX, 0, &master, &valueType) == 1 && std::isfinite(master))
                    g_setRtpcValue(g_seRtpcId, master * static_cast<float>(volume) / 100.0f, result, 0, 4, 0);
            }
        }
    }
    g_audioCallbacks.fetch_sub(1);
    return result;
}

static bool StopAudioRuntime()
{
	g_audioStopping.store(true);
	bool hookRestored = g_postEventTarget == LM_ADDRESS_BAD || RestoreLibmemHookAfterDrain(g_postEventTarget,
		reinterpret_cast<lm_address_t>(g_originalPostEvent), &g_postEventHookSize, g_postEventOriginal, sizeof(g_postEventOriginal), g_audioCallbacks);
	WriteRuntimeStatus(L"audio", hookRestored ? L"inactive" : L"restore_failed",
		hookRestored ? L"audio hook restored after active callbacks drained" : L"audio entry restoration could not be proven; module kept loaded");
    ReleaseRuntimeOwnerAfterVerifiedStop(L"audio", hookRestored);
	return hookRestored;
}

static DWORD RunAudioRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"audio"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"audio", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    std::wstring configPath = RuntimePath(L"audio.ini");
    if (configPath.empty() || !LoadAudioConfig(configPath.c_str(), true))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"audio", L"audio configuration is missing or invalid");
        return 1;
    }
    HMODULE game = GetModuleHandleW(nullptr);
    const char* postExport = "?PostEvent@SoundEngine@AK@@YAII_KIP6AXW4AkCallbackType@@PEAUAkCallbackInfo@@@ZPEAXIPEAUAkExternalSourceInfo@@I@Z";
    const char* getIdExport = "?GetIDFromString@SoundEngine@AK@@YAIPEBD@Z";
    const char* getRtpcExport = "?GetRTPCValue@Query@SoundEngine@AK@@YA?AW4AKRESULT@@I_KIAEAMAEAW4RTPCValue_type@123@@Z";
    const char* setRtpcExport = "?SetRTPCValueByPlayingID@SoundEngine@AK@@YA?AW4AKRESULT@@IMIHW4AkCurveInterpolation@@_N@Z";
    g_postEventTarget = reinterpret_cast<lm_address_t>(GetProcAddress(game, postExport));
    GetIdFunction getId = reinterpret_cast<GetIdFunction>(GetProcAddress(game, getIdExport));
    g_getRtpcValue = reinterpret_cast<GetRtpcFunction>(GetProcAddress(game, getRtpcExport));
    g_setRtpcValue = reinterpret_cast<SetRtpcFunction>(GetProcAddress(game, setRtpcExport));
    if (g_postEventTarget == LM_ADDRESS_BAD || !g_postEventTarget || !getId || !g_getRtpcValue || !g_setRtpcValue || !BuildAudioEventOwners())
    {
        WriteRuntimeInactiveAndReleaseOwner(L"audio", L"Wwise exports or character voice banks are unavailable");
        return 1;
    }
    g_voiceRtpcId = getId("Volume_Voice");
    g_seRtpcId = getId("Volume_SE");
    if (!g_voiceRtpcId || !g_seRtpcId)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"audio", L"Volume_Voice or Volume_SE RTPC was not resolved");
        return 1;
    }
	if (LM_ReadMemory(g_postEventTarget, g_postEventOriginal, sizeof(g_postEventOriginal)) != sizeof(g_postEventOriginal))
	{
		WriteRuntimeInactiveAndReleaseOwner(L"audio", L"audio entry preflight read failed");
		return 1;
	}
	g_postEventHookSize = LM_HookCode(g_postEventTarget, reinterpret_cast<lm_address_t>(&PostEventDetour), reinterpret_cast<lm_address_t*>(&g_originalPostEvent));
    if (!g_postEventHookSize)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"audio", L"audio hook installation failed");
        return 1;
    }
    WriteRuntimeStatus(L"audio", L"active", L"native Wwise character-volume runtime is active");
    FILETIME previous{};
    while (owner.Alive())
    {
        WIN32_FILE_ATTRIBUTE_DATA data{};
        if (!GetFileAttributesExW(configPath.c_str(), GetFileExInfoStandard, &data) || GetPrivateProfileIntW(L"audio", L"enabled", 0, configPath.c_str()) != 1) break;
        if (CompareFileTime(&data.ftLastWriteTime, &previous) != 0)
        {
            previous = data.ftLastWriteTime;
            if (!LoadAudioConfig(configPath.c_str(), false)) WriteRuntimeStatus(L"audio", L"active", L"hot configuration was rejected; keeping the last valid volumes");
        }
        Sleep(250);
    }
    StopAudioRuntime();
    return 0;
}

#pragma pack(push, 1)
struct RuntimePartyObserverHeader
{
    uint64_t magic;
    uint32_t version;
    uint32_t capacity;
    volatile LONG64 writeSequence;
    volatile LONG64 droppedEvents;
};

struct RuntimePartyObserverSigil
{
    uint32_t hash;
    uint32_t secondaryHash;
    uint32_t level;
};

struct RuntimePartyObserverEvent
{
    volatile LONG64 sequence;
    uint64_t tickMillis;
    uint32_t kind;
    uint32_t direction;
    uint32_t partyIndex;
    uint32_t characterHash;
    uint32_t weaponHash;
    uint32_t profileSize;
    RuntimePartyObserverSigil sigils[12];
};
#pragma pack(pop)

static_assert(sizeof(RuntimePartyObserverHeader) == 32, "runtime Party observer header layout changed");
static_assert(sizeof(RuntimePartyObserverEvent) == 184, "runtime Party observer event layout changed");

static constexpr uint64_t kRuntimePartyObserverMagic = 0x31564F5052464247ULL;
static constexpr uint32_t kRuntimePartyObserverVersion = 1;
static constexpr uint32_t kRuntimePartyObserverCapacity = 128;
static constexpr uint32_t kRuntimePartyObserverProfileKind = 1;
static constexpr uint32_t kRuntimePartyObserverResetKind = 2;
static constexpr uint32_t kRuntimePartyObserverLocal = 1;
static constexpr uint32_t kRuntimePartyObserverRemote = 2;
static const wchar_t* kRuntimePartyObserverMappingName = L"Local\\GBFRPlayerInfoEditPartyProfilesV1";

struct PartyObserverDataBuffer
{
    const void* buffer;
    uint32_t bufferByteCount;
    uint32_t padding;
};

struct PartyObserverStateChange
{
    uint32_t stateChangeType;
};

struct PartyObserverEndpointMessageReceived
{
    PartyObserverStateChange stateChange;
    uint32_t padding0;
    const void* network;
    const void* senderEndpoint;
    uint32_t receiverEndpointCount;
    uint32_t padding1;
    const void* const* receiverEndpoints;
    uint32_t options;
    uint32_t messageSize;
    const void* messageBuffer;
};

using PartyObserverSendMessageFunction = int32_t(__stdcall*)(const void*, uint32_t, const void* const*, uint32_t,
    const void*, uint32_t, const PartyObserverDataBuffer*, void*);
using PartyObserverStartChangesFunction = int32_t(__stdcall*)(const void*, uint32_t*, const PartyObserverStateChange* const**);

static PartyObserverSendMessageFunction g_originalPartySendMessage = nullptr;
static PartyObserverStartChangesFunction g_originalPartyStartChanges = nullptr;
static lm_address_t g_partySendTarget = LM_ADDRESS_BAD;
static lm_address_t g_partyStartTarget = LM_ADDRESS_BAD;
static lm_size_t g_partySendHookSize = 0;
static lm_size_t g_partyStartHookSize = 0;
static lm_byte_t g_partySendOriginal[32]{};
static lm_byte_t g_partyStartOriginal[32]{};
static std::atomic<LONG> g_partySendCallbacks{ 0 };
static std::atomic<LONG> g_partyStartCallbacks{ 0 };
static std::atomic<bool> g_partyObserverStopping{ false };
static SRWLOCK g_partyObserverPublishLock = SRWLOCK_INIT;
static HANDLE g_partyObserverMapping = nullptr;
static RuntimePartyObserverHeader* g_partyObserverHeader = nullptr;

static bool InitializePartyObserverMapping()
{
    const DWORD size = static_cast<DWORD>(sizeof(RuntimePartyObserverHeader) + sizeof(RuntimePartyObserverEvent) * kRuntimePartyObserverCapacity);
    g_partyObserverMapping = CreateFileMappingW(INVALID_HANDLE_VALUE, nullptr, PAGE_READWRITE, 0, size, kRuntimePartyObserverMappingName);
    if (!g_partyObserverMapping) return false;
    auto* view = reinterpret_cast<uint8_t*>(MapViewOfFile(g_partyObserverMapping, FILE_MAP_ALL_ACCESS, 0, 0, size));
    if (!view)
    {
        CloseHandle(g_partyObserverMapping);
        g_partyObserverMapping = nullptr;
        return false;
    }
    SecureZeroMemory(view, size);
    g_partyObserverHeader = reinterpret_cast<RuntimePartyObserverHeader*>(view);
    g_partyObserverHeader->magic = kRuntimePartyObserverMagic;
    g_partyObserverHeader->version = kRuntimePartyObserverVersion;
    g_partyObserverHeader->capacity = kRuntimePartyObserverCapacity;
    return true;
}

static void ClosePartyObserverMapping()
{
    if (g_partyObserverHeader)
    {
        UnmapViewOfFile(g_partyObserverHeader);
        g_partyObserverHeader = nullptr;
    }
    if (g_partyObserverMapping)
    {
        CloseHandle(g_partyObserverMapping);
        g_partyObserverMapping = nullptr;
    }
}

static uint32_t PartyObserverReadU32(const uint8_t* payload, size_t offset)
{
    uint32_t value = 0;
    memcpy(&value, payload + offset, sizeof(value));
    return value;
}

static void PublishPartyObserverEvent(const RuntimePartyObserverEvent& value)
{
    AcquireSRWLockExclusive(&g_partyObserverPublishLock);
    RuntimePartyObserverHeader* header = g_partyObserverHeader;
    if (!header || g_partyObserverStopping.load())
    {
        ReleaseSRWLockExclusive(&g_partyObserverPublishLock);
        return;
    }
    const LONG64 sequence = header->writeSequence + 1;
    if (sequence > static_cast<LONG64>(header->capacity)) InterlockedIncrement64(&header->droppedEvents);
    auto* events = reinterpret_cast<RuntimePartyObserverEvent*>(reinterpret_cast<uint8_t*>(header) + sizeof(RuntimePartyObserverHeader));
    RuntimePartyObserverEvent* event = &events[(sequence - 1) % header->capacity];
    InterlockedExchange64(&event->sequence, 0);
    memcpy(reinterpret_cast<uint8_t*>(event) + sizeof(event->sequence),
        reinterpret_cast<const uint8_t*>(&value) + sizeof(value.sequence), sizeof(*event) - sizeof(event->sequence));
    MemoryBarrier();
    InterlockedExchange64(&event->sequence, sequence);
    MemoryBarrier();
    InterlockedExchange64(&header->writeSequence, sequence);
    ReleaseSRWLockExclusive(&g_partyObserverPublishLock);
}

static void PublishPartyObserverReset()
{
    RuntimePartyObserverEvent event{};
    event.tickMillis = GetTickCount64();
    event.kind = kRuntimePartyObserverResetKind;
    PublishPartyObserverEvent(event);
}

static void CapturePartyObserverProfile(uint32_t direction, const void* rawPayload, uint32_t payloadSize)
{
    RuntimePartyObserverHeader* header = g_partyObserverHeader;
    if (!header || !rawPayload || g_partyObserverStopping.load()) return;
    if (direction != kRuntimePartyObserverLocal && direction != kRuntimePartyObserverRemote) return;
    if (payloadSize != 780 && payloadSize != 784) return;
    __try
    {
        const auto* payload = reinterpret_cast<const uint8_t*>(rawPayload);
        const uint32_t group = PartyObserverReadU32(payload, 0);
        const uint32_t type = PartyObserverReadU32(payload, 4);
        const uint32_t declared = PartyObserverReadU32(payload, 8);
        const uint32_t version = PartyObserverReadU32(payload, 12);
        const bool verifiedHeader = declared == payloadSize && version == 1 &&
            ((payloadSize == 784 && group == 3 && type == 14) || (payloadSize == 780 && group == 2 && type == 63));
        if (!verifiedHeader) return;
        const uint32_t partyIndex = PartyObserverReadU32(payload, 0x2B4);
        if (partyIndex >= 4) return;

        RuntimePartyObserverEvent event{};
        event.tickMillis = GetTickCount64();
        event.kind = kRuntimePartyObserverProfileKind;
        event.direction = direction;
        event.partyIndex = partyIndex;
        event.characterHash = PartyObserverReadU32(payload, 0x2B8);
        event.weaponHash = PartyObserverReadU32(payload, 0x1BC);
        event.profileSize = payloadSize;
        for (size_t index = 0; index < _countof(event.sigils); ++index)
        {
            event.sigils[index].hash = PartyObserverReadU32(payload, 0x1F4 + index * 4);
            event.sigils[index].secondaryHash = PartyObserverReadU32(payload, 0x224 + index * 4);
            event.sigils[index].level = payload[0x25C + index];
        }
        PublishPartyObserverEvent(event);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
}

static void CapturePartyObserverSendBuffers(uint32_t dataBufferCount, const PartyObserverDataBuffer* dataBuffers)
{
    if (!dataBuffers || dataBufferCount == 0 || dataBufferCount > 16) return;
    __try
    {
        std::array<uint8_t, 784> payload{};
        size_t total = 0;
        for (uint32_t index = 0; index < dataBufferCount; ++index)
        {
            const PartyObserverDataBuffer& source = dataBuffers[index];
            if (!source.buffer || source.bufferByteCount > payload.size() - total) return;
            memcpy(payload.data() + total, source.buffer, source.bufferByteCount);
            total += source.bufferByteCount;
        }
        if (total == 780 || total == 784) CapturePartyObserverProfile(kRuntimePartyObserverLocal, payload.data(), static_cast<uint32_t>(total));
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
}

static int32_t __stdcall PartyObserverSendMessageDetour(const void* endpoint, uint32_t targetEndpointCount,
    const void* const* targetEndpoints, uint32_t options, const void* queuingConfiguration,
    uint32_t dataBufferCount, const PartyObserverDataBuffer* dataBuffers, void* messageIdentifier)
{
    g_partySendCallbacks.fetch_add(1);
    if (!g_partyObserverStopping.load()) CapturePartyObserverSendBuffers(dataBufferCount, dataBuffers);
    const int32_t result = g_originalPartySendMessage ? g_originalPartySendMessage(endpoint, targetEndpointCount, targetEndpoints,
        options, queuingConfiguration, dataBufferCount, dataBuffers, messageIdentifier) : -1;
    g_partySendCallbacks.fetch_sub(1);
    return result;
}

static void CapturePartyObserverStateChanges(uint32_t count, const PartyObserverStateChange* const* changes)
{
    if (!changes || count > 4096) return;
    __try
    {
        for (uint32_t index = 0; index < count; ++index)
        {
            const PartyObserverStateChange* change = changes[index];
            if (!change) continue;
            const uint32_t type = change->stateChangeType;
            if (type == 21)
            {
                const auto* message = reinterpret_cast<const PartyObserverEndpointMessageReceived*>(change);
                CapturePartyObserverProfile(kRuntimePartyObserverRemote, message->messageBuffer, message->messageSize);
            }
            else if (type == 12 || type == 13 || type == 17 || type == 20)
            {
                PublishPartyObserverReset();
            }
        }
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
}

static int32_t __stdcall PartyObserverStartChangesDetour(const void* handle, uint32_t* stateChangeCount,
    const PartyObserverStateChange* const** stateChanges)
{
    g_partyStartCallbacks.fetch_add(1);
    const int32_t result = g_originalPartyStartChanges ? g_originalPartyStartChanges(handle, stateChangeCount, stateChanges) : -1;
    if (!g_partyObserverStopping.load() && result == 0 && stateChangeCount && stateChanges)
    {
        __try
        {
            CapturePartyObserverStateChanges(*stateChangeCount, *stateChanges);
        }
        __except (EXCEPTION_EXECUTE_HANDLER)
        {
        }
    }
    g_partyStartCallbacks.fetch_sub(1);
    return result;
}

static bool StopPartyObserverRuntime()
{
    g_partyObserverStopping.store(true);
    bool restored = true;
    if (g_partyStartTarget != LM_ADDRESS_BAD)
    {
        restored = RestoreLibmemHookAfterDrain(g_partyStartTarget, reinterpret_cast<lm_address_t>(g_originalPartyStartChanges),
            &g_partyStartHookSize, g_partyStartOriginal, sizeof(g_partyStartOriginal), g_partyStartCallbacks) && restored;
    }
    if (g_partySendTarget != LM_ADDRESS_BAD)
    {
        restored = RestoreLibmemHookAfterDrain(g_partySendTarget, reinterpret_cast<lm_address_t>(g_originalPartySendMessage),
            &g_partySendHookSize, g_partySendOriginal, sizeof(g_partySendOriginal), g_partySendCallbacks) && restored;
    }
    if (restored) ClosePartyObserverMapping();
    WriteRuntimeStatus(L"party-observer", restored ? L"inactive" : L"restore_failed",
        restored ? L"Party lifecycle observer hooks restored after active callbacks drained" : L"Party lifecycle observer restoration could not be proven; module kept loaded");
    ReleaseRuntimeOwnerAfterVerifiedStop(L"party-observer", restored);
    return restored;
}

static DWORD RunPartyObserverRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"party-observer"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"party-observer", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    const std::wstring configPath = RuntimePath(L"party-observer.ini");
    if (configPath.empty() || GetPrivateProfileIntW(L"party-observer", L"enabled", 0, configPath.c_str()) != 1)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"party-observer", L"Party lifecycle observer is disabled");
        return 1;
    }
    lm_module_t partyModule{};
    DWORD waitDeadline = GetTickCount() + 30000;
    while (owner.Alive() && GetPrivateProfileIntW(L"party-observer", L"enabled", 0, configPath.c_str()) == 1 &&
        !LM_FindModule("PartyWin.dll", &partyModule) && static_cast<LONG>(GetTickCount() - waitDeadline) < 0)
    {
        Sleep(100);
    }
    HMODULE party = GetModuleHandleW(L"PartyWin.dll");
    g_partySendTarget = reinterpret_cast<lm_address_t>(party ? GetProcAddress(party, "PartyEndpointSendMessage") : nullptr);
    g_partyStartTarget = reinterpret_cast<lm_address_t>(party ? GetProcAddress(party, "PartyStartProcessingStateChanges") : nullptr);
    const auto inPartyModule = [&partyModule](lm_address_t address) {
        return partyModule.base != LM_ADDRESS_BAD && address >= partyModule.base && address < partyModule.base + partyModule.size;
    };
    if (!party || !inPartyModule(g_partySendTarget) || !inPartyModule(g_partyStartTarget))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"party-observer", L"PartyWin lifecycle exports are unavailable");
        return 1;
    }
    if (!InitializePartyObserverMapping())
    {
        WriteRuntimeInactiveAndReleaseOwner(L"party-observer", L"Party profile shared memory could not be created");
        return 1;
    }
    if (LM_ReadMemory(g_partySendTarget, g_partySendOriginal, sizeof(g_partySendOriginal)) != sizeof(g_partySendOriginal) ||
        LM_ReadMemory(g_partyStartTarget, g_partyStartOriginal, sizeof(g_partyStartOriginal)) != sizeof(g_partyStartOriginal))
    {
        ClosePartyObserverMapping();
        WriteRuntimeInactiveAndReleaseOwner(L"party-observer", L"Party lifecycle export preflight read failed");
        return 1;
    }
    g_partyObserverStopping.store(false);
    g_partySendHookSize = LM_HookCode(g_partySendTarget, reinterpret_cast<lm_address_t>(&PartyObserverSendMessageDetour),
        reinterpret_cast<lm_address_t*>(&g_originalPartySendMessage));
    if (!g_partySendHookSize)
    {
        ClosePartyObserverMapping();
        WriteRuntimeInactiveAndReleaseOwner(L"party-observer", L"Party send observer hook installation failed");
        return 1;
    }
    g_partyStartHookSize = LM_HookCode(g_partyStartTarget, reinterpret_cast<lm_address_t>(&PartyObserverStartChangesDetour),
        reinterpret_cast<lm_address_t*>(&g_originalPartyStartChanges));
    if (!g_partyStartHookSize)
    {
        StopPartyObserverRuntime();
        return 1;
    }
    WriteRuntimeStatus(L"party-observer", L"active", L"read-only Party lifecycle profile observer is active");
    while (owner.Alive() && GetPrivateProfileIntW(L"party-observer", L"enabled", 0, configPath.c_str()) == 1) Sleep(100);
    StopPartyObserverRuntime();
    return 0;
}

#pragma pack(push, 1)
struct RuntimeDamageHeader
{
    uint64_t magic;
    uint32_t version;
    uint32_t capacity;
    volatile LONG64 writeSequence;
    volatile LONG64 droppedEvents;
};

struct RuntimeDamageEvent
{
    volatile LONG64 sequence;
    uint64_t tickMillis;
    uint64_t sourceAddress;
    uint64_t targetAddress;
    int32_t damage;
    int32_t damageCap;
    float baseDamage;
    float attackRate;
    uint64_t flags;
    uint32_t actionId;
    uint32_t reserved;
};
#pragma pack(pop)

static_assert(sizeof(RuntimeDamageHeader) == 32, "runtime damage header layout changed");
static_assert(sizeof(RuntimeDamageEvent) == 64, "runtime damage event layout changed");

static constexpr uint64_t kRuntimeDamageMagic = 0x31564D4446524247ULL;
static constexpr uint32_t kRuntimeDamageVersion = 1;
static constexpr uint32_t kRuntimeDamageCapacity = 4096;
static const wchar_t* kRuntimeDamageMappingName = L"Local\\GBFRPlayerInfoEditDamageEventsV1";

using ProcessDamageEventFunction = uintptr_t(*)(const uintptr_t*, const uintptr_t*, const uintptr_t*, uint8_t);
static ProcessDamageEventFunction g_originalProcessDamageEvent = nullptr;
static lm_address_t g_processDamageTarget = LM_ADDRESS_BAD;
static lm_size_t g_processDamageHookSize = 0;
static lm_byte_t g_processDamageOriginal[32]{};
static std::atomic<LONG> g_damageCallbacks{ 0 };
static std::atomic<bool> g_damageStopping{ false };
static HANDLE g_damageMapping = nullptr;
static RuntimeDamageHeader* g_damageHeader = nullptr;

static bool InitializeDamageMapping()
{
    const DWORD size = static_cast<DWORD>(sizeof(RuntimeDamageHeader) + sizeof(RuntimeDamageEvent) * kRuntimeDamageCapacity);
    g_damageMapping = CreateFileMappingW(INVALID_HANDLE_VALUE, nullptr, PAGE_READWRITE, 0, size, kRuntimeDamageMappingName);
    if (!g_damageMapping) return false;
    auto* view = reinterpret_cast<uint8_t*>(MapViewOfFile(g_damageMapping, FILE_MAP_ALL_ACCESS, 0, 0, size));
    if (!view)
    {
        CloseHandle(g_damageMapping);
        g_damageMapping = nullptr;
        return false;
    }
    SecureZeroMemory(view, size);
    g_damageHeader = reinterpret_cast<RuntimeDamageHeader*>(view);
    g_damageHeader->magic = kRuntimeDamageMagic;
    g_damageHeader->version = kRuntimeDamageVersion;
    g_damageHeader->capacity = kRuntimeDamageCapacity;
    return true;
}

static void CloseDamageMapping()
{
    if (g_damageHeader)
    {
        UnmapViewOfFile(g_damageHeader);
        g_damageHeader = nullptr;
    }
    if (g_damageMapping)
    {
        CloseHandle(g_damageMapping);
        g_damageMapping = nullptr;
    }
}

static lm_address_t ResolveRelativeCall(lm_address_t callAddress)
{
    if (callAddress == LM_ADDRESS_BAD || *reinterpret_cast<const uint8_t*>(callAddress) != 0xE8) return LM_ADDRESS_BAD;
    int32_t displacement = 0;
    if (LM_ReadMemory(callAddress + 1, reinterpret_cast<lm_byte_t*>(&displacement), sizeof(displacement)) != sizeof(displacement)) return LM_ADDRESS_BAD;
    return callAddress + 5 + static_cast<intptr_t>(displacement);
}

static uintptr_t ReadDamageSourceAddress(const uintptr_t* damageInstance)
{
    if (!damageInstance) return 0;
    __try
    {
        uintptr_t entity = *reinterpret_cast<const uintptr_t*>(reinterpret_cast<uintptr_t>(damageInstance) + 0x18);
        return entity ? *reinterpret_cast<const uintptr_t*>(entity + 0x70) : 0;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return 0;
    }
}

static uintptr_t ReadDamageTargetAddress(const uintptr_t* receiver)
{
    if (!receiver) return 0;
    __try
    {
        uintptr_t wrapper = *reinterpret_cast<const uintptr_t*>(reinterpret_cast<uintptr_t>(receiver) + 0x08);
        return wrapper ? *reinterpret_cast<const uintptr_t*>(wrapper) : 0;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return 0;
    }
}

static void CaptureDamageEvent(const uintptr_t* receiver, const uintptr_t* damageInstance)
{
    RuntimeDamageHeader* header = g_damageHeader;
    if (!header || !damageInstance) return;
    __try
    {
        const uintptr_t address = reinterpret_cast<uintptr_t>(damageInstance);
        const int32_t damage = *reinterpret_cast<const int32_t*>(address + 0xD4);
        const float attackRate = *reinterpret_cast<const float*>(address + 0xDC);
        const uint64_t flags = *reinterpret_cast<const uint64_t*>(address + 0xE8);
        const uint32_t actionId = *reinterpret_cast<const uint32_t*>(address + 0x16C);
        const int32_t damageCap = *reinterpret_cast<const int32_t*>(address + 0x2BC);
        const float baseDamage = *reinterpret_cast<const float*>(address + 0x2D4);
        if (damage < 0 || !std::isfinite(baseDamage) || !std::isfinite(attackRate)) return;

        const LONG64 sequence = InterlockedIncrement64(&header->writeSequence);
        if (sequence > static_cast<LONG64>(header->capacity)) InterlockedIncrement64(&header->droppedEvents);
        auto* events = reinterpret_cast<RuntimeDamageEvent*>(reinterpret_cast<uint8_t*>(header) + sizeof(RuntimeDamageHeader));
        RuntimeDamageEvent* event = &events[(sequence - 1) % header->capacity];
        InterlockedExchange64(&event->sequence, 0);
        event->tickMillis = GetTickCount64();
        event->sourceAddress = ReadDamageSourceAddress(damageInstance);
        event->targetAddress = ReadDamageTargetAddress(receiver);
        event->damage = damage;
        event->damageCap = damageCap;
        event->baseDamage = baseDamage;
        event->attackRate = attackRate;
        event->flags = flags;
        event->actionId = actionId;
        event->reserved = 0;
        MemoryBarrier();
        InterlockedExchange64(&event->sequence, sequence);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
}

static uintptr_t ProcessDamageEventDetour(const uintptr_t* receiver, const uintptr_t* damageInstance, const uintptr_t* context, uint8_t mode)
{
    g_damageCallbacks.fetch_add(1);
    uintptr_t result = g_originalProcessDamageEvent ? g_originalProcessDamageEvent(receiver, damageInstance, context, mode) : 0;
    if (!g_damageStopping.load()) CaptureDamageEvent(receiver, damageInstance);
    g_damageCallbacks.fetch_sub(1);
    return result;
}

static bool StopDamageRuntime()
{
    g_damageStopping.store(true);
    const bool restored = g_processDamageTarget == LM_ADDRESS_BAD || RestoreLibmemHookAfterDrain(g_processDamageTarget,
        reinterpret_cast<lm_address_t>(g_originalProcessDamageEvent), &g_processDamageHookSize,
        g_processDamageOriginal, sizeof(g_processDamageOriginal), g_damageCallbacks);
    CloseDamageMapping();
    WriteRuntimeStatus(L"damage", restored ? L"inactive" : L"restore_failed",
        restored ? L"damage hook restored after active callbacks drained" : L"damage entry restoration could not be proven; module kept loaded");
    ReleaseRuntimeOwnerAfterVerifiedStop(L"damage", restored);
	return restored;
}

static DWORD RunDamageRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"damage"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    std::wstring configPath = RuntimePath(L"damage.ini");
    if (configPath.empty() || GetPrivateProfileIntW(L"damage", L"enabled", 0, configPath.c_str()) != 1)
    {
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"damage capture configuration is missing or disabled");
        return 1;
    }
    lm_module_t module{};
    if (!LM_FindModule("granblue_fantasy_relink.exe", &module))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"game module is unavailable");
        return 1;
    }
    const char* signature = "E8 ?? ?? ?? ?? 66 83 BC 24 ?? ?? ?? ?? ?? 74 ?? F6 84 24";
    lm_address_t call = FindUniqueSignature(signature, module);
    g_processDamageTarget = ResolveRelativeCall(call);
    const lm_byte_t expected[] = { 0x55, 0x41, 0x57, 0x41, 0x56, 0x41, 0x55, 0x41, 0x54, 0x56, 0x57, 0x53 };
    lm_byte_t preflight[sizeof(expected)]{};
    if (call == LM_ADDRESS_BAD || g_processDamageTarget == LM_ADDRESS_BAD ||
        LM_ReadMemory(g_processDamageTarget, preflight, sizeof(preflight)) != sizeof(preflight) || !BytesEqual(preflight, expected, sizeof(expected)))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"damage event signature preflight failed or was ambiguous");
        return 1;
    }
    if (!InitializeDamageMapping())
    {
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"damage event shared memory could not be created");
        return 1;
    }
    if (LM_ReadMemory(g_processDamageTarget, g_processDamageOriginal, sizeof(g_processDamageOriginal)) != sizeof(g_processDamageOriginal))
    {
        CloseDamageMapping();
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"damage event entry preflight read failed");
        return 1;
    }
    g_damageStopping.store(false);
    g_processDamageHookSize = LM_HookCode(g_processDamageTarget, reinterpret_cast<lm_address_t>(&ProcessDamageEventDetour),
        reinterpret_cast<lm_address_t*>(&g_originalProcessDamageEvent));
    if (!g_processDamageHookSize)
    {
        CloseDamageMapping();
        WriteRuntimeInactiveAndReleaseOwner(L"damage", L"damage event hook installation failed");
        return 1;
    }
    WriteRuntimeStatus(L"damage", L"active", L"current-session damage ring buffer is active");
    while (owner.Alive() && GetPrivateProfileIntW(L"damage", L"enabled", 0, configPath.c_str()) == 1) Sleep(100);
    StopDamageRuntime();
    return 0;
}

#pragma pack(push, 1)
struct RuntimeQOLState
{
    uint64_t magic;
    uint32_t version;
    uint32_t pid;
    volatile LONG64 sessionGeneration;
    char latestSessionId[32];
    volatile LONG64 sessionSequence;
};
#pragma pack(pop)

static_assert(sizeof(RuntimeQOLState) == 64, "runtime QOL state layout changed");
static constexpr uint64_t kRuntimeQOLMagic = 0x31564C4F51465247ULL;
static constexpr uint32_t kRuntimeQOLVersion = 2;
static const wchar_t* kRuntimeQOLMappingName = L"Local\\GBFRPlayerInfoEditQOLV2";

struct QOLGameString
{
    const char* pointer;
    uint32_t length;
    uint32_t padding;
};

struct QOLEntityReference
{
    uint32_t actorId;
    uint32_t padding;
    uintptr_t wrapper;
    uint64_t timestamp;
};

struct QOLDamageSource
{
    uintptr_t field00;
    uintptr_t field08;
    QOLEntityReference source;
    QOLEntityReference attack;
    QOLEntityReference target;
};

using QOLDamageCapFunction = void(*)(uint8_t*, QOLDamageSource*);
using QOLEnemyHealthFunction = void(*)(uintptr_t, float);
using QOLPlayerParamFunction = void(*)(uintptr_t);
using QOLSetTextFromIntFunction = void(*)(uintptr_t, int64_t);
using QOLTextSetFunction = void(*)(uintptr_t, QOLGameString*, uint32_t, int);

static QOLDamageCapFunction g_qolOriginalDamageCap = nullptr;
static QOLEnemyHealthFunction g_qolOriginalEnemyHealth = nullptr;
static QOLPlayerParamFunction g_qolOriginalPlayerParam = nullptr;
static QOLSetTextFromIntFunction g_qolOriginalSetTextFromInt = nullptr;
static QOLTextSetFunction g_qolOriginalTextSet = nullptr;
static lm_address_t g_qolDamageCapTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolEnemyHealthTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolPlayerParamTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolSetTextFromIntTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolTextSetTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolDamageCapFlagAddress = LM_ADDRESS_BAD;
static lm_address_t g_qolDamageCapQuestCheck = LM_ADDRESS_BAD;
static lm_address_t g_qolEnemyPercentPatch = LM_ADDRESS_BAD;
static lm_address_t g_qolSBAPercentPatch = LM_ADDRESS_BAD;
static lm_size_t g_qolDamageCapHookSize = 0;
static lm_size_t g_qolEnemyHealthHookSize = 0;
static lm_size_t g_qolPlayerParamHookSize = 0;
static lm_size_t g_qolSetTextFromIntHookSize = 0;
static lm_size_t g_qolTextSetHookSize = 0;
static lm_byte_t g_qolDamageCapOriginal[32]{};
static lm_byte_t g_qolEnemyHealthOriginal[32]{};
static lm_byte_t g_qolPlayerParamOriginal[32]{};
static lm_byte_t g_qolSetTextFromIntOriginal[32]{};
static lm_byte_t g_qolTextSetOriginal[32]{};
static lm_byte_t g_qolQuestCheckOriginal[2]{};
static lm_byte_t g_qolEnemyPercentOriginal[18]{};
static lm_byte_t g_qolSBAPercentOriginal[12]{};
static bool g_qolEnemyPercentPatched = false;
static bool g_qolSBAPercentPatched = false;
static bool g_qolDamageCapQuestPatched = false;
static HANDLE g_qolMapping = nullptr;
static RuntimeQOLState* g_qolState = nullptr;
static SRWLOCK g_qolDamageCapLock = SRWLOCK_INIT;
static SRWLOCK g_qolSessionLock = SRWLOCK_INIT;
static std::atomic<LONG> g_qolCallbacks{ 0 };
static std::atomic<bool> g_qolStopping{ false };
static std::atomic<bool> g_qolDamageCapEnabled{ false };
static std::atomic<bool> g_qolEnemyHPEnabled{ false };
static std::atomic<bool> g_qolSBAEnabled{ false };
static std::atomic<bool> g_qolSessionEnabled{ false };
static std::atomic<bool> g_qolLevelSyncEnabled{ false };
static std::atomic<bool> g_qolReturnWrightstoneEnabled{ false };
static std::atomic<bool> g_qolFreeCaptainEnabled{ false };
static std::atomic<int> g_qolEnemyPrecision{ 2 };
static std::atomic<int> g_qolSBAPrecision{ 2 };
static thread_local bool g_qolEditingEnemy = false;
static thread_local bool g_qolEditingPlayer = false;
static std::atomic<LONG> g_qolLevelCaveCallbacks{ 0 };
static thread_local bool g_qolForcedNormalQuest = false;

static lm_address_t g_qolLevelSetTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolLevelConditionTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolLevelRewardTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolLevelConditionCave = LM_ADDRESS_BAD;
static lm_address_t g_qolLevelRewardCave = LM_ADDRESS_BAD;
static lm_byte_t g_qolLevelSetOriginal[7]{};
static lm_byte_t g_qolLevelConditionOriginal[6]{};
static lm_byte_t g_qolLevelRewardOriginal[6]{};
static bool g_qolLevelSyncInstalled = false;
static bool g_qolLevelSetPatched = false;
static bool g_qolLevelConditionPatched = false;
static bool g_qolLevelRewardPatched = false;

struct QOLPendulumData
{
    uint32_t skill1;
    uint32_t level1;
    uint32_t skill2;
    uint32_t level2;
    uint32_t skill3;
    uint32_t level3;
    uint32_t itemId;
};

using QOLGiveStoneFunction = void(*)(uint8_t*, uint32_t, bool);
using QOLBlacksmithDialogFunction = void(*)(uint8_t*);
using QOLGeneratePendulumFunction = void(*)(uint8_t*, QOLPendulumData*);
using QOLGiveItemFunction = void(*)(uint8_t*, uint32_t, uint32_t, bool);
static QOLGiveStoneFunction g_qolGiveStone = nullptr;
static QOLBlacksmithDialogFunction g_qolOriginalBlacksmithDialog = nullptr;
static QOLGeneratePendulumFunction g_qolOriginalGeneratePendulum = nullptr;
static QOLGiveItemFunction g_qolOriginalGiveItem = nullptr;
static lm_address_t g_qolBlacksmithDialogTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolGeneratePendulumTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolGiveItemTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolWeaponManagerGlobal = LM_ADDRESS_BAD;
static lm_address_t g_qolCurrentCharacterGlobal = LM_ADDRESS_BAD;
static lm_size_t g_qolBlacksmithDialogHookSize = 0;
static lm_size_t g_qolGeneratePendulumHookSize = 0;
static lm_size_t g_qolGiveItemHookSize = 0;
static lm_byte_t g_qolBlacksmithDialogOriginal[32]{};
static lm_byte_t g_qolGeneratePendulumOriginal[32]{};
static lm_byte_t g_qolGiveItemOriginal[32]{};
static std::atomic<uintptr_t> g_qolItemManager{ 0 };
static thread_local bool g_qolReturningStone = false;
static std::atomic<bool> g_qolWrightstoneTransactionArmed{ false };
static thread_local QOLPendulumData g_qolReturnedStone{};

using QOLFormationActionFunction = void(*)(uintptr_t);
using QOLFormationCallbackFunction = void(*)(uintptr_t);
using QOLFormationSelectionFunction = void(*)(uintptr_t, uintptr_t);
using QOLFormationReplacementFunction = int(*)(uintptr_t, int, uintptr_t);
static QOLFormationActionFunction g_qolOriginalApplyFormation = nullptr;
static QOLFormationCallbackFunction g_qolOriginalValidateRemoval = nullptr;
static QOLFormationCallbackFunction g_qolOriginalRemovalResult = nullptr;
static QOLFormationSelectionFunction g_qolOriginalSelectCharacter = nullptr;
static QOLFormationReplacementFunction g_qolOriginalValidateReplacement = nullptr;
static lm_address_t g_qolApplyFormationTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolValidateRemovalTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolRemovalResultTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolSelectCharacterTarget = LM_ADDRESS_BAD;
static lm_address_t g_qolValidateReplacementTarget = LM_ADDRESS_BAD;
static lm_size_t g_qolApplyFormationHookSize = 0;
static lm_size_t g_qolValidateRemovalHookSize = 0;
static lm_size_t g_qolRemovalResultHookSize = 0;
static lm_size_t g_qolSelectCharacterHookSize = 0;
static lm_size_t g_qolValidateReplacementHookSize = 0;
static lm_byte_t g_qolApplyFormationOriginal[32]{};
static lm_byte_t g_qolValidateRemovalOriginal[32]{};
static lm_byte_t g_qolRemovalResultOriginal[32]{};
static lm_byte_t g_qolSelectCharacterOriginal[32]{};
static lm_byte_t g_qolValidateReplacementOriginal[32]{};
static lm_address_t g_qolFormationOutputGlobal = LM_ADDRESS_BAD;
static lm_address_t g_qolPlayerDataGlobal = LM_ADDRESS_BAD;

static bool InstallQOLHook(lm_address_t target, lm_address_t detour, lm_address_t* original, lm_size_t* size, lm_byte_t* bytes, size_t capacity);

static bool InitializeQOLMapping()
{
    g_qolMapping = CreateFileMappingW(INVALID_HANDLE_VALUE, nullptr, PAGE_READWRITE, 0, sizeof(RuntimeQOLState), kRuntimeQOLMappingName);
    if (!g_qolMapping) return false;
    g_qolState = reinterpret_cast<RuntimeQOLState*>(MapViewOfFile(g_qolMapping, FILE_MAP_ALL_ACCESS, 0, 0, sizeof(RuntimeQOLState)));
    if (!g_qolState)
    {
        CloseHandle(g_qolMapping);
        g_qolMapping = nullptr;
        return false;
    }
    SecureZeroMemory(g_qolState, sizeof(RuntimeQOLState));
    g_qolState->magic = kRuntimeQOLMagic;
    g_qolState->version = kRuntimeQOLVersion;
    g_qolState->pid = GetCurrentProcessId();
    return true;
}

static void CloseQOLMapping()
{
    if (g_qolState)
    {
        UnmapViewOfFile(g_qolState);
        g_qolState = nullptr;
    }
    if (g_qolMapping)
    {
        CloseHandle(g_qolMapping);
        g_qolMapping = nullptr;
    }
}

static bool IsSessionId(const char* value, uint32_t length)
{
    if (!value || length != 19 || memcmp(value, "**** **** **** ****", 19) == 0) return false;
    for (uint32_t index = 0; index < length; ++index)
    {
        if (index == 4 || index == 9 || index == 14)
        {
            if (value[index] != ' ') return false;
            continue;
        }
        unsigned char character = static_cast<unsigned char>(value[index]);
        if (!std::isalnum(character)) return false;
    }
    return true;
}

static void CaptureSessionId(const QOLGameString* value)
{
    if (!g_qolSessionEnabled.load() || !g_qolState || !value || value->length > 31) return;
    __try
    {
        if (!IsSessionId(value->pointer, value->length)) return;
        char next[32]{};
        memcpy(next, value->pointer, value->length);
        AcquireSRWLockExclusive(&g_qolSessionLock);
        if (memcmp(g_qolState->latestSessionId, next, sizeof(next)) != 0)
        {
            InterlockedIncrement64(&g_qolState->sessionGeneration);
            memcpy(g_qolState->latestSessionId, next, sizeof(next));
            InterlockedIncrement64(&g_qolState->sessionSequence);
            MemoryBarrier();
            InterlockedIncrement64(&g_qolState->sessionGeneration);
        }
        ReleaseSRWLockExclusive(&g_qolSessionLock);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
}

static void QOLTextSetDetour(uintptr_t component, QOLGameString* value, uint32_t hash, int mode)
{
    g_qolCallbacks.fetch_add(1);
    if (!g_qolStopping.load()) CaptureSessionId(value);
    if (g_qolOriginalTextSet) g_qolOriginalTextSet(component, value, hash, mode);
    g_qolCallbacks.fetch_sub(1);
}

static void QOLDamageCapDetour(uint8_t* display, QOLDamageSource* source)
{
    g_qolCallbacks.fetch_add(1);
    bool eligible = false;
    if (!g_qolStopping.load() && g_qolDamageCapEnabled.load() && source && g_qolDamageCapFlagAddress != LM_ADDRESS_BAD && g_qolDamageCapQuestCheck != LM_ADDRESS_BAD)
    {
        __try
        {
            uintptr_t wrapper = source->source.wrapper;
            uintptr_t object = wrapper ? *reinterpret_cast<uintptr_t*>(wrapper + 0x50) : 0;
            int objectId = object ? *reinterpret_cast<int*>(object + 0x14) : 0;
            eligible = (objectId & 0xFF0000) == 0x10000;
        }
        __except (EXCEPTION_EXECUTE_HANDLER)
        {
            eligible = false;
        }
    }
    if (!eligible)
    {
        if (g_qolOriginalDamageCap) g_qolOriginalDamageCap(display, source);
        g_qolCallbacks.fetch_sub(1);
        return;
    }

    AcquireSRWLockExclusive(&g_qolDamageCapLock);
    volatile uint16_t* flag = reinterpret_cast<volatile uint16_t*>(g_qolDamageCapFlagAddress);
    bool hadPercent = false;
    __try
    {
        hadPercent = (*flag & 0x100) != 0;
        *flag |= 0x100;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
    if (g_qolOriginalDamageCap) g_qolOriginalDamageCap(display, source);
    __try
    {
        if (!hadPercent) *flag &= static_cast<uint16_t>(~0x100);
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
    ReleaseSRWLockExclusive(&g_qolDamageCapLock);
    g_qolCallbacks.fetch_sub(1);
}

static void QOLEnemyHealthDetour(uintptr_t controller, float delta)
{
    g_qolCallbacks.fetch_add(1);
    bool previous = g_qolEditingEnemy;
    g_qolEditingEnemy = !g_qolStopping.load() && g_qolEnemyHPEnabled.load();
    if (g_qolOriginalEnemyHealth) g_qolOriginalEnemyHealth(controller, delta);
    g_qolEditingEnemy = previous;
    g_qolCallbacks.fetch_sub(1);
}

static void QOLPlayerParamDetour(uintptr_t controller)
{
    g_qolCallbacks.fetch_add(1);
    bool previous = g_qolEditingPlayer;
    g_qolEditingPlayer = !g_qolStopping.load() && g_qolSBAEnabled.load();
    if (g_qolOriginalPlayerParam) g_qolOriginalPlayerParam(controller);
    g_qolEditingPlayer = previous;
    g_qolCallbacks.fetch_sub(1);
}

static void QOLSetTextFromIntDetour(uintptr_t component, int64_t number)
{
    g_qolCallbacks.fetch_add(1);
    bool interceptEnemy = g_qolEditingEnemy && g_qolEnemyHPEnabled.load();
    bool interceptSBA = g_qolEditingPlayer && g_qolSBAEnabled.load() && (static_cast<uint64_t>(number) >> 20) != 0;
    if (!g_qolStopping.load() && (interceptEnemy || interceptSBA) && g_qolOriginalTextSet)
    {
        uint32_t raw = static_cast<uint32_t>(number);
        float percentage = 0.0f;
        memcpy(&percentage, &raw, sizeof(percentage));
        if (std::isfinite(percentage) && percentage >= 0.0f && percentage <= 100.0f)
        {
            char buffer[32]{};
            int precision = interceptEnemy ? g_qolEnemyPrecision.load() : g_qolSBAPrecision.load();
            int length = snprintf(buffer, sizeof(buffer), "%.*f", precision, percentage * 100.0f);
            if (length > 0 && length < static_cast<int>(sizeof(buffer)))
            {
                QOLGameString value{ buffer, static_cast<uint32_t>(length), 0 };
                g_qolOriginalTextSet(component, &value, 0x887AE0B0, -1);
                g_qolCallbacks.fetch_sub(1);
                return;
            }
        }
    }
    if (g_qolOriginalSetTextFromInt) g_qolOriginalSetTextFromInt(component, number);
    g_qolCallbacks.fetch_sub(1);
}

static bool WriteQOLCode(lm_address_t cave, const std::vector<lm_byte_t>& code)
{
    return cave != LM_ADDRESS_BAD && !code.empty() && PatchBytes(cave, code.data(), code.size());
}

static bool AppendRelativeBranch(std::vector<lm_byte_t>& code, lm_address_t cave, lm_byte_t opcode, lm_address_t target)
{
    size_t instruction = code.size();
    code.push_back(opcode);
    code.resize(code.size() + 4);
    int64_t delta = static_cast<int64_t>(target) - static_cast<int64_t>(cave + instruction + 5);
    if (delta < INT32_MIN || delta > INT32_MAX) return false;
    int32_t relative = static_cast<int32_t>(delta);
    memcpy(code.data() + instruction + 1, &relative, sizeof(relative));
    return true;
}

static bool AppendRelativeJcc(std::vector<lm_byte_t>& code, lm_address_t cave, lm_byte_t condition, lm_address_t target)
{
    size_t instruction = code.size();
    code.push_back(0x0F);
    code.push_back(condition);
    code.resize(code.size() + 4);
    int64_t delta = static_cast<int64_t>(target) - static_cast<int64_t>(cave + instruction + 6);
    if (delta < INT32_MIN || delta > INT32_MAX) return false;
    int32_t relative = static_cast<int32_t>(delta);
    memcpy(code.data() + instruction + 2, &relative, sizeof(relative));
    return true;
}

static void AppendQOLImmediate64(std::vector<lm_byte_t>& code, uint64_t value)
{
    size_t offset = code.size();
    code.resize(offset + sizeof(value));
    memcpy(code.data() + offset, &value, sizeof(value));
}

static bool QOLSetForcedNormalQuest(uint32_t questId)
{
    g_qolForcedNormalQuest = !g_qolStopping.load() && g_qolLevelSyncEnabled.load() && questId != 0x00F00000;
    return g_qolForcedNormalQuest;
}

static bool QOLConsumeForcedNormalQuest()
{
    bool forced = !g_qolStopping.load() && g_qolLevelSyncEnabled.load() && g_qolForcedNormalQuest;
    g_qolForcedNormalQuest = false;
    return forced;
}

static bool InstallQOLLevelSync(const lm_module_t& module)
{
    const char* setSignature = "0F 94 83 ?? ?? ?? ?? 48 8B 35";
    const char* conditionSignature = "3B 05 ?? ?? ?? ?? 0F 94 87";
    const char* rewardSignature = "0F 84 ?? ?? ?? ?? 8B 06 C1 E8";
    g_qolLevelSetTarget = FindUniqueSignature(setSignature, module);
    g_qolLevelConditionTarget = FindUniqueSignature(conditionSignature, module);
    g_qolLevelRewardTarget = FindUniqueSignature(rewardSignature, module);
    if (g_qolLevelSetTarget == LM_ADDRESS_BAD || g_qolLevelConditionTarget == LM_ADDRESS_BAD || g_qolLevelRewardTarget == LM_ADDRESS_BAD ||
        LM_ReadMemory(g_qolLevelSetTarget, g_qolLevelSetOriginal, sizeof(g_qolLevelSetOriginal)) != sizeof(g_qolLevelSetOriginal) ||
        LM_ReadMemory(g_qolLevelConditionTarget, g_qolLevelConditionOriginal, sizeof(g_qolLevelConditionOriginal)) != sizeof(g_qolLevelConditionOriginal) ||
        LM_ReadMemory(g_qolLevelRewardTarget, g_qolLevelRewardOriginal, sizeof(g_qolLevelRewardOriginal)) != sizeof(g_qolLevelRewardOriginal) ||
        g_qolLevelSetOriginal[0] != 0x0F || g_qolLevelSetOriginal[1] != 0x94 || g_qolLevelSetOriginal[2] != 0x83 ||
        g_qolLevelConditionOriginal[0] != 0x3B || g_qolLevelConditionOriginal[1] != 0x05 ||
        g_qolLevelRewardOriginal[0] != 0x0F || g_qolLevelRewardOriginal[1] != 0x84) return false;

    lm_byte_t forceSet[7]{ 0xC6, 0x83, 0, 0, 0, 0, 1 };
    memcpy(forceSet + 2, g_qolLevelSetOriginal + 3, 4);
    if (!PatchBytes(g_qolLevelSetTarget, forceSet, sizeof(forceSet))) return false;
    g_qolLevelSetPatched = true;

    int32_t conditionDisplacement = 0;
    memcpy(&conditionDisplacement, g_qolLevelConditionOriginal + 2, sizeof(conditionDisplacement));
    lm_address_t quickQuestIdAddress = g_qolLevelConditionTarget + 6 + static_cast<intptr_t>(conditionDisplacement);
    g_qolLevelConditionCave = AllocNear(g_qolLevelConditionTarget, 128);
    if (g_qolLevelConditionCave == LM_ADDRESS_BAD) return false;
    std::vector<lm_byte_t> condition{ 0x41, 0x53, 0x49, 0xBB };
    AppendQOLImmediate64(condition, reinterpret_cast<uint64_t>(&g_qolLevelCaveCallbacks));
    condition.insert(condition.end(), {
        0xF0, 0x41, 0xFF, 0x03,
        0x41, 0x52, 0x41, 0x51, 0x41, 0x50, 0x52, 0x51, 0x50,
        0x8B, 0x0C, 0x24, 0x49, 0x89, 0xE3, 0x48, 0x83, 0xE4, 0xF0,
        0x48, 0x83, 0xEC, 0x30, 0x4C, 0x89, 0x5C, 0x24, 0x20, 0x48, 0xB8
    });
    AppendQOLImmediate64(condition, reinterpret_cast<uint64_t>(&QOLSetForcedNormalQuest));
    condition.insert(condition.end(), {
        0xFF, 0xD0, 0x48, 0x8B, 0x64, 0x24, 0x20,
        0x58, 0x59, 0x5A, 0x41, 0x58, 0x41, 0x59, 0x41, 0x5A,
        0x49, 0xBB
    });
    AppendQOLImmediate64(condition, reinterpret_cast<uint64_t>(&g_qolLevelCaveCallbacks));
    condition.insert(condition.end(), { 0xF0, 0x41, 0xFF, 0x0B, 0x41, 0x5B, 0x41, 0x53, 0x49, 0xBB });
    AppendQOLImmediate64(condition, static_cast<uint64_t>(quickQuestIdAddress));
    condition.insert(condition.end(), { 0x41, 0x3B, 0x03, 0x41, 0x5B });
    if (!AppendRelativeBranch(condition, g_qolLevelConditionCave, 0xE9, g_qolLevelConditionTarget + 6) || condition.size() > 128 ||
        !WriteQOLCode(g_qolLevelConditionCave, condition)) return false;
    lm_byte_t conditionJump[6]{ 0xE9, 0, 0, 0, 0, 0x90 };
    int64_t conditionDelta = static_cast<int64_t>(g_qolLevelConditionCave) - static_cast<int64_t>(g_qolLevelConditionTarget + 5);
    if (conditionDelta < INT32_MIN || conditionDelta > INT32_MAX) return false;
    int32_t conditionRelative = static_cast<int32_t>(conditionDelta);
    memcpy(conditionJump + 1, &conditionRelative, sizeof(conditionRelative));
    if (!PatchBytes(g_qolLevelConditionTarget, conditionJump, sizeof(conditionJump))) return false;
    g_qolLevelConditionPatched = true;

    int32_t rewardDisplacement = 0;
    memcpy(&rewardDisplacement, g_qolLevelRewardOriginal + 2, sizeof(rewardDisplacement));
    lm_address_t rewardTarget = g_qolLevelRewardTarget + 6 + static_cast<intptr_t>(rewardDisplacement);
    g_qolLevelRewardCave = AllocNear(g_qolLevelRewardTarget, 160);
    if (g_qolLevelRewardCave == LM_ADDRESS_BAD) return false;
    std::vector<lm_byte_t> reward{ 0x9C, 0x41, 0x53, 0x49, 0xBB };
    AppendQOLImmediate64(reward, reinterpret_cast<uint64_t>(&g_qolLevelCaveCallbacks));
    reward.insert(reward.end(), {
        0xF0, 0x41, 0xFF, 0x03,
        0x41, 0x52, 0x41, 0x51, 0x41, 0x50, 0x52, 0x51, 0x50,
        0x49, 0x89, 0xE3, 0x48, 0x83, 0xE4, 0xF0, 0x48, 0x83, 0xEC, 0x30,
        0x4C, 0x89, 0x5C, 0x24, 0x20, 0x48, 0xB8
    });
    AppendQOLImmediate64(reward, reinterpret_cast<uint64_t>(&QOLConsumeForcedNormalQuest));
    reward.insert(reward.end(), { 0xFF, 0xD0, 0x48, 0x8B, 0x64, 0x24, 0x20, 0x84, 0xC0, 0x75, 0x00 });
    size_t forcedDisplacement = reward.size() - 1;
    reward.insert(reward.end(), { 0x49, 0xBB });
    AppendQOLImmediate64(reward, reinterpret_cast<uint64_t>(&g_qolLevelCaveCallbacks));
    reward.insert(reward.end(), {
        0xF0, 0x41, 0xFF, 0x0B,
        0x58, 0x59, 0x5A, 0x41, 0x58, 0x41, 0x59, 0x41, 0x5A, 0x41, 0x5B, 0x9D
    });
    if (!AppendRelativeJcc(reward, g_qolLevelRewardCave, 0x84, rewardTarget) ||
        !AppendRelativeBranch(reward, g_qolLevelRewardCave, 0xE9, g_qolLevelRewardTarget + 6)) return false;
    size_t forcedLabel = reward.size();
    reward.insert(reward.end(), { 0x49, 0xBB });
    AppendQOLImmediate64(reward, reinterpret_cast<uint64_t>(&g_qolLevelCaveCallbacks));
    reward.insert(reward.end(), {
        0xF0, 0x41, 0xFF, 0x0B,
        0x58, 0x59, 0x5A, 0x41, 0x58, 0x41, 0x59, 0x41, 0x5A, 0x41, 0x5B,
        0x48, 0x8D, 0x64, 0x24, 0x08
    });
    if (!AppendRelativeBranch(reward, g_qolLevelRewardCave, 0xE9, rewardTarget)) return false;
    ptrdiff_t shortDelta = static_cast<ptrdiff_t>(forcedLabel) - static_cast<ptrdiff_t>(forcedDisplacement + 1);
    if (shortDelta < INT8_MIN || shortDelta > INT8_MAX) return false;
    reward[forcedDisplacement] = static_cast<lm_byte_t>(static_cast<int8_t>(shortDelta));
    if (reward.size() > 160 || !WriteQOLCode(g_qolLevelRewardCave, reward)) return false;
    lm_byte_t rewardJump[6]{ 0xE9, 0, 0, 0, 0, 0x90 };
    int64_t rewardDelta = static_cast<int64_t>(g_qolLevelRewardCave) - static_cast<int64_t>(g_qolLevelRewardTarget + 5);
    if (rewardDelta < INT32_MIN || rewardDelta > INT32_MAX) return false;
    int32_t rewardRelative = static_cast<int32_t>(rewardDelta);
    memcpy(rewardJump + 1, &rewardRelative, sizeof(rewardRelative));
    if (!PatchBytes(g_qolLevelRewardTarget, rewardJump, sizeof(rewardJump))) return false;
    g_qolLevelRewardPatched = true;
    g_qolLevelSyncInstalled = true;
    g_qolForcedNormalQuest = false;
    return true;
}

static void QOLGiveItemDetour(uint8_t* manager, uint32_t itemId, uint32_t count, bool flag)
{
    g_qolCallbacks.fetch_add(1);
    if (!g_qolStopping.load() && manager) g_qolItemManager.store(reinterpret_cast<uintptr_t>(manager));
    if (g_qolOriginalGiveItem) g_qolOriginalGiveItem(manager, itemId, count, flag);
    g_qolCallbacks.fetch_sub(1);
}

static void QOLGeneratePendulumDetour(uint8_t* generator, QOLPendulumData* output)
{
    g_qolCallbacks.fetch_add(1);
    if (!g_qolStopping.load() && g_qolReturningStone && output)
    {
        *output = g_qolReturnedStone;
    }
    else if (g_qolOriginalGeneratePendulum)
    {
        g_qolOriginalGeneratePendulum(generator, output);
    }
    g_qolCallbacks.fetch_sub(1);
}

static void QOLBlacksmithDialogDetour(uint8_t* dialog)
{
    g_qolCallbacks.fetch_add(1);
    QOLPendulumData snapshot{};
    uintptr_t recordAddress = 0;
    int32_t weaponSlot = -1;
    bool found = false;
    if (!g_qolStopping.load() && g_qolReturnWrightstoneEnabled.load() && dialog)
    {
        __try
        {
            const bool performTransaction = *(dialog + 0x240) != 0;
            if (!performTransaction)
            {
                g_qolWrightstoneTransactionArmed.store(true);
            }
            else if (g_qolWrightstoneTransactionArmed.exchange(false))
            {
                if (g_qolWeaponManagerGlobal == LM_ADDRESS_BAD || g_qolCurrentCharacterGlobal == LM_ADDRESS_BAD) __leave;
                uintptr_t currentState = *reinterpret_cast<uintptr_t*>(g_qolCurrentCharacterGlobal);
                uintptr_t weaponManager = *reinterpret_cast<uintptr_t*>(g_qolWeaponManagerGlobal);
                weaponSlot = currentState ? *reinterpret_cast<int32_t*>(currentState + 0xD4) : -1;
                if (weaponManager && weaponSlot >= 0)
                {
                    for (int character = 0; character < 32 && !found; ++character)
                    {
                        for (int weapon = 0; weapon < 8; ++weapon)
                        {
                            uintptr_t record = weaponManager + 0x370 + static_cast<uintptr_t>(character) * 0x680 + static_cast<uintptr_t>(weapon) * 0xD0;
                            if (*reinterpret_cast<int32_t*>(record) != weaponSlot) continue;
                            snapshot = *reinterpret_cast<QOLPendulumData*>(record + 0x20);
                            found = snapshot.itemId != 0 && snapshot.itemId != 0x887AE0B0;
                            if (found) recordAddress = record;
                            break;
                        }
                    }
                }
            }
        }
        __except (EXCEPTION_EXECUTE_HANDLER)
        {
            found = false;
        }
    }
    if (g_qolOriginalBlacksmithDialog) g_qolOriginalBlacksmithDialog(dialog);
    if (found)
    {
        __try
        {
            const auto* record = reinterpret_cast<const uint8_t*>(recordAddress);
            found = record && *reinterpret_cast<const int32_t*>(record) == weaponSlot;
        }
        __except (EXCEPTION_EXECUTE_HANDLER)
        {
            found = false;
        }
    }
    uintptr_t manager = g_qolItemManager.load();
    if (found && manager && g_qolGiveStone)
    {
        g_qolReturnedStone = snapshot;
        __try
        {
            g_qolReturningStone = true;
            g_qolGiveStone(reinterpret_cast<uint8_t*>(manager), snapshot.itemId, false);
        }
        __finally
        {
            g_qolReturningStone = false;
        }
    }
    g_qolCallbacks.fetch_sub(1);
}

static lm_address_t ResolveRIPGlobal(lm_address_t instruction, size_t displacementOffset, size_t instructionSize)
{
    if (instruction == LM_ADDRESS_BAD) return LM_ADDRESS_BAD;
    int32_t displacement = 0;
    if (LM_ReadMemory(instruction + displacementOffset, reinterpret_cast<lm_byte_t*>(&displacement), sizeof(displacement)) != sizeof(displacement)) return LM_ADDRESS_BAD;
    return instruction + instructionSize + static_cast<intptr_t>(displacement);
}

static bool InstallQOLReturnWrightstone(const lm_module_t& module)
{
    const char* giveStoneSignature = "55 41 57 41 56 41 55 41 54 56 57 53 48 81 EC ?? ?? ?? ?? 48 8D AC 24 ?? ?? ?? ?? 48 C7 45 ?? ?? ?? ?? ?? 45 89 C6 89 D6";
    const char* dialogSignature = "55 41 57 41 56 41 55 41 54 56 57 53 48 81 EC ?? ?? ?? ?? 48 8D AC 24 ?? ?? ?? ?? 48 C7 85 ?? ?? ?? ?? ?? ?? ?? ?? 48 89 CE 48 8B 05 ?? ?? ?? ?? 8B B8";
    const char* generateSignature = "55 41 57 41 56 41 55 41 54 56 57 53 48 83 EC ?? 48 8D 6C 24 ?? 48 C7 45 ?? ?? ?? ?? ?? 48 89 D6 8B 52";
    const char* giveItemSignature = "55 41 57 41 56 41 55 41 54 56 57 53 48 81 EC ?? ?? ?? ?? 48 8D AC 24 ?? ?? ?? ?? 48 C7 85 ?? ?? ?? ?? ?? ?? ?? ?? 45 85 C0 0F 84";
    const char* weaponGlobalSignature = "48 89 35 ?? ?? ?? ?? 48 89 F1 31 D2 E8 ?? ?? ?? ?? C5 78 29 56";
    const char* characterGlobalSignature = "48 89 05 ?? ?? ?? ?? 48 B8 ?? ?? ?? ?? ?? ?? ?? ?? 48 89 06";
    lm_address_t giveStone = FindUniqueSignature(giveStoneSignature, module);
    g_qolBlacksmithDialogTarget = FindUniqueSignature(dialogSignature, module);
    g_qolGeneratePendulumTarget = FindUniqueSignature(generateSignature, module);
    g_qolGiveItemTarget = FindUniqueSignature(giveItemSignature, module);
    g_qolWeaponManagerGlobal = ResolveRIPGlobal(FindUniqueSignature(weaponGlobalSignature, module), 3, 7);
    g_qolCurrentCharacterGlobal = ResolveRIPGlobal(FindUniqueSignature(characterGlobalSignature, module), 3, 7);
    if (giveStone == LM_ADDRESS_BAD || g_qolWeaponManagerGlobal == LM_ADDRESS_BAD || g_qolCurrentCharacterGlobal == LM_ADDRESS_BAD) return false;
    g_qolGiveStone = reinterpret_cast<QOLGiveStoneFunction>(giveStone);
    if (!InstallQOLHook(g_qolGiveItemTarget, reinterpret_cast<lm_address_t>(&QOLGiveItemDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalGiveItem),
        &g_qolGiveItemHookSize, g_qolGiveItemOriginal, sizeof(g_qolGiveItemOriginal))) return false;
    if (!InstallQOLHook(g_qolGeneratePendulumTarget, reinterpret_cast<lm_address_t>(&QOLGeneratePendulumDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalGeneratePendulum),
        &g_qolGeneratePendulumHookSize, g_qolGeneratePendulumOriginal, sizeof(g_qolGeneratePendulumOriginal))) return false;
    if (!InstallQOLHook(g_qolBlacksmithDialogTarget, reinterpret_cast<lm_address_t>(&QOLBlacksmithDialogDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalBlacksmithDialog),
        &g_qolBlacksmithDialogHookSize, g_qolBlacksmithDialogOriginal, sizeof(g_qolBlacksmithDialogOriginal))) return false;
    return true;
}

static uint32_t QOLRotateBits(uint32_t value, int count)
{
    return (value << count) | (value >> (32 - count));
}

static uint32_t QOLFormationHash(const uint8_t* input, size_t length)
{
    constexpr uint32_t p1 = 0x9E3779B1, p2 = 0x85EBCA77, p3 = 0xC2B2AE3D, p4 = 0x27D4EB2F, p5 = 0x165667B1;
    auto read32 = [](const uint8_t* value) {
        uint32_t output = 0;
        memcpy(&output, value, sizeof(output));
        return output;
    };
    auto round = [&](uint32_t seed, uint32_t value) { return QOLRotateBits(seed + value * p2, 13) * p1; };
    const uint8_t* cursor = input;
    size_t remaining = length;
    uint32_t hash = 0x178A54A4;
    if (remaining >= 16)
    {
        uint32_t v1 = 0x2557311B, v2 = 0x871FB76A, v3 = 0x0133ECF3, v4 = 0x62FC7342;
        do
        {
            v1 = round(v1, read32(cursor));
            v2 = round(v2, read32(cursor + 4));
            v3 = round(v3, read32(cursor + 8));
            v4 = round(v4, read32(cursor + 12));
            cursor += 16;
            remaining -= 16;
        } while (remaining > 16);
        hash = QOLRotateBits(v1, 1) + QOLRotateBits(v2, 7) + QOLRotateBits(v3, 12) + QOLRotateBits(v4, 18);
    }
    hash += static_cast<uint32_t>(length);
    while (remaining >= 4)
    {
        hash = QOLRotateBits(hash + read32(cursor) * p3, 17) * p4;
        cursor += 4;
        remaining -= 4;
    }
    while (remaining)
    {
        hash = QOLRotateBits(hash + *cursor * p5, 11) * p1;
        ++cursor;
        --remaining;
    }
    hash ^= hash >> 15; hash *= p2;
    hash ^= hash >> 13; hash *= p3;
    hash ^= hash >> 16;
    return hash;
}

static uint32_t QOLCurrentFormationKey()
{
    if (g_qolFormationOutputGlobal == LM_ADDRESS_BAD) return 0x887AE0B0;
    __try
    {
        uintptr_t output = *reinterpret_cast<uintptr_t*>(g_qolFormationOutputGlobal);
        return output ? *reinterpret_cast<uint32_t*>(output) : 0x887AE0B0;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return 0x887AE0B0;
    }
}

static bool QOLReadFormationAction(uintptr_t action, uintptr_t* data, int64_t* length, uint32_t* key)
{
    if (!action || !data || !length || !key) return false;
    __try
    {
        int64_t size = *reinterpret_cast<int64_t*>(action + 0x40);
        int64_t capacity = *reinterpret_cast<int64_t*>(action + 0x48);
        if (size < 0 || size > 0x1000 || capacity < 0 || size > capacity) return false;
        uintptr_t pointer = capacity < 0x10 ? action + 0x30 : *reinterpret_cast<uintptr_t*>(action + 0x30);
        if (!pointer) return false;
        *data = pointer;
        *length = size;
        *key = QOLFormationHash(reinterpret_cast<const uint8_t*>(pointer), static_cast<size_t>(size));
        return true;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

struct QOLTemporaryByte
{
    uintptr_t address;
    uint8_t value;
    bool active;
};

static QOLTemporaryByte QOLClearByte(uintptr_t address)
{
    QOLTemporaryByte result{};
    if (!address) return result;
    __try
    {
        uint8_t value = *reinterpret_cast<uint8_t*>(address);
        if (!value) return result;
        *reinterpret_cast<uint8_t*>(address) = 0;
        result = { address, value, true };
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
    }
    return result;
}

static void QOLRestoreByte(QOLTemporaryByte value)
{
    if (!value.active) return;
    __try { *reinterpret_cast<uint8_t*>(value.address) = value.value; }
    __except (EXCEPTION_EXECUTE_HANDLER) {}
}

static uintptr_t QOLPlayerData()
{
    if (g_qolPlayerDataGlobal == LM_ADDRESS_BAD) return 0;
    __try { return *reinterpret_cast<uintptr_t*>(g_qolPlayerDataGlobal); }
    __except (EXCEPTION_EXECUTE_HANDLER) { return 0; }
}

static QOLTemporaryByte QOLClearMandatoryCharacter(int character)
{
    uintptr_t playerData = QOLPlayerData();
    if (!playerData || character < 0 || character > 1024) return {};
    return QOLClearByte(playerData + static_cast<uintptr_t>(character) * 0x10 + 5);
}

static void QOLApplyFormationDetour(uintptr_t action)
{
    g_qolCallbacks.fetch_add(1);
    uintptr_t data = 0;
    int64_t length = 0;
    uint32_t key = 0;
    bool altered = false;
    uint8_t first = 0;
    if (!g_qolStopping.load() && g_qolFreeCaptainEnabled.load() && QOLReadFormationAction(action, &data, &length, &key) && key == 0x4FA6A50B)
    {
        __try
        {
            first = *reinterpret_cast<uint8_t*>(data);
            *reinterpret_cast<uint8_t*>(data) = 0;
            *reinterpret_cast<int64_t*>(action + 0x40) = 0;
            altered = true;
        }
        __except (EXCEPTION_EXECUTE_HANDLER)
        {
            altered = false;
        }
    }
    __try
    {
        if (g_qolOriginalApplyFormation) g_qolOriginalApplyFormation(action);
    }
    __finally
    {
        if (altered)
        {
            __try
            {
                *reinterpret_cast<uint8_t*>(data) = first;
                *reinterpret_cast<int64_t*>(action + 0x40) = length;
            }
            __except (EXCEPTION_EXECUTE_HANDLER) {}
        }
        g_qolCallbacks.fetch_sub(1);
    }
}

static void QOLValidateRemovalDetour(uintptr_t callback)
{
    g_qolCallbacks.fetch_add(1);
    QOLTemporaryByte mandatory{};
    uintptr_t controller = 0;
    uint32_t formation = QOLCurrentFormationKey();
    if (!g_qolStopping.load() && g_qolFreeCaptainEnabled.load() && formation == 0x4FA6A50B && callback)
    {
        __try
        {
            controller = *reinterpret_cast<uintptr_t*>(callback);
            uintptr_t party = controller ? *reinterpret_cast<uintptr_t*>(controller + 0x1B0) : 0;
            int current = party ? *reinterpret_cast<int*>(party + 0x2E4) : -1;
            mandatory = QOLClearMandatoryCharacter(current);
        }
        __except (EXCEPTION_EXECUTE_HANDLER) {}
    }
    __try
    {
        if (g_qolOriginalValidateRemoval) g_qolOriginalValidateRemoval(callback);
        if (formation == 0x4FA6A50B && controller && *reinterpret_cast<uint32_t*>(controller + 0x268) == 0xB9544AA2)
            *reinterpret_cast<uint32_t*>(controller + 0x268) = 0x887AE0B0;
    }
    __finally
    {
        QOLRestoreByte(mandatory);
        g_qolCallbacks.fetch_sub(1);
    }
}

static void QOLRemovalResultDetour(uintptr_t callback)
{
    g_qolCallbacks.fetch_add(1);
    QOLTemporaryByte mandatory{};
    if (!g_qolStopping.load() && g_qolFreeCaptainEnabled.load() && QOLCurrentFormationKey() == 0x4FA6A50B && callback)
    {
        __try
        {
            uintptr_t controller = *reinterpret_cast<uintptr_t*>(callback);
            uintptr_t party = controller ? *reinterpret_cast<uintptr_t*>(controller + 0x1B0) : 0;
            if (controller && party && *reinterpret_cast<uint32_t*>(controller + 0x268) == 0xB9544AA2)
            {
                int previous = *reinterpret_cast<int*>(controller + 0x260);
                int current = *reinterpret_cast<int*>(party + 0x2E4);
                *reinterpret_cast<uint32_t*>(controller + 0x268) = 0x887AE0B0;
                mandatory = QOLClearMandatoryCharacter(current >= 0 ? current : previous);
            }
        }
        __except (EXCEPTION_EXECUTE_HANDLER) {}
    }
    __try { if (g_qolOriginalRemovalResult) g_qolOriginalRemovalResult(callback); }
    __finally
    {
        QOLRestoreByte(mandatory);
        g_qolCallbacks.fetch_sub(1);
    }
}

static void QOLSelectCharacterDetour(uintptr_t callback, uintptr_t selection)
{
    g_qolCallbacks.fetch_add(1);
    QOLTemporaryByte mandatory{}, lock{};
    if (!g_qolStopping.load() && g_qolFreeCaptainEnabled.load() && QOLCurrentFormationKey() == 0x4FA6A50B && callback && selection)
    {
        __try
        {
            int selected = *reinterpret_cast<int*>(selection);
            uintptr_t controller = *reinterpret_cast<uintptr_t*>(callback);
            uintptr_t party = controller ? *reinterpret_cast<uintptr_t*>(controller + 0x1B0) : 0;
            int previous = controller ? *reinterpret_cast<int*>(controller + 0x260) : -1;
            int current = party ? *reinterpret_cast<int*>(party + 0x2E4) : -1;
            mandatory = QOLClearMandatoryCharacter(current >= 0 ? current : selected);
            uintptr_t playerData = QOLPlayerData();
            if (playerData && selected >= 0 && selected <= 1024 && (selected == 0 || previous == 0 || current == 0))
                lock = QOLClearByte(playerData + static_cast<uintptr_t>(selected) * 0x10 + 4);
        }
        __except (EXCEPTION_EXECUTE_HANDLER) {}
    }
    __try { if (g_qolOriginalSelectCharacter) g_qolOriginalSelectCharacter(callback, selection); }
    __finally
    {
        QOLRestoreByte(lock);
        QOLRestoreByte(mandatory);
        g_qolCallbacks.fetch_sub(1);
    }
}

static int QOLValidateReplacementDetour(uintptr_t party, int selected, uintptr_t output)
{
    g_qolCallbacks.fetch_add(1);
    int result = g_qolOriginalValidateReplacement ? g_qolOriginalValidateReplacement(party, selected, output) : 0;
    if (!g_qolStopping.load() && g_qolFreeCaptainEnabled.load() && QOLCurrentFormationKey() == 0x4FA6A50B && result == 3) result = 0;
    g_qolCallbacks.fetch_sub(1);
    return result;
}

static bool InstallQOLFreeCaptain(const lm_module_t& module)
{
    struct Layout
    {
        lm_address_t applyFormationRVA;
        lm_address_t validateRemovalRVA;
        lm_address_t removalResultRVA;
        lm_address_t selectCharacterRVA;
        lm_address_t validateReplacementRVA;
        lm_address_t formationOutputGlobalRVA;
        lm_address_t playerDataGlobalRVA;
    };
    static constexpr Layout layouts[] = {
        { 0x1CA7870, 0x3F105D0, 0x3F10410, 0x3F10240, 0x41E3F70, 0x7C24980, 0x7C23878 },
        { 0x1CA16D0, 0x3F0C500, 0x3F0C340, 0x3F0C170, 0x41DFEA0, 0x7C21940, 0x7C20838 },
    };
    struct Entry { const char* signature; lm_address_t* target; lm_address_t detour; lm_address_t* original; lm_size_t* size; lm_byte_t* bytes; };
    Entry entries[] = {
        { "56 57 48 83 EC 28 48 89 CE 48 83 79 48 10 72 06 48 8B 76 30 EB 04 48 83 C6 30 48 89 F1 E8 ?? ?? ?? ?? 48 89 F1 48 89 C2 E8 ?? ?? ?? ?? 48 8B 0D ?? ?? ?? ?? 89 01 3D 0B A5 A6 4F", &g_qolApplyFormationTarget, reinterpret_cast<lm_address_t>(&QOLApplyFormationDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalApplyFormation), &g_qolApplyFormationHookSize, g_qolApplyFormationOriginal },
        { "41 56 56 57 55 53 48 83 EC 30 48 8B 39 48 8B 0D ?? ?? ?? ?? E8 ?? ?? ?? ?? 85 C0 7E ?? 48 83 C4 30 5B 5D 5F 5E 41 5E C3 C7 87 68 02 00 00 B0 E0 7A 88", &g_qolValidateRemovalTarget, reinterpret_cast<lm_address_t>(&QOLValidateRemovalDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalValidateRemoval), &g_qolValidateRemovalHookSize, g_qolValidateRemovalOriginal },
        { "41 57 41 56 56 57 53 48 83 EC 70 48 8B 19 48 8B 0D ?? ?? ?? ?? E8 ?? ?? ?? ?? 85 C0 0F 8F ?? ?? ?? ?? 48 8B 35 ?? ?? ?? ?? 48 8B 05 ?? ?? ?? ?? 8B 88 58 0D 00 00", &g_qolRemovalResultTarget, reinterpret_cast<lm_address_t>(&QOLRemovalResultDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalRemovalResult), &g_qolRemovalResultHookSize, g_qolRemovalResultOriginal },
        { "56 57 53 48 83 EC 70 48 8B 39 48 83 BF B0 01 00 00 00 74 4B 8B 02 48 63 C8 48 8B 15 ?? ?? ?? ?? 48 C1 E1 04 80 7C 0A 04 00", &g_qolSelectCharacterTarget, reinterpret_cast<lm_address_t>(&QOLSelectCharacterDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalSelectCharacter), &g_qolSelectCharacterHookSize, g_qolSelectCharacterOriginal },
        { "41 57 41 56 56 57 53 48 83 EC 50 4C 89 C7 85 D2 78 ?? 48 89 CE 41 89 D7 48 8B 81 48 03 00 00 48 8B 89 50 03 00 00 48 29 C1 48 C1 F9 02", &g_qolValidateReplacementTarget, reinterpret_cast<lm_address_t>(&QOLValidateReplacementDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalValidateReplacement), &g_qolValidateReplacementHookSize, g_qolValidateReplacementOriginal },
    };
    lm_address_t found[std::size(entries)]{};
    for (size_t index = 0; index < std::size(entries); ++index)
    {
        found[index] = FindUniqueSignature(entries[index].signature, module);
        if (found[index] == LM_ADDRESS_BAD) return false;
    }
    const Layout* selected = nullptr;
    for (const auto& candidate : layouts)
    {
        const lm_address_t expected[] = {
            module.base + candidate.applyFormationRVA,
            module.base + candidate.validateRemovalRVA,
            module.base + candidate.removalResultRVA,
            module.base + candidate.selectCharacterRVA,
            module.base + candidate.validateReplacementRVA,
        };
        bool matches = true;
        for (size_t index = 0; index < std::size(entries); ++index) matches = matches && found[index] == expected[index];
        if (!matches) continue;
        if (selected != nullptr) return false;
        selected = &candidate;
    }
    if (selected == nullptr) return false;
    for (auto& entry : entries)
    {
        const size_t index = static_cast<size_t>(&entry - entries);
        *entry.target = found[index];
        if (!InstallQOLHook(found[index], entry.detour, entry.original, entry.size, entry.bytes, 32)) return false;
    }
    g_qolFormationOutputGlobal = module.base + selected->formationOutputGlobalRVA;
    g_qolPlayerDataGlobal = module.base + selected->playerDataGlobalRVA;
    return QOLFormationHash(nullptr, 0) == 0x887AE0B0;
}

static bool ReadQOLConfig(const wchar_t* path, bool requireEnabled)
{
    bool enabled = GetPrivateProfileIntW(L"qol", L"enabled", 0, path) == 1;
    if (requireEnabled && !enabled) return false;
    int enemyPrecision = GetPrivateProfileIntW(L"qol", L"enemyHpPrecision", 2, path);
    int sbaPrecision = GetPrivateProfileIntW(L"qol", L"sbaPrecision", 2, path);
    if (enemyPrecision < 0 || enemyPrecision > 4 || sbaPrecision < 0 || sbaPrecision > 4) return false;
    g_qolDamageCapEnabled.store(GetPrivateProfileIntW(L"qol", L"damageCapPercentage", 0, path) == 1);
    g_qolEnemyHPEnabled.store(GetPrivateProfileIntW(L"qol", L"detailedEnemyHp", 0, path) == 1);
    g_qolSBAEnabled.store(GetPrivateProfileIntW(L"qol", L"detailedSba", 0, path) == 1);
    g_qolSessionEnabled.store(GetPrivateProfileIntW(L"qol", L"sessionCapture", 0, path) == 1);
    g_qolLevelSyncEnabled.store(GetPrivateProfileIntW(L"qol", L"normalQuestLevelSync", 0, path) == 1);
    g_qolReturnWrightstoneEnabled.store(GetPrivateProfileIntW(L"qol", L"returnWrightstone", 0, path) == 1);
    g_qolFreeCaptainEnabled.store(GetPrivateProfileIntW(L"qol", L"freeCaptain", 0, path) == 1);
    g_qolEnemyPrecision.store(enemyPrecision);
    g_qolSBAPrecision.store(sbaPrecision);
    return enabled && (g_qolDamageCapEnabled.load() || g_qolEnemyHPEnabled.load() || g_qolSBAEnabled.load() || g_qolSessionEnabled.load() ||
        g_qolLevelSyncEnabled.load() || g_qolReturnWrightstoneEnabled.load() || g_qolFreeCaptainEnabled.load());
}

static bool QOLPercentageInstructionsValid(const lm_byte_t* enemy, const lm_byte_t* sba)
{
    const lm_byte_t enemyTail[] = { 0xC4, 0xE3, 0x79, 0x0A, 0xC0, 0x0A, 0xC5, 0xFA, 0x2C, 0xD0 };
    const lm_byte_t sbaTail[] = { 0xC5, 0xFA, 0x2C, 0xD0 };
    return enemy && sba && enemy[0] == 0xC5 && enemy[1] == 0xCA && enemy[2] == 0x59 && enemy[3] == 0x05 &&
        memcmp(enemy + 8, enemyTail, sizeof(enemyTail)) == 0 && sba[0] == 0xC5 && sba[1] == 0xCA && sba[2] == 0x59 && sba[3] == 0x05 &&
        memcmp(sba + 8, sbaTail, sizeof(sbaTail)) == 0;
}

struct QOLHookStopEntry
{
    lm_address_t target;
    lm_address_t trampoline;
    lm_size_t* hookSize;
    const lm_byte_t* original;
    size_t originalCapacity;
};

static bool RestoreQOLHookEntry(const QOLHookStopEntry& entry)
{
    if (!entry.hookSize || !*entry.hookSize) return true;
    const lm_size_t size = *entry.hookSize;
    return entry.target != LM_ADDRESS_BAD && size <= entry.originalCapacity &&
        PatchBytesWhileSuspended(entry.target, entry.original, size);
}

static bool ReleaseQOLHook(const QOLHookStopEntry& entry)
{
    if (!entry.hookSize || !*entry.hookSize) return true;
    const lm_size_t size = *entry.hookSize;
    if (!LM_UnhookCode(entry.target, entry.trampoline, size)) return false;
    std::vector<lm_byte_t> verified(size);
    if (LM_ReadMemory(entry.target, verified.data(), size) != size || memcmp(verified.data(), entry.original, size) != 0) return false;
    *entry.hookSize = 0;
    return true;
}

static bool CurrentModuleRange(std::array<std::pair<lm_address_t, lm_size_t>, 1>* ranges)
{
    if (!ranges || !g_patchCoreModule) return false;
    __try
    {
        const auto* base = reinterpret_cast<const uint8_t*>(g_patchCoreModule);
        const auto* dos = reinterpret_cast<const IMAGE_DOS_HEADER*>(base);
        if (dos->e_magic != IMAGE_DOS_SIGNATURE || dos->e_lfanew <= 0) return false;
        const auto* nt = reinterpret_cast<const IMAGE_NT_HEADERS*>(base + dos->e_lfanew);
        if (nt->Signature != IMAGE_NT_SIGNATURE || !nt->OptionalHeader.SizeOfImage) return false;
        (*ranges)[0] = std::make_pair(reinterpret_cast<lm_address_t>(base), static_cast<lm_size_t>(nt->OptionalHeader.SizeOfImage));
        return true;
    }
    __except (EXCEPTION_EXECUTE_HANDLER)
    {
        return false;
    }
}

static bool ReleaseQOLHooksAfterDrain(const std::array<QOLHookStopEntry, 13>& hooks)
{
    std::array<std::pair<lm_address_t, lm_size_t>, 1> moduleRange{};
    if (!CurrentModuleRange(&moduleRange)) return false;
    const DWORD deadline = GetTickCount() + 5000;
    while (static_cast<LONG>(GetTickCount() - deadline) < 0)
    {
        bool released = false;
        {
            ScopedOtherThreadSuspension suspension;
            if (suspension.Active() && g_qolCallbacks.load() == 0 && suspension.InstructionPointersOutside(moduleRange))
            {
                released = true;
                for (const auto& hook : hooks) released = ReleaseQOLHook(hook) && released;
            }
        }
        if (released) return true;
        Sleep(1);
    }
    return false;
}

static bool RetireQOLLevelCaves()
{
    const std::array<std::pair<lm_address_t, lm_size_t>, 2> caves{
        std::make_pair(g_qolLevelConditionCave, static_cast<lm_size_t>(128)),
        std::make_pair(g_qolLevelRewardCave, static_cast<lm_size_t>(160)),
    };
    if (caves[0].first == LM_ADDRESS_BAD && caves[1].first == LM_ADDRESS_BAD) return true;

    const DWORD deadline = GetTickCount() + 5000;
    while (static_cast<LONG>(GetTickCount() - deadline) < 0)
    {
        bool retired = false;
        {
            ScopedOtherThreadSuspension suspension;
            if (suspension.Active() && g_qolLevelCaveCallbacks.load() == 0 && suspension.InstructionPointersOutside(caves))
            {
                retired = true;
                if (g_qolLevelConditionCave != LM_ADDRESS_BAD)
                {
                    retired = VirtualFree(reinterpret_cast<void*>(g_qolLevelConditionCave), 0, MEM_RELEASE) != FALSE;
                    if (retired) g_qolLevelConditionCave = LM_ADDRESS_BAD;
                }
                if (retired && g_qolLevelRewardCave != LM_ADDRESS_BAD)
                {
                    retired = VirtualFree(reinterpret_cast<void*>(g_qolLevelRewardCave), 0, MEM_RELEASE) != FALSE;
                    if (retired) g_qolLevelRewardCave = LM_ADDRESS_BAD;
                }
            }
        }
        if (retired) return true;
        Sleep(1);
    }
    return false;
}

static bool StopQOLRuntime()
{
    g_qolStopping.store(true);
    const std::array<QOLHookStopEntry, 13> hooks{{
        { g_qolDamageCapTarget, reinterpret_cast<lm_address_t>(g_qolOriginalDamageCap), &g_qolDamageCapHookSize, g_qolDamageCapOriginal, sizeof(g_qolDamageCapOriginal) },
        { g_qolEnemyHealthTarget, reinterpret_cast<lm_address_t>(g_qolOriginalEnemyHealth), &g_qolEnemyHealthHookSize, g_qolEnemyHealthOriginal, sizeof(g_qolEnemyHealthOriginal) },
        { g_qolPlayerParamTarget, reinterpret_cast<lm_address_t>(g_qolOriginalPlayerParam), &g_qolPlayerParamHookSize, g_qolPlayerParamOriginal, sizeof(g_qolPlayerParamOriginal) },
        { g_qolSetTextFromIntTarget, reinterpret_cast<lm_address_t>(g_qolOriginalSetTextFromInt), &g_qolSetTextFromIntHookSize, g_qolSetTextFromIntOriginal, sizeof(g_qolSetTextFromIntOriginal) },
        { g_qolTextSetTarget, reinterpret_cast<lm_address_t>(g_qolOriginalTextSet), &g_qolTextSetHookSize, g_qolTextSetOriginal, sizeof(g_qolTextSetOriginal) },
        { g_qolBlacksmithDialogTarget, reinterpret_cast<lm_address_t>(g_qolOriginalBlacksmithDialog), &g_qolBlacksmithDialogHookSize, g_qolBlacksmithDialogOriginal, sizeof(g_qolBlacksmithDialogOriginal) },
        { g_qolGeneratePendulumTarget, reinterpret_cast<lm_address_t>(g_qolOriginalGeneratePendulum), &g_qolGeneratePendulumHookSize, g_qolGeneratePendulumOriginal, sizeof(g_qolGeneratePendulumOriginal) },
        { g_qolGiveItemTarget, reinterpret_cast<lm_address_t>(g_qolOriginalGiveItem), &g_qolGiveItemHookSize, g_qolGiveItemOriginal, sizeof(g_qolGiveItemOriginal) },
        { g_qolValidateReplacementTarget, reinterpret_cast<lm_address_t>(g_qolOriginalValidateReplacement), &g_qolValidateReplacementHookSize, g_qolValidateReplacementOriginal, sizeof(g_qolValidateReplacementOriginal) },
        { g_qolSelectCharacterTarget, reinterpret_cast<lm_address_t>(g_qolOriginalSelectCharacter), &g_qolSelectCharacterHookSize, g_qolSelectCharacterOriginal, sizeof(g_qolSelectCharacterOriginal) },
        { g_qolRemovalResultTarget, reinterpret_cast<lm_address_t>(g_qolOriginalRemovalResult), &g_qolRemovalResultHookSize, g_qolRemovalResultOriginal, sizeof(g_qolRemovalResultOriginal) },
        { g_qolValidateRemovalTarget, reinterpret_cast<lm_address_t>(g_qolOriginalValidateRemoval), &g_qolValidateRemovalHookSize, g_qolValidateRemovalOriginal, sizeof(g_qolValidateRemovalOriginal) },
        { g_qolApplyFormationTarget, reinterpret_cast<lm_address_t>(g_qolOriginalApplyFormation), &g_qolApplyFormationHookSize, g_qolApplyFormationOriginal, sizeof(g_qolApplyFormationOriginal) },
    }};

    bool hookEntrypointsRestored = true;
    bool directPatchesRestored = true;
    bool levelEntrypointsRestored = true;
    {
        ScopedOtherThreadSuspension suspension;
        if (!suspension.Active())
        {
            hookEntrypointsRestored = false;
            directPatchesRestored = false;
            levelEntrypointsRestored = false;
        }
        else
        {
            for (const auto& hook : hooks) hookEntrypointsRestored = RestoreQOLHookEntry(hook) && hookEntrypointsRestored;
            if (g_qolEnemyPercentPatched) directPatchesRestored = PatchBytesWhileSuspended(g_qolEnemyPercentPatch, g_qolEnemyPercentOriginal, sizeof(g_qolEnemyPercentOriginal)) && directPatchesRestored;
            if (g_qolSBAPercentPatched) directPatchesRestored = PatchBytesWhileSuspended(g_qolSBAPercentPatch, g_qolSBAPercentOriginal, sizeof(g_qolSBAPercentOriginal)) && directPatchesRestored;
            if (g_qolDamageCapQuestPatched) directPatchesRestored = PatchBytesWhileSuspended(g_qolDamageCapQuestCheck, g_qolQuestCheckOriginal, sizeof(g_qolQuestCheckOriginal)) && directPatchesRestored;
            if (g_qolLevelRewardPatched) levelEntrypointsRestored = PatchBytesWhileSuspended(g_qolLevelRewardTarget, g_qolLevelRewardOriginal, sizeof(g_qolLevelRewardOriginal)) && levelEntrypointsRestored;
            if (g_qolLevelConditionPatched) levelEntrypointsRestored = PatchBytesWhileSuspended(g_qolLevelConditionTarget, g_qolLevelConditionOriginal, sizeof(g_qolLevelConditionOriginal)) && levelEntrypointsRestored;
            if (g_qolLevelSetPatched) levelEntrypointsRestored = PatchBytesWhileSuspended(g_qolLevelSetTarget, g_qolLevelSetOriginal, sizeof(g_qolLevelSetOriginal)) && levelEntrypointsRestored;
        }
    }

    const bool hooksReleased = hookEntrypointsRestored && ReleaseQOLHooksAfterDrain(hooks);

    const bool cavesRetired = levelEntrypointsRestored && RetireQOLLevelCaves();
    const bool restored = hookEntrypointsRestored && directPatchesRestored && levelEntrypointsRestored && hooksReleased && cavesRetired;
    g_qolForcedNormalQuest = false;
    if (!restored) g_patchCoreCanUnload.store(false);
    WriteRuntimeStatus(L"qol", restored ? L"inactive" : L"restore_failed", restored ?
        L"convenience hooks and percentage instructions restored" : L"convenience restoration could not be proven; module kept loaded");
    ReleaseRuntimeOwnerAfterVerifiedStop(L"qol", restored);
    CloseQOLMapping();
	return restored;
}

static bool InstallQOLHook(lm_address_t target, lm_address_t detour, lm_address_t* original, lm_size_t* size, lm_byte_t* bytes, size_t capacity)
{
    if (target == LM_ADDRESS_BAD || !target || !original || !size || !bytes || capacity < 16) return false;
    if (LM_ReadMemory(target, bytes, capacity) != capacity) return false;
    *size = LM_HookCode(target, detour, original);
    return *size != 0 && *size <= capacity;
}

static DWORD RunQOLRuntime()
{
    RuntimeOwnerGuard owner;
    if (!owner.OpenFromCommand(L"qol"))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"qol", L"tool owner identity is missing or no longer alive");
        return 1;
    }
    std::wstring configPath = RuntimePath(L"qol.ini");
    if (configPath.empty() || !ReadQOLConfig(configPath.c_str(), true) || !InitializeQOLMapping())
    {
        WriteRuntimeInactiveAndReleaseOwner(L"qol", L"convenience configuration or shared state is unavailable");
        return 1;
    }
    lm_module_t module{};
    if (!LM_FindModule("granblue_fantasy_relink.exe", &module))
    {
        WriteRuntimeInactiveAndReleaseOwner(L"qol", L"game module is unavailable");
        CloseQOLMapping();
        return 1;
    }

    g_qolStopping.store(false);
    const char* textSignature = "55 41 57 41 56 41 55 41 54 56 57 53 48 83 EC ?? 48 8D 6C 24 ?? 48 C7 45 ?? ?? ?? ?? ?? 44 89 C7 49 89 D3";
    const bool needsText = g_qolSessionEnabled.load() || g_qolEnemyHPEnabled.load() || g_qolSBAEnabled.load();
    if (needsText)
    {
        g_qolTextSetTarget = FindUniqueSignature(textSignature, module);
        if (!InstallQOLHook(g_qolTextSetTarget, reinterpret_cast<lm_address_t>(&QOLTextSetDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalTextSet),
            &g_qolTextSetHookSize, g_qolTextSetOriginal, sizeof(g_qolTextSetOriginal)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"text component hook preflight failed");
            return 1;
        }
    }

    if (g_qolEnemyHPEnabled.load() || g_qolSBAEnabled.load())
    {
        const char* setIntSignature = "56 57 48 83 EC ?? 48 89 CE 89 D0 F7 D8";
        g_qolSetTextFromIntTarget = FindUniqueSignature(setIntSignature, module);
        if (!InstallQOLHook(g_qolSetTextFromIntTarget, reinterpret_cast<lm_address_t>(&QOLSetTextFromIntDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalSetTextFromInt),
            &g_qolSetTextFromIntHookSize, g_qolSetTextFromIntOriginal, sizeof(g_qolSetTextFromIntOriginal)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"numeric text hook preflight failed");
            return 1;
        }
    }

    if (g_qolEnemyHPEnabled.load())
    {
        const char* enemySignature = "56 48 83 EC ?? C5 F8 29 74 24 ?? C5 F8 28 F1 48 89 CE 48 8B 89 ?? ?? ?? ?? 48 85 C9 74 ?? C5 CA 59 05";
        g_qolEnemyHealthTarget = FindUniqueSignature(enemySignature, module);
        g_qolEnemyPercentPatch = g_qolEnemyHealthTarget == LM_ADDRESS_BAD ? LM_ADDRESS_BAD : g_qolEnemyHealthTarget + 0x1E;
        if (g_qolEnemyPercentPatch == LM_ADDRESS_BAD || LM_ReadMemory(g_qolEnemyPercentPatch, g_qolEnemyPercentOriginal, sizeof(g_qolEnemyPercentOriginal)) != sizeof(g_qolEnemyPercentOriginal))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"enemy percentage instruction preflight failed");
            return 1;
        }
        lm_byte_t dummySBA[12]{ 0xC5, 0xCA, 0x59, 0x05, 0, 0, 0, 0, 0xC5, 0xFA, 0x2C, 0xD0 };
        if (!QOLPercentageInstructionsValid(g_qolEnemyPercentOriginal, dummySBA))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"enemy percentage instructions do not match 2.0.2");
            return 1;
        }
        lm_byte_t patch[18]{ 0x66, 0x0F, 0x7E, 0xF2 };
        memset(patch + 4, 0x90, sizeof(patch) - 4);
        if (!PatchBytes(g_qolEnemyPercentPatch, patch, sizeof(patch)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"enemy percentage patch failed");
            return 1;
        }
        g_qolEnemyPercentPatched = true;
        if (!InstallQOLHook(g_qolEnemyHealthTarget, reinterpret_cast<lm_address_t>(&QOLEnemyHealthDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalEnemyHealth),
            &g_qolEnemyHealthHookSize, g_qolEnemyHealthOriginal, sizeof(g_qolEnemyHealthOriginal)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"enemy percentage context hook failed");
            return 1;
        }
    }

    if (g_qolSBAEnabled.load())
    {
        const char* playerSignature = "41 57 41 56 41 55 41 54 56 57 55 53 48 81 EC ?? ?? ?? ?? C5 78 29 84 24 ?? ?? ?? ?? C5 F8 29 BC 24 ?? ?? ?? ?? C5 F8 29 B4 24 ?? ?? ?? ?? 49 89 CC";
        const char* sbaSignature = "C5 CA 59 05 ?? ?? ?? ?? C5 FA 2C D0 E8 ?? ?? ?? ?? C4 C1 7A 11 B4 24 ?? ?? ?? ?? 49 8B 45";
        g_qolPlayerParamTarget = FindUniqueSignature(playerSignature, module);
        g_qolSBAPercentPatch = FindUniqueSignature(sbaSignature, module);
        if (g_qolSBAPercentPatch == LM_ADDRESS_BAD || LM_ReadMemory(g_qolSBAPercentPatch, g_qolSBAPercentOriginal, sizeof(g_qolSBAPercentOriginal)) != sizeof(g_qolSBAPercentOriginal))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"SBA percentage instruction preflight failed");
            return 1;
        }
        lm_byte_t dummyEnemy[18]{ 0xC5, 0xCA, 0x59, 0x05, 0, 0, 0, 0, 0xC4, 0xE3, 0x79, 0x0A, 0xC0, 0x0A, 0xC5, 0xFA, 0x2C, 0xD0 };
        if (!QOLPercentageInstructionsValid(dummyEnemy, g_qolSBAPercentOriginal))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"SBA percentage instructions do not match 2.0.2");
            return 1;
        }
        lm_byte_t patch[12]{ 0x66, 0x0F, 0x7E, 0xF2 };
        memset(patch + 4, 0x90, sizeof(patch) - 4);
        if (!PatchBytes(g_qolSBAPercentPatch, patch, sizeof(patch)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"SBA percentage patch failed");
            return 1;
        }
        g_qolSBAPercentPatched = true;
        if (!InstallQOLHook(g_qolPlayerParamTarget, reinterpret_cast<lm_address_t>(&QOLPlayerParamDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalPlayerParam),
            &g_qolPlayerParamHookSize, g_qolPlayerParamOriginal, sizeof(g_qolPlayerParamOriginal)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"SBA context hook failed");
            return 1;
        }
    }

    if (g_qolDamageCapEnabled.load())
    {
        const char* damageSignature = "41 57 41 56 41 55 41 54 56 57 55 53 48 81 EC ?? ?? ?? ?? C5 F8 29 B4 24 ?? ?? ?? ?? 48 89 D6 48 89 CF 48 8B 42";
        const char* flagSignature = "40 88 3D ?? ?? ?? ?? 48 8B 8B";
        const char* questSignature = "74 ?? 80 3D ?? ?? ?? ?? ?? 75 ?? 41 B4";
        g_qolDamageCapTarget = FindUniqueSignature(damageSignature, module);
        lm_address_t flagInstruction = FindUniqueSignature(flagSignature, module);
        g_qolDamageCapQuestCheck = FindUniqueSignature(questSignature, module);
        int32_t displacement = 0;
        if (flagInstruction == LM_ADDRESS_BAD || g_qolDamageCapQuestCheck == LM_ADDRESS_BAD ||
            LM_ReadMemory(flagInstruction + 3, reinterpret_cast<lm_byte_t*>(&displacement), sizeof(displacement)) != sizeof(displacement) ||
            LM_ReadMemory(g_qolDamageCapQuestCheck, g_qolQuestCheckOriginal, sizeof(g_qolQuestCheckOriginal)) != sizeof(g_qolQuestCheckOriginal) ||
            g_qolQuestCheckOriginal[0] != 0x74)
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"damage-cap display preflight failed");
            return 1;
        }
        g_qolDamageCapFlagAddress = flagInstruction + 6 + static_cast<intptr_t>(displacement);
        const lm_byte_t nops[2]{ 0x90, 0x90 };
        if (!PatchBytes(g_qolDamageCapQuestCheck, nops, sizeof(nops)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"damage-cap quest check patch failed");
            return 1;
        }
        g_qolDamageCapQuestPatched = true;
        if (!InstallQOLHook(g_qolDamageCapTarget, reinterpret_cast<lm_address_t>(&QOLDamageCapDetour), reinterpret_cast<lm_address_t*>(&g_qolOriginalDamageCap),
            &g_qolDamageCapHookSize, g_qolDamageCapOriginal, sizeof(g_qolDamageCapOriginal)))
        {
            WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"damage-cap display hook failed");
            return 1;
        }
    }

    if (g_qolLevelSyncEnabled.load() && !InstallQOLLevelSync(module))
    {
        WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"normal-quest level-sync preflight failed");
        return 1;
    }

    if (g_qolReturnWrightstoneEnabled.load() && !InstallQOLReturnWrightstone(module))
    {
        WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"wrightstone-return preflight failed");
        return 1;
    }

    if (g_qolFreeCaptainEnabled.load() && !InstallQOLFreeCaptain(module))
    {
        WriteStartupFailureAfterStop(L"qol", StopQOLRuntime(), L"main-story free-captain preflight failed");
        return 1;
    }

    WriteRuntimeStatus(L"qol", L"active", L"built-in convenience runtime is active");
    while (owner.Alive() && GetPrivateProfileIntW(L"qol", L"enabled", 0, configPath.c_str()) == 1) Sleep(250);
    StopQOLRuntime();
    return 0;
}

static bool ShouldApply(const char* requestedId, const PatchPoint& point)
{
    return PatchIdEquals(requestedId, point.id);
}

static bool ApplyMonsterPatches(wchar_t* message, size_t messageSize)
{
    char patchId[64]{};
    if (!ReadPatchId(patchId, sizeof(patchId)))
    {
        swprintf_s(message, messageSize, L"missing patch id");
        return false;
    }
    if (strcmp(patchId, "all") == 0)
    {
        swprintf_s(message, messageSize, L"batch patch id is unsupported");
        return false;
    }
    constexpr bool kStableReleaseCandidateMonsterDamageEnabled = true;
    if (!kStableReleaseCandidateMonsterDamageEnabled && PatchIdEquals(patchId, "monster_damage_new"))
    {
        swprintf_s(message, messageSize, L"candidate monster damage is disabled in the stable build pending field acceptance");
        return false;
    }

    lm_module_t module{};
    if (!LM_FindModule("granblue_fantasy_relink.exe", &module))
    {
        swprintf_s(message, messageSize, L"LM_FindModule failed");
        return false;
    }

    if (PatchIdEquals(patchId, "monster_damage") || PatchIdEquals(patchId, "monster_damage_new"))
    {
        if (!ConfigurePlayerDamage(message, messageSize)) return false;
        if (!InstallPlayerPointerHook(module, message, messageSize)) return false;
    }

    int patched = 0;
    int already = 0;
    int selected = 0;

    for (const auto& point : kMonsterPatches)
    {
        if (!ShouldApply(patchId, point)) continue;
        ++selected;

		lm_address_t resolvedRva = point.rva;
		bool resolvedKnownEntry = false;
		const lm_address_t rva203 = MonsterPatchRva203(point.id);
		const lm_address_t rva204 = MonsterPatchRva204(point.id);
		const lm_address_t knownRvas[] = { point.rva, rva203, rva204 };
		for (const lm_address_t candidate : knownRvas)
		{
			if (candidate == LM_ADDRESS_BAD || (candidate == point.rva && resolvedKnownEntry)) continue;
			lm_byte_t candidateBytes[32]{};
			if (point.size > sizeof(candidateBytes)) continue;
			const lm_address_t candidateTarget = module.base + candidate;
			if (LM_ReadMemory(candidateTarget, candidateBytes, point.size) != point.size) continue;
			if (BytesEqual(candidateBytes, point.expected, point.size) ||
				(point.hook && IsMarkedMonsterHook(candidateTarget, point.id)))
			{
				resolvedRva = candidate;
				resolvedKnownEntry = true;
				break;
			}
		}
		const char* signature = nullptr;
		lm_address_t signatureOffset = 0;
		if (strcmp(point.id, "monster_hp") == 0)
			signature = "48 8B 41 10 45 31 C9 48 29 D0 4C 0F 43 C8 B8 01 00 00 00 49 0F 47 C1 45 85 C0 49 0F 44 C1 48 89 41 10 C3";
		else if (strcmp(point.id, "monster_damage_new") == 0)
		{
			signature = "48 89 51 18 48 89 51 10 C3 CC CC CC CC CC CC CC 48 89 51 18 C3 CC CC CC CC CC CC CC CC CC CC CC 48 89 51 10 C3";
			signatureOffset = 0x20;
		}
		else if (strcmp(point.id, "monster_stun") == 0)
			signature = "C5 FA 58 86 60 ?? ?? ?? C5 FA 5D 86 64 ?? ?? ?? C5 FA 11 86 60 ?? ?? ??";
		else if (strcmp(point.id, "overdrive_state") == 0)
			signature = "8B 46 10 83 F8 03 0F 84 ?? ?? ?? ?? 83 F8 01 0F 84 ?? ?? ?? ??";
		else if (strcmp(point.id, "od_rate") == 0)
			signature = "80 79 50 00 74 13 48 03 51 18";

		if (!resolvedKnownEntry && signature != nullptr)
		{
			const lm_address_t match = FindUniqueSignature(signature, module);
			if (match != LM_ADDRESS_BAD) resolvedRva = match + signatureOffset - module.base;
		}
		else if (strcmp(point.id, "inventory_set_45") == 0)
		{
			// This shared inventory/material instruction moved in 2.0.3. Resolve
			// only between the two audited RVAs and still require the complete
			// seven original bytes before any write.
			const lm_address_t candidates[] = { 0x356621, 0x34F8F1 };
			bool found = false;
			for (const lm_address_t candidate : candidates)
			{
				lm_byte_t original[sizeof(kInventorySet45Expected)]{};
				const lm_address_t candidateTarget = module.base + candidate;
				if (LM_ReadMemory(candidateTarget, original, sizeof(original)) == sizeof(original) &&
					BytesEqual(original, kInventorySet45Expected, sizeof(original)))
				{
					resolvedRva = candidate;
					found = true;
					break;
				}
			}
			if (!found)
			{
				const lm_address_t signature = FindUniqueSignature(
					"41 01 76 04 4C 89 E1 E8 ?? ?? ?? ?? 41 8B 0C 24 31 C0 85 C9 0F 4F C1",
					module);
				if (signature != LM_ADDRESS_BAD)
				{
					lm_byte_t original[sizeof(kInventorySet45Expected)]{};
					if (LM_ReadMemory(signature, original, sizeof(original)) == sizeof(original) &&
						BytesEqual(original, kInventorySet45Expected, sizeof(original)))
					{
						resolvedRva = signature - module.base;
						found = true;
					}
				}
			}
			if (!found)
			{
				swprintf_s(message, messageSize, L"inventory/material entry did not match known RVAs or unique AOB");
				return false;
			}
		}

		lm_address_t target = module.base + resolvedRva;
        std::vector<lm_byte_t> current(point.size);
        if (LM_ReadMemory(target, current.data(), point.size) != point.size)
        {
            swprintf_s(message, messageSize, L"read failed: %s at +%llX", point.name, static_cast<unsigned long long>(resolvedRva));
            return false;
        }

        if (point.hook && current[0] == 0xE9)
        {
            ++already;
            continue;
        }
        if (!point.hook && BytesEqual(current.data(), point.patch, point.size))
        {
            ++already;
            continue;
        }

        if (!BytesEqual(current.data(), point.expected, point.size))
        {
            swprintf_s(message, messageSize, L"unexpected bytes: %s at +%llX", point.name, static_cast<unsigned long long>(resolvedRva));
            return false;
        }

        if (point.hook)
        {
            if (strcmp(point.id, "monster_stun") == 0)
            {
                if (!PatchStunHook(target, message, messageSize)) return false;
            }
            else if (strcmp(point.id, "monster_damage_new") == 0)
            {
                if (!PatchMonsterDamageNewHook(target, message, messageSize)) return false;
            }
            else if (strcmp(point.id, "monster_damage") == 0)
            {
                if (!PatchMonsterDamageHook(target, message, messageSize)) return false;
            }
            else if (strcmp(point.id, "overdrive_state") == 0)
            {
                if (!PatchOverdriveHook(target, message, messageSize)) return false;
            }
            else if (strcmp(point.id, "od_rate") == 0)
            {
                if (!PatchOdRateHook(target, message, messageSize)) return false;
            }
            else if (strcmp(point.id, "inventory_set_45") == 0)
            {
                if (!PatchInventorySetQuantityHook(target, message, messageSize)) return false;
            }
            else if (!PatchDamageHook(target, message, messageSize)) return false;
        }
        else if (!PatchBytes(target, point.patch, point.size))
        {
            swprintf_s(message, messageSize, L"write failed: %s at +%llX", point.name, static_cast<unsigned long long>(resolvedRva));
            return false;
        }
        ++patched;
    }

    if (selected == 0)
    {
        swprintf_s(message, messageSize, L"unknown patch id: %S", patchId);
        return false;
    }

    swprintf_s(message, messageSize, L"monster enhance ok: id %S patched %d, already %d", patchId, patched, already);
    return true;
}

static DWORD WINAPI InitThread(LPVOID)
{
    char command[64]{};
    if (ReadPatchId(command, sizeof(command)))
    {
        if (PatchIdEquals(command, "runtime_camera")) return RunCameraRuntime();
        if (PatchIdEquals(command, "runtime_audio")) return RunAudioRuntime();
        if (PatchIdEquals(command, "runtime_party_observer")) return RunPartyObserverRuntime();
        if (PatchIdEquals(command, "runtime_virtual_sigils")) return RunVirtualSigilRuntime();
        if (PatchIdEquals(command, "runtime_weapon_skills")) return RunWeaponSkillsRuntime();
        if (PatchIdEquals(command, "runtime_damage")) return RunDamageRuntime();
        if (PatchIdEquals(command, "runtime_qol")) return RunQOLRuntime();
    }
    wchar_t message[256]{};
    ApplyMonsterPatches(message, _countof(message));

    wchar_t debugMessage[320]{};
    swprintf_s(debugMessage, L"[patch_core] %s\n", message);
    OutputDebugStringW(debugMessage);

	return 0;
}

static DWORD WINAPI ModuleThread(LPVOID parameter)
{
	HMODULE module = static_cast<HMODULE>(parameter);
	DWORD result = InitThread(nullptr);
	if (g_patchCoreCanUnload.load()) FreeLibraryAndExitThread(module, result);
	return result;
}

BOOL APIENTRY DllMain( HMODULE hModule,
                       DWORD  ul_reason_for_call,
                       LPVOID lpReserved
                     )
{
    switch (ul_reason_for_call)
    {
	case DLL_PROCESS_ATTACH:
		g_patchCoreModule = hModule;
		DisableThreadLibraryCalls(hModule);
		if (HANDLE thread = CreateThread(nullptr, 0, ModuleThread, hModule, 0, nullptr))
        {
            CloseHandle(thread);
        }
        break;
    case DLL_PROCESS_DETACH:
        ClosePlayerPointers();
        break;
    }
    return TRUE;
}

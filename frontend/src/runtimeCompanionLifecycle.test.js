import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('../../src_dll/patch_core/dllmain.cpp', import.meta.url), 'utf8')

test('all built-in runtimes have explicit start and restoration paths', () => {
  for (const feature of ['Camera', 'Audio', 'VirtualSigil']) {
    assert.match(source, new RegExp(`Run${feature}Runtime\\(\\)`), `${feature} start path is missing`)
    assert.match(source, new RegExp(`Stop${feature}Runtime\\(\\)`), `${feature} stop path is missing`)
  }
  assert.match(source, /runtime_camera/)
  assert.match(source, /runtime_audio/)
  assert.match(source, /runtime_virtual_sigils/)
  assert.match(source, /FreeLibraryAndExitThread/)
  assert.match(source, /GetModuleFileNameW\(g_patchCoreModule/)
  assert.match(source, /\.command/)
})

test('libmem hooks restore their entry bytes before draining callbacks and freeing trampolines', () => {
  const helper = source.slice(source.indexOf('static bool RestoreLibmemHookAfterDrain'), source.indexOf('static bool StampMonsterCave'))
  assert.ok(helper.indexOf('PatchBytes(target, original, size)') < helper.indexOf('callbacks.load()'))
  assert.ok(helper.indexOf('callbacks.load()') < helper.indexOf('LM_UnhookCode'))
  assert.match(helper, /g_patchCoreCanUnload\.store\(false\)/)
})

test('camera stops new callbacks, restores hooks, then restores owned values', () => {
  const shutdown = source.slice(source.indexOf('static bool StopCameraRuntime()'), source.indexOf('static DWORD RunCameraRuntime()'))
  assert.ok(shutdown.indexOf('g_cameraStopping.store(true)') < shutdown.indexOf('RestoreLibmemHookAfterDrain'))
  assert.ok(shutdown.indexOf('RestoreLibmemHookAfterDrain') < shutdown.indexOf('RestoreCameraValuesLocked()'))
})

test('camera signatures scan only committed readable regions', () => {
  const scanner = source.slice(source.indexOf('static bool ParseSignature'), source.indexOf('static LONG g_autoOverdrivePhase'))
  assert.match(scanner, /VirtualQuery/)
  assert.match(scanner, /MEM_COMMIT/)
  assert.match(scanner, /PAGE_GUARD \| PAGE_NOACCESS/)
  assert.doesNotMatch(scanner, /LM_SigScan|LM_PatternScan/)
})

test('callback runtimes use the shared drain-before-free restoration path', () => {
  const audio = source.slice(source.indexOf('static bool StopAudioRuntime()'), source.indexOf('static DWORD RunAudioRuntime()'))
  assert.ok(audio.indexOf('g_audioStopping.store(true)') < audio.indexOf('RestoreLibmemHookAfterDrain'))

  const virtual = source.slice(source.indexOf('static bool StopVirtualSigilRuntime()'), source.indexOf('static DWORD RunVirtualSigilRuntime()'))
  assert.ok(virtual.indexOf('g_virtualStopping.store(true)') < virtual.indexOf('RestoreLibmemHookAfterDrain'))
  assert.ok(virtual.indexOf('g_traitFetchInstalled') < virtual.indexOf('RestoreLibmemHookAfterDrain'))
})

test('runtime status protocol separates safe startup failures, active warnings, and failed restoration', () => {
  for (const feature of ['camera', 'virtual-sigils', 'audio', 'damage', 'qol']) {
    assert.match(source, new RegExp(`WriteRuntimeStatus\\(L"${feature}", [^;]*L"restore_failed"`), `${feature} does not expose failed restoration`)
  }
  assert.match(source, /WriteRuntimeStatus\(L"virtual-sigils", L"active", L"hot configuration was rejected; keeping the active mapping"\)/)
  assert.match(source, /WriteRuntimeStatus\(L"audio", L"active", L"hot configuration was rejected; keeping the last valid volumes"\)/)
  assert.doesNotMatch(source, /WriteRuntimeStatus\([^\n]+L"error"/)
})

test('virtual sigil config reads allow atomic replacement while the runtime is active', () => {
  const virtual = source.slice(source.indexOf('static bool ReadSharedFile'), source.indexOf('static bool ResolveOwnedStatus'))
  assert.match(virtual, /FILE_SHARE_READ \| FILE_SHARE_WRITE \| FILE_SHARE_DELETE/)
  assert.doesNotMatch(virtual, /std::ifstream/)
})

test('the pages do not expose external-loader or mod-directory selection', () => {
  for (const page of ['CameraLab.vue', 'AudioMixerLab.vue', 'VirtualSigilLab.vue']) {
    const value = readFileSync(new URL(`./components/${page}`, import.meta.url), 'utf8')
    assert.doesNotMatch(value, /Mods directory|Mods 目录|Select\w+ModsDirectory/)
    assert.match(value, /BUILT-IN|内置运行时/)
  }
})

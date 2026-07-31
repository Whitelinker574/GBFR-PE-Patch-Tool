const zhRuntimeMessages = Object.freeze([
  ['hooks and owned camera values restored', '镜头 Hook 与应用管理的镜头参数已恢复'],
  ['camera entry restoration could not be proven; module kept loaded', '无法确认镜头入口已完整恢复；为避免崩溃，运行时保持加载'],
  ['camera configuration is missing or invalid', '镜头配置缺失或无效'],
  ['game module is unavailable', '无法读取游戏主模块'],
  ['camera signature preflight failed or was ambiguous', '镜头签名预检失败或命中不唯一'],
  ['camera entry preflight read failed', '镜头入口预检读取失败'],
  ['camera hook installation failed', '镜头 Hook 安装失败'],
  ['native camera runtime is active', '内置镜头运行时正在工作'],
  ['hooks and native loop limits restored', '虚拟因子 Hook 与原生槽位上限已恢复'],
  ['virtual sigil restoration could not be proven; module kept loaded', '无法确认虚拟因子 Hook 已完整恢复；为避免崩溃，运行时保持加载'],
  ['virtual sigil configuration is missing or invalid', '虚拟因子配置缺失或无效'],
  ['virtual sigil executable preflight failed', '虚拟因子游戏版本预检失败'],
  ['virtual sigil getter preflight read failed', '虚拟因子读取入口预检失败'],
  ['virtual sigil hook installation failed', '虚拟因子 Hook 安装失败'],
  ['virtual sigil loop-limit patch failed', '虚拟因子槽位上限补丁失败'],
  ['native virtual sigil runtime is active', '内置虚拟因子运行时正在工作'],
  ['hot configuration was rejected; keeping the active mapping', '热更新配置被拒绝，继续保留上一份有效映射'],
  ['audio hook restored after active callbacks drained', '音频回调已安全结束，Wwise Hook 已恢复'],
  ['audio entry restoration could not be proven; module kept loaded', '无法确认音频入口已完整恢复；为避免崩溃，运行时保持加载'],
  ['audio configuration is missing or invalid', '音频配置缺失或无效'],
  ['Wwise exports or character voice banks are unavailable', '无法读取 Wwise 接口或角色语音 bank'],
  ['Volume_Voice or Volume_SE RTPC was not resolved', '无法定位游戏语音或界面音效总音量参数'],
  ['Volume_Voice RTPC was not resolved', '无法定位游戏语音总音量参数'],
  ['audio entry preflight read failed', '音频入口预检读取失败'],
  ['audio hook installation failed', '音频 Hook 安装失败'],
  ['native Wwise character-volume runtime is active', '内置角色语音运行时正在工作'],
  ['hot configuration was rejected; keeping the last valid volumes', '热更新配置被拒绝，继续使用上一份有效音量'],
])

export function runtimeCompanionMessage(value, locale = 'zh') {
  const message = String(value || '')
  if (!message || locale === 'en') return message
  return zhRuntimeMessages.reduce(
    (result, [source, translated]) => result.replaceAll(source, translated),
    message,
  )
}

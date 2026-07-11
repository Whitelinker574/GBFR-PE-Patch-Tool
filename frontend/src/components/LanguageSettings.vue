<script setup>
import { computed, ref } from 'vue'
import { SetLanguage } from '../../wailsjs/go/main/App'
import { language, storeLanguage } from '../i18n'

const applying = ref(false)
const current = computed(() => language.value)

const text = computed(() => current.value === 'zh' ? {
  title: '语言设置',
  hint: '选择界面语言。更改后界面会自动重新加载。',
  current: '当前语言',
  english: 'English',
  englishDescription: '使用完整英文界面（默认）',
  chinese: '简体中文',
  chineseDescription: '使用原始中文界面',
  active: '已启用',
  switch: '切换',
  switching: '正在切换…',
  defaultLabel: '默认',
} : {
  title: 'Language Settings',
  hint: 'Choose the interface language. The interface reloads automatically after a change.',
  current: 'Current language',
  english: 'English',
  englishDescription: 'Use the complete English interface (default)',
  chinese: 'Simplified Chinese',
  chineseDescription: 'Use the original Chinese interface',
  active: 'Active',
  switch: 'Switch',
  switching: 'Switching…',
  defaultLabel: 'Default',
})

async function selectLanguage(next) {
  if (applying.value || next === current.value) return
  applying.value = true
  try {
    await SetLanguage(next)
    storeLanguage(next)
    window.setTimeout(() => window.location.reload(), 80)
  } catch (error) {
    console.error('Failed to change language:', error)
    applying.value = false
  }
}
</script>

<template>
  <section class="language-panel">
    <div class="section-header">
      <div>
        <h2>{{ text.title }}</h2>
        <p>{{ text.hint }}</p>
      </div>
      <span class="current-language">{{ text.current }}: {{ current === 'en' ? text.english : text.chinese }}</span>
    </div>

    <div class="language-grid">
      <article class="language-card" :class="{ active: current === 'en' }">
        <div class="language-icon">EN</div>
        <div class="language-copy">
          <div class="language-title-row">
            <h3>{{ text.english }}</h3>
            <span class="default-badge">{{ text.defaultLabel }}</span>
          </div>
          <p>{{ text.englishDescription }}</p>
        </div>
        <button class="language-button" :disabled="applying || current === 'en'" @click="selectLanguage('en')">
          {{ current === 'en' ? text.active : applying ? text.switching : text.switch }}
        </button>
      </article>

      <article class="language-card" :class="{ active: current === 'zh' }">
        <div class="language-icon">中</div>
        <div class="language-copy">
          <div class="language-title-row">
            <h3>{{ text.chinese }}</h3>
          </div>
          <p>{{ text.chineseDescription }}</p>
        </div>
        <button class="language-button" :disabled="applying || current === 'zh'" @click="selectLanguage('zh')">
          {{ current === 'zh' ? text.active : applying ? text.switching : text.switch }}
        </button>
      </article>
    </div>
  </section>
</template>

<style scoped>
.language-panel {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 18px;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 16px;
  background: rgba(255,255,255,0.045);
}
h2 { margin: 0; color: rgba(255,255,255,0.9); font-size: 1.1rem; }
.section-header p { margin: 7px 0 0; color: rgba(255,255,255,0.45); font-size: 0.8rem; line-height: 1.5; }
.current-language {
  flex-shrink: 0;
  padding: 5px 10px;
  border-radius: 999px;
  color: #67e8f9;
  background: rgba(103,232,249,0.12);
  border: 1px solid rgba(103,232,249,0.2);
  font-size: 0.72rem;
  font-weight: 700;
}
.language-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.language-card {
  display: grid;
  grid-template-columns: auto 1fr;
  grid-template-rows: 1fr auto;
  gap: 12px;
  padding: 18px;
  border-radius: 16px;
  border: 1px solid rgba(255,255,255,0.08);
  background: linear-gradient(145deg, rgba(255,255,255,0.06), rgba(255,255,255,0.025));
  transition: border-color 0.2s, transform 0.2s, background 0.2s;
}
.language-card:hover { transform: translateY(-2px); border-color: rgba(103,232,249,0.22); }
.language-card.active { border-color: rgba(103,232,249,0.5); background: linear-gradient(145deg, rgba(103,232,249,0.13), rgba(99,102,241,0.08)); }
.language-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  background: rgba(103,232,249,0.12);
  color: #67e8f9;
  font-weight: 800;
  font-size: 1rem;
}
.language-copy { min-width: 0; }
.language-title-row { display: flex; align-items: center; gap: 8px; }
h3 { margin: 2px 0 0; font-size: 0.92rem; color: rgba(255,255,255,0.84); }
.language-copy p { margin: 7px 0 0; color: rgba(255,255,255,0.42); font-size: 0.76rem; line-height: 1.45; }
.default-badge { font-size: 0.62rem; padding: 2px 7px; border-radius: 999px; color: #fbbf24; background: rgba(251,191,36,0.12); }
.language-button {
  grid-column: 1 / -1;
  width: 100%;
  padding: 9px 12px;
  border-radius: 10px;
  border: 1px solid rgba(103,232,249,0.28);
  color: #67e8f9;
  background: rgba(103,232,249,0.09);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
}
.language-button:not(:disabled):hover { background: rgba(103,232,249,0.18); }
.language-button:disabled { cursor: default; opacity: 0.55; }
.language-card.active .language-button { color: #4ade80; border-color: rgba(74,222,128,0.25); background: rgba(74,222,128,0.09); }
@media (max-width: 650px) {
  .language-grid { grid-template-columns: 1fr; }
  .section-header { flex-direction: column; }
}
</style>

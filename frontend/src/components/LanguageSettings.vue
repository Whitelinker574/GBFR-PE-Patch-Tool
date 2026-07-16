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
  container-type: inline-size;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 18px;
  border: 1px solid rgba(145,108,51,0.22);
  border-radius: 8px 16px 8px 16px;
  background: rgba(255,250,236,0.76);
}
h2 { margin: 0; color: #51483e; font-size: 1.1rem; font-weight: 900; }
.section-header p { margin: 7px 0 0; color: #87765f; font-size: 0.8rem; line-height: 1.5; }
.current-language {
  flex-shrink: 0;
  padding: 5px 10px;
  border-radius: 999px;
  color: #1f5c61;
  background: rgba(91,190,201,0.15);
  border: 1px solid rgba(48,137,145,0.28);
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
  border-radius: 7px 15px 7px 15px;
  border: 1px solid rgba(145,108,51,0.22);
  background: linear-gradient(145deg, #fffaf0, #f5ead1);
  transition: border-color 0.2s, transform 0.2s, background 0.2s;
}
.language-card:hover { transform: translateY(-2px); border-color: rgba(48,137,145,0.42); }
.language-card.active { border-color: rgba(48,137,145,0.58); background: linear-gradient(145deg, #e7f5ef, #f4ecd5); }
.language-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  background: rgba(91,190,201,0.18);
  color: #1f5c61;
  font-weight: 800;
  font-size: 1rem;
}
.language-copy { min-width: 0; }
.language-title-row { display: flex; align-items: center; gap: 8px; }
h3 { margin: 2px 0 0; font-size: 0.92rem; color: #51483e; font-weight: 900; }
.language-copy p { margin: 7px 0 0; color: #87765f; font-size: 0.76rem; line-height: 1.45; }
.default-badge { font-size: 0.62rem; padding: 2px 7px; border-radius: 999px; color: #6e4a14; background: rgba(218,187,115,0.23); font-weight: 800; }
.language-button {
  grid-column: 1 / -1;
  width: 100%;
  padding: 9px 12px;
  border-radius: 10px;
  border: 1px solid rgba(48,137,145,0.34);
  color: #286e75;
  background: rgba(91,190,201,0.11);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
}
.language-button:not(:disabled):hover { background: rgba(91,190,201,0.2); }
.language-button:disabled { cursor: default; opacity: 1; color:#5b4930; border-color:rgba(126,91,42,.34); background:#e6d2a5; }
.language-card.active .language-button { color:#5b4930; border-color:rgba(126,91,42,.34); background:#e6d2a5; }
@media (max-width: 650px) {
  .language-grid { grid-template-columns: 1fr; }
  .section-header { flex-direction: column; }
}
@container (max-width:560px) {
  .language-grid { grid-template-columns:1fr; }
  .section-header { flex-direction:column; }
}
</style>

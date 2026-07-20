<script setup>
import { computed, ref } from 'vue'
import { SetLanguage } from '../../wailsjs/go/main/App'
import { language, storeLanguage } from '../i18n'

const applying = ref(false)
const current = computed(() => language.value)
const isActive = id => current.value === id

const text = computed(() => current.value === 'zh' ? {
  title: '语言设置',
  hint: '选择界面语言。更改后界面会自动重新加载。',
  current: '当前语言',
  english: 'English',
  englishDescription: '使用纯英文界面',
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
  englishDescription: 'Use the English interface',
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
  <section class="language-panel ui-page-stack" :aria-label="text.title" :aria-busy="applying">
    <div class="language-summary ui-notice">
      <p>{{ text.hint }}</p>
      <span class="current-language ui-tag">{{ text.current }}: {{ current === 'en' ? text.english : text.chinese }}</span>
    </div>

    <div class="language-grid ui-card-grid" role="group" :aria-label="text.title">
      <article class="language-card ui-card" :class="{ active: isActive('en') }">
        <div class="language-icon">EN</div>
        <div class="language-copy">
          <div class="language-title-row">
            <h3>{{ text.english }}</h3>
            <span class="default-badge ui-tag">{{ text.defaultLabel }}</span>
          </div>
          <p>{{ text.englishDescription }}</p>
        </div>
        <button class="language-button ui-btn" :class="isActive('en') ? 'is-ghost' : 'is-primary'" :aria-pressed="isActive('en')" :disabled="applying || isActive('en')" @click="selectLanguage('en')">
          {{ current === 'en' ? text.active : applying ? text.switching : text.switch }}
        </button>
      </article>

      <article class="language-card ui-card" :class="{ active: isActive('zh') }">
        <div class="language-icon">中</div>
        <div class="language-copy">
          <div class="language-title-row">
            <h3>{{ text.chinese }}</h3>
          </div>
          <p>{{ text.chineseDescription }}</p>
        </div>
        <button class="language-button ui-btn" :class="isActive('zh') ? 'is-ghost' : 'is-primary'" :aria-pressed="isActive('zh')" :disabled="applying || isActive('zh')" @click="selectLanguage('zh')">
          {{ current === 'zh' ? text.active : applying ? text.switching : text.switch }}
        </button>
      </article>
    </div>
  </section>
</template>

<style scoped>
.language-panel {
  width:min(100%,780px);
  margin-inline:auto;
  container:language-panel / inline-size;
}
.language-summary {
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-5);
}
.language-summary p {
  margin:0;
  color:inherit;
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.current-language {
  flex:0 0 auto;
  color:var(--accent-hover);
  background:var(--accent-soft);
}
.language-grid {
  --ui-grid-min:280px;
  align-items:stretch;
}
.language-card {
  position:relative;
  min-width:0;
  display:grid;
  grid-template-columns:auto minmax(0,1fr);
  grid-template-rows:minmax(66px,1fr) auto;
  gap:var(--space-5);
  padding:var(--space-6);
  overflow:hidden;
  transition:var(--transition-control);
}
.language-card.active {
  border-color:var(--selected-border);
  background:color-mix(in srgb,var(--accent-soft) 42%,var(--surface-card-pop));
  box-shadow:4px 0 0 var(--selected-bar),var(--shadow-1);
}
.language-icon {
  width:48px;
  height:48px;
  display:grid;
  place-items:center;
  border:1px solid var(--border-strong);
  border-radius:var(--radius-md);
  color:var(--accent-hover);
  background:var(--surface-field);
  font-family:var(--font-data);
  font-size:var(--fs-base);
  font-weight:var(--fw-bold);
}
.language-card.active .language-icon {
  color:var(--selected-fg);
  border-color:var(--selected-border);
  background:var(--selected-bg);
}
.language-copy { min-width:0; }
.language-title-row {
  min-width:0;
  display:flex;
  flex-wrap:wrap;
  align-items:center;
  gap:var(--space-2);
}
.language-title-row h3 {
  margin:0;
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:var(--fs-base);
  font-weight:var(--fw-bold);
}
.language-copy p {
  margin:var(--space-2) 0 0;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.default-badge { color:var(--text-secondary); background:var(--surface-field); }
.language-button { grid-column:1 / -1; width:100%; }
.language-card.active .language-button:disabled {
  opacity:1;
  color:var(--accent-hover);
  border-color:var(--border-strong);
  background:transparent;
}

@container language-panel (max-width:640px) {
  .language-summary { align-items:flex-start; flex-direction:column; }
  .language-grid { grid-template-columns:minmax(0,1fr); }
}
@media (max-height:620px) {
  .language-card { grid-template-rows:auto auto; padding:var(--space-4); }
}
</style>

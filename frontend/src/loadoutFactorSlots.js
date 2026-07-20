export const FACTOR_SLOT_COUNT = 12

function normalizeSlots(slots = []) {
  return Array.from({ length: FACTOR_SLOT_COUNT }, (_, index) => slots[index] || null)
}

export function createFactorSlots(sigils = []) {
  const slots = Array.from({ length: FACTOR_SLOT_COUNT }, () => null)
  sigils.slice(0, FACTOR_SLOT_COUNT).forEach((item, fallbackIndex) => {
    const slotId = Number(typeof item === 'object' ? item?.slotId : item)
    const savedIndex = Number(typeof item === 'object' ? item?.index : NaN)
    const index = Number.isInteger(savedIndex) && savedIndex >= 0 && savedIndex < FACTOR_SLOT_COUNT
      ? savedIndex
      : fallbackIndex
    if (slotId > 0) slots[index] = { kind: 'bag', slotId }
  })
  return slots
}

export function factorSlotCount(slots = []) {
  return normalizeSlots(slots).filter(Boolean).length
}

export function putBagFactor(slots, index, slotId) {
  const next = normalizeSlots(slots)
  if (index < 0 || index >= FACTOR_SLOT_COUNT || !Number(slotId)) return next
  const duplicateIndex = next.findIndex((entry, slotIndex) => slotIndex !== index && entry?.kind === 'bag' && entry.slotId === Number(slotId))
  if (duplicateIndex >= 0) next[duplicateIndex] = next[index]
  next[index] = { kind: 'bag', slotId: Number(slotId) }
  return next
}

export function putConstructedFactor(slots, index, item, preview = {}) {
  const next = normalizeSlots(slots)
  if (index < 0 || index >= FACTOR_SLOT_COUNT || !item?.sigilId) return next
  next[index] = { kind: 'construct', item: { ...item }, preview: { ...preview } }
  return next
}

export function clearFactorSlot(slots, index) {
  const next = normalizeSlots(slots)
  if (index >= 0 && index < FACTOR_SLOT_COUNT) next[index] = null
  return next
}

export function buildFactorWritePayload(slots = []) {
  const normalized = normalizeSlots(slots)
  return {
    sigilSlotIds: normalized.map(entry => entry?.kind === 'bag' ? entry.slotId : 0),
    constructedSigils: normalized.flatMap((entry, index) => {
      if (entry?.kind !== 'construct') return []
      const templateSlotId = Number(entry.item.templateSlotId || 0)
      return [{
        index,
        ...(templateSlotId > 0 ? { templateSlotId } : {}),
        item: Object.fromEntries(Object.entries(entry.item).filter(([key]) => key !== 'templateSlotId')),
      }]
    }),
  }
}

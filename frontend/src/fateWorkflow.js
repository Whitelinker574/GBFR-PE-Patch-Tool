// Nonzero mission states already count as complete in the save reader.
// Preserve the original state instead of normalising it back to 1.
export function fateFieldNeedsWrite(field) {
  const current = Number(field?.currentValue ?? 0) >>> 0
  if (field?.field === 'missionState') return current === 0
  const target = Number(field?.allowedTargetValues?.[0] ?? current) >>> 0
  return current !== target
}

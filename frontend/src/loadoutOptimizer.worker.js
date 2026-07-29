import { solveEquipmentAwareSuggestions, solveLoadoutSuggestions, solveLoadoutSuggestionsByDomain, solveMixedOptimizerDomains } from './loadoutOptimizer.js'

self.addEventListener('message', event => {
  const id = Number(event.data?.id || 0)
  try {
    const payload = event.data?.payload || {}
    const results = event.data?.solveMixedDomains
      ? solveMixedOptimizerDomains(payload)
      : event.data?.solveEquipmentAware
        ? solveEquipmentAwareSuggestions(payload).map((result, index) => ({ ...result, domain: payload?.snapshot?.domain || 'inventory', domainRank: index + 1 }))
        : event.data?.solveAllDomains
          ? solveLoadoutSuggestionsByDomain(payload)
          : solveLoadoutSuggestions(payload)
    self.postMessage({ id, results })
  } catch (error) {
    self.postMessage({ id, error: String(error?.message || error) })
  }
})

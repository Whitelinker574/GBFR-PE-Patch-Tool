export function createOptimizerWorkerMessage(id, payload, solveAllDomains, modes = {}) {
  return JSON.parse(JSON.stringify({
    id,
    payload,
    solveAllDomains: Boolean(solveAllDomains),
    solveFixedRoute: Boolean(modes?.solveFixedRoute),
  }))
}

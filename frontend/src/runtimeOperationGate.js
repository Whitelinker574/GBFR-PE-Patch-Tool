export function createOperationGate() {
  let active = null
  let sequence = 0

  return {
    begin(kind) {
      if (active) return null
      active = Object.freeze({ id: ++sequence, kind: String(kind || '') })
      return active
    },
    isCurrent(token) {
      return token != null && active === token
    },
    finish(token) {
      if (active === token) active = null
    },
    invalidate() {
      sequence += 1
      active = null
    },
    get busy() {
      return active != null
    },
  }
}

export function freezeSigilLoadout(entries, normalize) {
  return Object.freeze(entries.map(entry => Object.freeze({ ...normalize(entry) })))
}

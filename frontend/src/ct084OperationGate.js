import { createOperationGate } from './runtimeOperationGate.js'

export function createCT084OperationGate(onChange = () => {}) {
  if (typeof onChange !== 'function') throw new TypeError('CT operation change listener must be a function')

  const gate = createOperationGate()
  let current = null

  function publish() {
    onChange(current)
  }

  return {
    begin(kind, featureID = '') {
      const token = gate.begin(kind)
      if (!token) return null
      current = Object.freeze({
        token,
        kind: token.kind,
        featureID: String(featureID || ''),
      })
      publish()
      return token
    },
    isCurrent(token) {
      return gate.isCurrent(token)
    },
    finish(token) {
      if (!gate.isCurrent(token)) return
      gate.finish(token)
      current = null
      publish()
    },
    reset() {
      gate.invalidate()
      current = null
      publish()
    },
    get busy() {
      return gate.busy
    },
    get current() {
      return current
    },
  }
}

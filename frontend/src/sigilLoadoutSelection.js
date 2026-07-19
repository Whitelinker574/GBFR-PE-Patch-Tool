function normalizeSelectedAddress(value) {
  const address = Number(value)
  return Number.isSafeInteger(address) && address > 0 ? address : 0
}

export function createSelectionTracker() {
  return {
    observedAddress: 0,
    handledAddresses: new Set(),
  }
}

export function takeSelectionAddress(tracker, value) {
  const address = normalizeSelectedAddress(value)
  const previousAddress = tracker.observedAddress
  tracker.observedAddress = address

  if (!address || address === previousAddress || tracker.handledAddresses.has(address)) return 0
  tracker.handledAddresses.add(address)
  return address
}

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'

const canvas = ref(null)
let observer

onMounted(() => {
  const element = canvas.value
  const context = element?.getContext('2d')
  if (!element || !context) return

  let width = 0
  let height = 0
  let stars = []

  function resize() {
    const ratio = Math.min(window.devicePixelRatio || 1, 2)
    width = element.clientWidth
    height = element.clientHeight
    element.width = Math.max(1, Math.floor(width * ratio))
    element.height = Math.max(1, Math.floor(height * ratio))
    context.setTransform(ratio, 0, 0, ratio, 0, 0)
    const count = Math.max(64, Math.floor((width * height) / 10500))
    stars = Array.from({ length: count }, (_, index) => ({
      x: ((index * 97.31) % 1000) / 1000 * width,
      y: ((index * 53.77 + 117) % 1000) / 1000 * height,
      radius: .35 + (index % 7) * .12,
      alpha: .18 + (index % 9) * .055,
    }))
    draw()
  }

  function draw() {
    context.clearRect(0, 0, width, height)
    for (const star of stars) {
      context.beginPath()
      context.fillStyle = `rgba(176, 230, 255, ${star.alpha})`
      context.arc(star.x, star.y, star.radius, 0, Math.PI * 2)
      context.fill()
      if (star.radius > .8) {
        context.fillStyle = `rgba(221, 195, 128, ${star.alpha * .32})`
        context.fillRect(star.x - 2.8, star.y, 5.6, .35)
        context.fillRect(star.x, star.y - 2.8, .35, 5.6)
      }
    }
  }

  observer = new ResizeObserver(resize)
  observer.observe(element)
  resize()
})

onBeforeUnmount(() => {
  observer?.disconnect()
})
</script>

<template><canvas ref="canvas" class="starfield" aria-hidden="true"></canvas></template>

<style scoped>
.starfield { position:absolute; inset:0; width:100%; height:100%; pointer-events:none; opacity:.9; }
</style>

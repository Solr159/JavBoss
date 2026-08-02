/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {},
  },
  plugins: [
    // Tailwind v3 没有内置 pointer-coarse 变体（v4 才有），这里手动注册：
    // 匹配触摸设备（移动端），用于播放器移动端贴边全屏样式
    ({ addVariant }) => {
      addVariant('pointer-coarse', '@media (pointer: coarse)')
    },
  ],
}

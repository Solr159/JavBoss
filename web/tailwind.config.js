/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        'ui-primary': 'var(--ui-primary)',
        'ui-primary-hover': 'var(--ui-primary-hover)',
        'ui-canvas': 'rgb(var(--ui-canvas) / <alpha-value>)',
        'ui-navigation': 'rgb(var(--ui-navigation) / <alpha-value>)',
        'ui-surface': 'rgb(var(--ui-surface) / <alpha-value>)',
        'ui-muted': 'rgb(var(--ui-muted) / <alpha-value>)',
        'ui-hover': 'rgb(var(--ui-hover) / <alpha-value>)',
        'ui-border': 'rgb(var(--ui-border) / <alpha-value>)',
        'ui-text': 'rgb(var(--ui-text) / <alpha-value>)',
        'ui-secondary': 'rgb(var(--ui-secondary) / <alpha-value>)',
        'ui-faint': 'rgb(var(--ui-faint) / <alpha-value>)',
        'ui-blue-surface': 'var(--theme-blue-surface)',
        'ui-blue-hover': 'var(--theme-blue-hover)',
        'ui-blue-border': 'var(--theme-blue-border)',
      },
    },
  },
  plugins: [],
}

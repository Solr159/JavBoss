/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        blue: {
          600: 'rgb(var(--theme-action-blue, 37 99 235) / <alpha-value>)',
          700: 'rgb(var(--theme-action-blue-hover, 29 78 216) / <alpha-value>)',
        },
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

module.exports = {
  root: true,
  env: { browser: true, es2022: true, node: true },
  extends: [
    'eslint:recommended',
    'plugin:react/recommended',
    'plugin:react-hooks/recommended',
    'plugin:jsx-a11y/recommended',
    'plugin:prettier/recommended',
  ],
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module', ecmaFeatures: { jsx: true } },
  settings: { react: { version: 'detect' } },
  rules: {
    'no-restricted-imports': [
      'error',
      {
        patterns: [
          {
            group: ['./*', '../*'],
            message: '前端内部模块请使用 @/ 开头的绝对路径导入。',
          },
        ],
      },
    ],
    'react/react-in-jsx-scope': 'off',
    'react/prop-types': 'off',
  },
  overrides: [
    {
      files: ['src/**/*.{js,jsx}'],
      excludedFiles: ['src/shared/ui/Tooltip.jsx'],
      rules: {
        'no-restricted-syntax': [
          'error',
          {
            selector:
              "ImportDeclaration[source.value='@mui/material'] > ImportSpecifier[imported.name='Tooltip']",
            message: '请使用 @/shared/ui/Tooltip，统一处理弹窗关闭后的焦点恢复和悬停行为。',
          },
          {
            selector: "ImportDeclaration[source.value='@mui/material/Tooltip']",
            message: '请使用 @/shared/ui/Tooltip，避免绕过 Tooltip 的交互规则。',
          },
        ],
      },
    },
    {
      files: [
        'src/App.jsx',
        'src/app/**/*.jsx',
        'src/shared/**/*.jsx',
        'src/features/*/components/**/*.jsx',
      ],
      // These existing containers own business updates; presentation components use actions.
      excludedFiles: [
        'src/features/settings/components/GlobalSettings.jsx',
        'src/features/tags/components/JavTagManager.jsx',
        'src/features/tags/components/VideoTagManager.jsx',
      ],
      rules: {
        'no-restricted-properties': [
          'error',
          {
            object: 'useStore',
            property: 'setState',
            message: '请通过业务模块或命名的 store action 更新状态，避免展示组件直接修改全局数据。',
          },
        ],
      },
    },
    {
      files: ['src/shared/**/*.{js,jsx}'],
      rules: {
        'no-restricted-imports': [
          'error',
          {
            patterns: [
              {
                group: ['./*', '../*'],
                message: '前端内部模块请使用 @/ 开头的绝对路径导入。',
              },
              {
                group: [
                  '@/App',
                  '@/App.*',
                  '@/app/**',
                  '@/features/**',
                  '@/store',
                  '@/store.*',
                  '@/state/**',
                  '@/api',
                  '@/api.*',
                  '@/api/**',
                  '@/auth',
                  '@/auth.*',
                  '@/routes/**',
                  '@/navigation/**',
                  '@/query/**',
                  '@/hooks/**',
                ],
                message: 'shared 只能依赖通用 UI 和工具，业务数据及行为请通过 props 传入。',
              },
            ],
          },
        ],
      },
    },
    {
      files: ['tests/**/*.js'],
      rules: {
        'no-restricted-imports': 'off',
      },
    },
  ],
  ignorePatterns: ['dist/'],
}

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
      files: ['src/App.jsx', 'src/components/**/*.jsx'],
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
      files: ['tests/**/*.js'],
      rules: {
        'no-restricted-imports': 'off',
      },
    },
  ],
  ignorePatterns: ['dist/'],
}

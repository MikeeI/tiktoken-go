export default [
    {
        ignores: ['bin/**', 'node_modules/**', '.eslintcache/**']
    },
    {
        files: ['*.js', '*.mjs', '*.cjs'],
        languageOptions: {
            ecmaVersion: 'latest',
            sourceType: 'module'
        },
        rules: {
            'no-unused-vars': 'error',
            'prefer-const': 'error',
            'no-var': 'error',
            eqeqeq: 'error',
            curly: 'error'
        }
    }
];

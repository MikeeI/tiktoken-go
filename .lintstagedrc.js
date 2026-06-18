import { existsSync, readFileSync } from 'node:fs';

const goModule =
    (existsSync('go.mod') ? readFileSync('go.mod', 'utf8') : '')
        .split('\n')
        .find(line => line.startsWith('module '))
        ?.replace('module ', '')
        .trim() ?? '';

const goimports = goModule ? `goimports -local ${goModule} -w` : 'goimports -w';

export default {
    '*.py': ['ruff check --fix --unsafe-fixes --quiet', 'ruff format --quiet'],
    '*.go': ['gofumpt -w', goimports],
    '*.{js,jsx,ts,tsx}': [
        'eslint --cache --cache-location .eslintcache --fix --quiet',
        'prettier --cache --write --log-level warn'
    ],
    '*.svelte': [
        'eslint --cache --cache-location .eslintcache --fix --quiet',
        'prettier --cache --write --log-level warn'
    ],
    'package.json': ['sort-package-json', 'prettier --cache --write --log-level warn'],
    '*.toml': ['taplo format'],
    '*.{json,md,yml,yaml,css,scss}': ['prettier --cache --write --log-level warn'],
    '*.sh': ['shfmt -w']
};

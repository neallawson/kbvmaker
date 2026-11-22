import path from 'node:path';
import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';

const repoRoot = path.resolve(fileURLToPath(new URL('.', import.meta.url)), '..');
const built = process.env.KBVM_BUILT || new Date().toISOString().replace(/\.\d+Z$/, 'Z');

export default defineConfig({
	define: {
		__KBVM_VERSION__: JSON.stringify(process.env.KBVM_VERSION || 'dev'),
		__KBVM_BUILT__: JSON.stringify(built)
	},
	server: {
		fs: {
			allow: [repoRoot]
		}
	},
	resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	test: {
		expect: { requireAssertions: true },
		environment: 'happy-dom',
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});

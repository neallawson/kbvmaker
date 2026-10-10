import { afterEach, describe, expect, it } from 'vitest';
import { createClient } from '#lib/montage/v1/client';
import { currentHost } from './host';

describe('currentHost', () => {
	afterEach(() => {
		delete (globalThis as { go?: unknown }).go;
	});

	it('is the browser when the window runtime is absent', () => {
		expect(currentHost().name).toBe('browser');
		expect(currentHost().pickMontage).toBeUndefined();
	});

	it('sends a window save through the API binding with its body', async () => {
		const calls: { method: string; path: string; body: string }[] = [];
		(globalThis as { go?: unknown }).go = {
			main: {
				Dialogs: {},
				API: {
					Call: async (method: string, path: string, body: string) => {
						calls.push({ method, path, body });
						return { status: 200, body: '{"ok":true}' };
					}
				}
			}
		};
		const host = currentHost();
		await createClient(host.fetch).save('/films/piece.montage.json', { montage: 1, slots: [] });
		expect(calls).toHaveLength(1);
		expect(calls[0].method).toBe('POST');
		expect(calls[0].path).toBe('/v1/document');
		expect(JSON.parse(calls[0].body)).toEqual({
			document: '/films/piece.montage.json',
			body: { montage: 1, slots: [] }
		});
	});

	it('turns an API rule reply into an error the editor shows', async () => {
		(globalThis as { go?: unknown }).go = {
			main: {
				Dialogs: {},
				API: {
					Call: async () => ({ status: 422, body: '{"slot":0,"code":"bad-number","message":"request body"}' })
				}
			}
		};
		await expect(createClient(currentHost().fetch).save('/x.montage.json', { montage: 1 })).rejects.toThrow(
			'request body'
		);
	});
});

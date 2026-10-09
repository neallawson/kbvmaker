import { describe, expect, it } from 'vitest';
import { createClient, RuleError } from './client';

function json(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

describe('Montage 1 client', () => {
	it('reads the API version', async () => {
		const seen: string[] = [];
		const client = createClient(async (url) => {
			seen.push(String(url));
			return json({ montage: 1 });
		}, 'http://127.0.0.1:8787');
		await expect(client.version()).resolves.toEqual({ montage: 1 });
		expect(seen).toEqual(['http://127.0.0.1:8787/v1/version']);
	});

	it('turns a rejected plan into a rule error', async () => {
		const client = createClient(async () =>
			json({ slot: 0, code: 'missing-media', message: 'document gone' }, 422)
		);
		await expect(client.plan('/gone.montage.json')).rejects.toMatchObject({
			name: 'RuleError',
			slot: 0,
			code: 'missing-media',
			message: 'document gone'
		});
		await expect(client.plan('/gone.montage.json')).rejects.toBeInstanceOf(RuleError);
	});
});

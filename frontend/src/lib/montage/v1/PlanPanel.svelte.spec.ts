import { cleanup, render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

afterEach(cleanup);
import PlanPanel from './PlanPanel.svelte';
import { RuleError, type Client } from './client';

function client(): Client {
	return {
		async version() {
			return { montage: 1 };
		},
		async audio() {
			return { seconds: 1 };
		},
		async plan(document) {
			if (document === 'missing.montage.json') {
				throw new RuleError(0, 'missing-media', 'document missing');
			}
			return { montage: 1, duration: 4, slots: [{ src: 'a.jpg', clipDur: 4 }] };
		},
		async planBody() {
			return { montage: 1, duration: 4, slots: [] };
		},
		async read() {
			return { montage: 1 };
		},
		async save() {},
		async exportScript() {},
		async render() {},
		async thumb() {
			return '';
		},
		async thumbCard() {
			return '';
		}
	};
}

describe('PlanPanel', () => {
	it('shows the Montage version and a plan summary', async () => {
		const user = userEvent.setup();
		render(PlanPanel, { client: client(), host: 'browser' });

		expect(await screen.findByText('Montage 1')).toBeTruthy();
		expect(screen.getByText('Host: browser')).toBeTruthy();

		await user.type(screen.getByRole('textbox'), 'piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'Show plan' }));
		expect(await screen.findByText('1 picture, 4s')).toBeTruthy();
	});

	it('shows a rule error from the API', async () => {
		const user = userEvent.setup();
		render(PlanPanel, { client: client(), host: 'browser' });
		await user.type(screen.getByRole('textbox'), 'missing.montage.json');
		await user.click(screen.getByRole('button', { name: 'Show plan' }));
		expect((await screen.findByRole('alert')).textContent).toContain('document missing');
	});

	it('fills the path from the window dialog', async () => {
		const user = userEvent.setup();
		render(PlanPanel, {
			client: client(),
			host: 'window',
			pickMontage: async () => 'album.montage.json'
		});

		await user.click(await screen.findByRole('button', { name: 'Open' }));
		expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe('album.montage.json');
	});
});

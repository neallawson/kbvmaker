import { cleanup, render, screen, within } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import type { Client, MontageFile } from './client';
import Editor from './Editor.svelte';
import type { Host } from '#lib/platform/host';

afterEach(cleanup);

const browser: Host = { name: 'browser' };

function client(read?: MontageFile): { client: Client; plans: MontageFile[] } {
	const plans: MontageFile[] = [];
	return {
		plans,
		client: {
			async version() {
				return { montage: 1 };
			},
			async audio() {
				return { seconds: 222 };
			},
			async plan() {
				throw new Error('unused');
			},
			async planBody(body) {
				plans.push({
					montage: 1,
					audio: body.audio,
					timing: body.timing,
					slots: body.slots?.map((slot) => ({ src: slot.src, mode: slot.mode, dur: slot.dur }))
				});
				return {
					montage: 1,
					duration: 20,
					slots: (body.slots ?? []).map((slot, index) => ({
						src: slot.src,
						clipStart: index * 9,
						clipDur: slot.dur ?? 10
					}))
				};
			},
			async read() {
				if (!read) {
					throw new Error('no file');
				}
				return {
					montage: 1,
					audio: read.audio,
					timing: read.timing,
					slots: read.slots?.map((slot) => ({ ...slot }))
				};
			},
			async save() {},
			async exportScript() {},
			async render() {}
		}
	};
}

describe('Editor', () => {
	it('starts a Classic project', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });

		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));

		expect(screen.getByText('Unsaved changes')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Classic' }).getAttribute('aria-pressed')).toBe('true');
		expect(screen.getByText('Each picture shows up here as you add it.')).toBeTruthy();
	});

	it('shows a picture on the score before a song is chosen', async () => {
		const user = userEvent.setup();
		render(Editor, { client: client().client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		expect(
			await within(screen.getByRole('group', { name: 'Score' })).findByRole('button', { name: 'a.jpg' })
		).toBeTruthy();
	});

	it('shows the song length beside its name', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Song file' }), '/films/song.mp3');
		await user.click(screen.getByRole('button', { name: 'Add song' }));
		expect(await screen.findByText('3:42 · 222s')).toBeTruthy();
	});

	it('does not ask for a file until Save', async () => {
		const user = userEvent.setup();
		let asked = false;
		const host: Host = {
			name: 'window',
			pickSaveMontage: async () => {
				asked = true;
				return '/films/piece.montage.json';
			}
		};
		render(Editor, { client: client().client, host });

		await user.click(screen.getByRole('button', { name: 'New' }));
		expect(asked).toBe(false);
		expect(screen.getByText('Unsaved changes')).toBeTruthy();

		await user.click(screen.getByRole('button', { name: 'Save' }));
		expect(asked).toBe(true);
		expect(screen.getByText('Saved')).toBeTruthy();
	});

	it('shows the score and stores a typed length on that picture only', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			slots: [
				{ src: 'a.jpg', mode: 'in-center' },
				{ src: 'b.jpg', mode: 'out-center' }
			]
		});
		render(Editor, { client: api.client, host: browser });

		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'Open' }));

		const score = await within(await screen.findByRole('group', { name: 'Score' })).findByRole('button', {
			name: 'b.jpg'
		});
		await user.click(score);

		const length = screen.getByRole('textbox', { name: 'Length in seconds' }) as HTMLInputElement;
		await user.clear(length);
		await user.type(length, '40');
		await user.tab();

		await screen.findByRole('button', { name: 'Share the rest' });
		const last = api.plans.at(-1);
		expect(last?.slots?.[1].dur).toBe(40);
		expect(last?.slots?.[0].dur).toBeUndefined();
	});

	it('locks the editor and shows progress beside Render until the video is written', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			slots: [{ src: 'a.jpg', mode: 'in-center' }]
		});
		let finish = () => {};
		api.client.render = () => new Promise<void>((resolve) => (finish = resolve));
		const host: Host = {
			name: 'window',
			pickMontage: async () => '/films/piece.montage.json',
			pickSaveVideo: async () => '/films/out.mp4'
		};
		render(Editor, { client: api.client, host });
		await user.click(screen.getByRole('button', { name: 'Open' }));
		await user.click(await screen.findByRole('button', { name: 'Render video' }));

		expect((await screen.findByRole('status')).textContent).toContain('Rendering…');
		expect((screen.getByRole('button', { name: 'Render video' }) as HTMLButtonElement).matches(':disabled')).toBe(
			true
		);
		expect((screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement).matches(':disabled')).toBe(true);

		finish();
		expect(await screen.findByText(/Wrote out\.mp4 in 0:0\d/)).toBeTruthy();
		expect((screen.getByRole('button', { name: 'Render video' }) as HTMLButtonElement).matches(':disabled')).toBe(
			false
		);
	});
});

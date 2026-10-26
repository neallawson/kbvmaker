import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { tick } from 'svelte';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import { RuleError, type Client, type MontageFile } from './client';
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
					slots: body.slots?.map((slot) => ({
						src: slot.src,
						card: slot.card,
						mode: slot.mode,
						dur: slot.dur,
						fixed: slot.fixed
					}))
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
					defaults: read.defaults,
					moves: read.moves,
					slots: read.slots?.map((slot) => ({ ...slot }))
				};
			},
			async save() {},
			async exportScript() {},
			async render() {},
			async thumb() {
				return 'data:image/jpeg;base64,thumb';
			},
			async thumbCard() {
				return 'data:image/jpeg;base64,cardthumb';
			}
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

		expect(screen.getByRole('heading', { level: 1, name: 'kbvmaker - Montage (v1) editor, renderer' })).toBeTruthy();
		expect(screen.getByRole('heading', { level: 1 }).parentElement?.textContent).not.toContain('Unsaved changes');
		const buttons = screen.getByRole('button', { name: 'New' }).closest('.grid');
		expect(buttons?.textContent).toContain('Unsaved changes');
		expect(screen.getByRole('button', { name: 'Classic' }).getAttribute('aria-pressed')).toBe('true');
		const title = screen.getByRole('heading', { level: 1 });
		for (const heading of [title, screen.getByRole('heading', { name: 'Look' }), screen.getByRole('heading', { name: 'Pictures' }), screen.getByRole('heading', { name: 'Render' })]) {
			expect(heading.className).not.toContain('border-b');
		}
		expect(screen.getByRole('tab', { name: 'Soundtrack', selected: true })).toBeTruthy();
		expect(screen.getByRole('tab', { name: 'Soundtrack' }).closest('section')?.className).toContain('border-b');
		expect(title.closest('section')?.className).toContain('border-b');
		expect(screen.getByRole('heading', { name: 'Pictures' }).closest('section')?.className).toContain('border-b');
		expect(screen.getByRole('heading', { name: 'Render' }).closest('section')?.className).not.toContain('border-b');
		expect(screen.getByText('Timeline').className).not.toContain('border-b');
		expect(screen.getByText('Each picture shows up here as you add it.')).toBeTruthy();
		expect(document.querySelectorAll('[data-timeline] .film-rail')).toHaveLength(2);
	});

	it('uses the window width and wraps a long file name', async () => {
		const user = userEvent.setup();
		render(Editor, { client: client().client, host: browser });
		const main = screen.getByRole('heading', { level: 1 }).closest('main');
		expect(main?.className).toContain('w-full');
		expect(main?.className).toContain('p-4');
		expect(main?.className).not.toContain('max-w-');

		const name = `${'wedding-photo-'.repeat(8)}final.jpg`;
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		const status = screen.getByText('Unsaved changes');
		expect(status.getAttribute('data-aside')).toBe('');
		expect(status.className).not.toContain('ml-auto');
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), `/films/${name}`);
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		const row = await screen.findByRole('listitem');
		const label = within(row).getByRole('button', { name: new RegExp(`^1\\. ${name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) });
		expect(label.className).toContain('min-w-0');
		expect(label.querySelector('span')?.className).toContain('break-all');
		expect(screen.getByText('/films/piece.montage.json').className).toContain('break-all');
	});

	it('offers every move when changing one picture', async () => {
		const user = userEvent.setup();
		render(Editor, { client: client().client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));

		const everyMove = [
			'Slow zoom in',
			'Pan right',
			'Pan left',
			'Drift down',
			'Drift up',
			'Slow zoom out',
			'Hold still'
		];
		const options = () => [...(screen.getByRole('combobox') as HTMLSelectElement).options].map((option) => option.text);
		expect(options()).toEqual(everyMove);

		await user.click(screen.getByRole('button', { name: 'Photo film' }));
		expect(options()).toEqual(everyMove);

		await user.click(screen.getByRole('button', { name: 'Quick slides' }));
		expect(options()).toEqual(everyMove);
	});

	it('keeps every move available when the file stores a shorter shuffle list', async () => {
		const user = userEvent.setup();
		render(Editor, {
			client: client({
				montage: 1,
				audio: 'song.mp3',
				moves: ['in-center', 'in-right-left'],
				slots: [{ src: 'a.jpg', mode: 'in-right-left' }]
			}).client,
			host: browser
		});
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'Open' }));
		const move = (await screen.findByRole('combobox')) as HTMLSelectElement;
		expect([...move.options].map((option) => option.text)).toEqual([
			'Slow zoom in',
			'Pan right',
			'Pan left',
			'Drift down',
			'Drift up',
			'Slow zoom out',
			'Hold still'
		]);
		expect(move.value).toBe('in-right-left');
	});

	it('shows a picture on the timeline before a song is chosen', async () => {
		const user = userEvent.setup();
		render(Editor, { client: client().client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		const mark = await within(screen.getByRole('group', { name: 'Timeline' })).findByRole('button', {
			name: '1. a.jpg'
		});
		expect(mark.getAttribute('title')).toBe('a.jpg');
		expect(mark.textContent?.trim()).toBe('1');
		await waitFor(() => {
			expect(mark.querySelector('img')?.getAttribute('src')).toBe('data:image/jpeg;base64,thumb');
		});
		const row = screen.getByRole('listitem');
		expect(within(row).getByRole('button', { name: /^1\. a\.jpg/ }).querySelector('img')?.getAttribute('src')).toBe(
			'data:image/jpeg;base64,thumb'
		);
	});

	it('shows the song length beside its name', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Song file' }), '/films/song.mp3');
		await user.click(screen.getByRole('button', { name: 'Add soundtrack' }));
		expect(await screen.findByText('3:42 · 222s')).toBeTruthy();
		expect(screen.queryByRole('textbox', { name: 'Time for each picture' })).toBeNull();
	});

	it('shows the finished length and the time for each picture without a song', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		expect(screen.queryByRole('textbox', { name: 'Time for each picture' })).toBeNull();

		await user.click(screen.getByRole('tab', { name: 'No Soundtrack' }));
		expect(screen.getByText('0:05 · 5s')).toBeTruthy();
		const each = screen.getByRole('textbox', { name: 'Time for each picture' }) as HTMLInputElement;
		expect(each.value).toBe('5');
		expect(each.disabled).toBe(false);

		await user.clear(each);
		await user.type(each, '8');
		await user.tab();
		expect(screen.getByText('0:08 · 8s')).toBeTruthy();

		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/b.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		expect(await screen.findByText('0:20 · 20s')).toBeTruthy();
		expect((screen.getByRole('textbox', { name: 'Time for each picture' }) as HTMLInputElement).value).toBe('8');
		await waitFor(() => {
			expect(api.plans.at(-1)?.timing).toEqual({ policy: 'fixed-clip', clip: 8 });
		});
	});

	it('gives one picture its own time without a song and leaves the other at the pace', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/b.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));

		const length = (await screen.findByRole('textbox', { name: 'Length in seconds' })) as HTMLInputElement;
		expect(length.disabled).toBe(false);
		await user.clear(length);
		await user.type(length, '40');
		await user.tab();
		await screen.findByRole('button', { name: 'Remove custom time' });
		await waitFor(() => {
			const last = api.plans.at(-1);
			expect(last?.audio).toBeUndefined();
			expect(last?.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
			expect(last?.slots?.[0].dur).toBe(40);
			expect(last?.slots?.[1].dur).toBeUndefined();
		});
		await user.click(screen.getByRole('tab', { name: 'No Soundtrack' }));
		expect(screen.getByText('Film length')).toBeTruthy();
		expect((screen.getByRole('textbox', { name: 'Time for each picture' }) as HTMLInputElement).value).toBe('5');
	});

	it('lets the song take the clock from Classic and keeps Quick slides', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.type(screen.getByRole('textbox', { name: 'Song file' }), '/films/song.mp3');
		await user.click(screen.getByRole('button', { name: 'Add soundtrack' }));
		await waitFor(() => {
			expect(api.plans.at(-1)?.timing).toEqual({ policy: 'fit-audio' });
			expect(api.plans.at(-1)?.audio).toBe('/films/song.mp3');
		});

		await screen.findByText('0:20');
		await user.click(screen.getByRole('tab', { name: 'No Soundtrack' }));
		await waitFor(() => {
			const last = api.plans.at(-1);
			expect(last?.audio).toBeUndefined();
			expect(last?.timing).toEqual({ policy: 'fixed-clip', clip: 10 });
		});
		expect(screen.getByRole('tab', { name: 'No Soundtrack', selected: true })).toBeTruthy();

		await user.click(screen.getByRole('button', { name: 'Quick slides' }));
		await user.click(screen.getByRole('tab', { name: 'Soundtrack' }));
		await user.type(screen.getByRole('textbox', { name: 'Song file' }), '/films/song.mp3');
		await user.click(screen.getByRole('button', { name: 'Add soundtrack' }));
		await waitFor(() => {
			expect(api.plans.at(-1)?.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		});
		expect((await screen.findByRole('textbox', { name: 'Length in seconds' })) as HTMLInputElement).toHaveProperty(
			'disabled',
			true
		);
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

		const timeline = await screen.findByRole('group', { name: 'Timeline' });
		expect(await within(timeline).findByText('0:00')).toBeTruthy();
		const asides = document.querySelectorAll('[data-aside]');
		expect(asides).toHaveLength(2);
		expect(within(timeline).getByText('0:20')).toBeTruthy();
		expect(timeline.querySelectorAll('[data-join]')).toHaveLength(1);
		const score = await within(timeline).findByRole('button', { name: '2. b.jpg' });
		await user.click(score);

		const length = screen.getByRole('textbox', { name: 'Length in seconds' }) as HTMLInputElement;
		await user.clear(length);
		await user.type(length, '40');
		await user.tab();

		await screen.findByRole('button', { name: 'Remove custom time' });
		for (const label of ['Move up', 'Move down', 'Replace', 'Remove']) {
			const button = screen.getByRole('button', { name: label });
			expect(button.querySelector('svg')).toBeTruthy();
			expect(button.textContent?.trim()).toBe('');
			expect(button.parentElement?.querySelector('[role="tooltip"]')?.textContent).toBe(label);
		}
		expect((screen.getByRole('button', { name: 'Move down' }) as HTMLButtonElement).disabled).toBe(true);
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
		const renderVideo = await screen.findByRole('button', { name: 'Render video' });
		await waitFor(() => expect((renderVideo as HTMLButtonElement).disabled).toBe(false));
		await user.click(renderVideo);

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

	it('keeps a render failure on screen after the plan refreshes', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			slots: [{ src: 'a.jpg', mode: 'in-center' }]
		});
		const pending: Array<() => void> = [];
		let holdPlans = false;
		const plan = api.client.planBody.bind(api.client);
		api.client.planBody = (body, dir) => {
			const run = () => plan(body, dir);
			if (!holdPlans) {
				return run();
			}
			return new Promise((resolve) => {
				pending.push(() => {
					void run().then(resolve);
				});
			});
		};
		api.client.render = async () => {
			throw new RuleError(0, 'render', 'clip: ffmpeg failed');
		};
		const host: Host = {
			name: 'window',
			pickMontage: async () => '/films/piece.montage.json',
			pickSaveVideo: async () => '/films/out.mp4'
		};
		render(Editor, { client: api.client, host });
		await user.click(screen.getByRole('button', { name: 'Open' }));
		const renderVideo = await screen.findByRole('button', { name: 'Render video' });
		await waitFor(() => expect((renderVideo as HTMLButtonElement).disabled).toBe(false));
		holdPlans = true;
		await user.click(renderVideo);

		expect((await screen.findByRole('alert')).textContent).toContain('ffmpeg failed');
		for (const release of pending) {
			release();
		}
		await tick();
		expect(screen.getByRole('alert').textContent).toContain('ffmpeg failed');
	});

	it('does not start a render when the soundtrack is missing', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			slots: [{ src: 'a.jpg', mode: 'in-center' }]
		});
		api.client.planBody = async () => {
			throw new RuleError(0, 'missing-media', 'audio song.mp3');
		};
		let asked = false;
		const host: Host = {
			name: 'window',
			pickMontage: async () => '/films/piece.montage.json',
			pickSaveVideo: async () => {
				asked = true;
				return '/films/out.mp4';
			}
		};
		render(Editor, { client: api.client, host });
		await user.click(screen.getByRole('button', { name: 'Open' }));

		expect((await screen.findByRole('alert')).textContent).toContain('audio song.mp3');
		const renderVideo = screen.getByRole('button', { name: 'Render video' }) as HTMLButtonElement;
		const exportScript = screen.getByRole('button', { name: 'Export shell script' }) as HTMLButtonElement;
		expect(renderVideo.disabled).toBe(true);
		expect(exportScript.disabled).toBe(true);
		await user.click(renderVideo);
		expect(asked).toBe(false);
	});

	it('reports every missing picture when a montage is opened', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			slots: [
				{ src: 'a.jpg', mode: 'in-center' },
				{ src: 'b.jpg', mode: 'out-center' },
				{ src: 'c.jpg', mode: 'in-center' }
			]
		});
		api.client.planBody = async () => {
			throw new RuleError(0, 'missing-media', 'Missing picture 2: b.jpg\nMissing picture 3: c.jpg');
		};
		const host: Host = {
			name: 'window',
			pickMontage: async () => '/films/piece.montage.json'
		};
		render(Editor, { client: api.client, host });
		await user.click(screen.getByRole('button', { name: 'Open' }));

		const alert = await screen.findByRole('alert');
		expect(alert.textContent).toContain('Missing picture 2: b.jpg');
		expect(alert.textContent).toContain('Missing picture 3: c.jpg');
		expect(screen.getByRole('heading', { name: 'Render' }).parentElement?.contains(alert)).toBe(true);
		expect((screen.getByRole('button', { name: 'Render video' }) as HTMLButtonElement).disabled).toBe(true);
	});

	it('re-scans images and audio from the button and when the window returns', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			slots: [{ src: 'a.jpg', mode: 'in-center' }]
		});
		let plans = 0;
		let thumbs = 0;
		const plan = api.client.planBody.bind(api.client);
		api.client.planBody = async (body, dir) => {
			plans += 1;
			if (plans === 1) {
				throw new RuleError(0, 'missing-media', 'Missing picture 1: a.jpg');
			}
			return plan(body, dir);
		};
		api.client.thumb = async () => {
			thumbs += 1;
			return `data:image/jpeg;base64,${thumbs}`;
		};
		const host: Host = {
			name: 'window',
			pickMontage: async () => '/films/piece.montage.json'
		};
		render(Editor, { client: api.client, host });
		await user.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByRole('alert')).toHaveProperty('textContent', expect.stringContaining('Missing picture 1: a.jpg'));
		await waitFor(() => expect(thumbs).toBeGreaterThan(0));
		const thumbsBefore = thumbs;
		await user.click(screen.getByRole('button', { name: 'Re-scan Pictures/Audio' }));
		await waitFor(() => expect(plans).toBe(2));
		await waitFor(() => expect(thumbs).toBeGreaterThan(thumbsBefore));
		await waitFor(() => expect((screen.getByRole('button', { name: 'Render video' }) as HTMLButtonElement).disabled).toBe(false));

		const plansBeforeFocus = plans;
		window.dispatchEvent(new Event('focus'));
		await waitFor(() => expect(plans).toBe(plansBeforeFocus + 1));
	});

	it('drops a picture into a new place on the timeline', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/b.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));

		const timeline = screen.getByRole('group', { name: 'Timeline' });
		stubTrack();
		await within(timeline).findByRole('button', { name: 'Change the lengths beside pictures 1 and 2' });
		const moving = within(timeline).getByRole('button', { name: '2. b.jpg' });
		fireEvent.pointerDown(moving, { clientX: 150, clientY: 8, button: 0, pointerId: 1 });
		fireEvent.pointerMove(window, { clientX: 10, clientY: 8, button: 0, pointerId: 1 });
		fireEvent.pointerUp(window, { clientX: 10, clientY: 8, button: 0, pointerId: 1 });
		await tick();

		expect(pictureLabels(timeline)).toEqual(['1. b.jpg', '2. a.jpg']);
		expect(screen.getByText('Unsaved changes')).toBeTruthy();
		await waitFor(() => {
			expect(api.plans.at(-1)?.slots?.map((slot) => slot.src)).toEqual(['/films/b.jpg', '/films/a.jpg']);
		});
	});

	it('lifts a picture and opens a gap while it is dragged', async () => {
		const user = userEvent.setup();
		render(Editor, { client: client().client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/b.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));

		const timeline = screen.getByRole('group', { name: 'Timeline' });
		stubTrack();
		await within(timeline).findByRole('button', { name: 'Change the lengths beside pictures 1 and 2' });
		const moving = within(timeline).getByRole('button', { name: '2. b.jpg' });
		fireEvent.pointerDown(moving, { clientX: 150, clientY: 8, button: 0 });
		fireEvent.pointerMove(window, { clientX: 10, clientY: 8, button: 0 });
		await waitFor(() => {
			expect(document.querySelector('[data-lift]')).toBeTruthy();
		});
		const held = document.querySelector('[data-lift]') as HTMLElement;
		expect(held.className).toContain('shadow-');
		expect(held.textContent).toContain('2');
		expect(parseFloat(within(timeline).getByRole('button', { name: '1. a.jpg' }).style.left)).toBeCloseTo(45);
		expect(within(timeline).queryByRole('button', { name: '2. b.jpg' })).toBeNull();
		fireEvent.pointerUp(window, { clientX: 10, clientY: 8, button: 0 });
		await tick();
		expect(document.querySelector('[data-lift]')).toBeNull();
		expect(pictureLabels(timeline)).toEqual(['1. b.jpg', '2. a.jpg']);
	});

	it('selects a picture when the press does not travel', async () => {
		const user = userEvent.setup();
		render(Editor, { client: client().client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/a.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.type(screen.getByRole('textbox', { name: 'Picture file' }), '/films/b.jpg');
		await user.click(screen.getByRole('button', { name: 'Add pictures' }));
		await user.click(within(screen.getByRole('group', { name: 'Timeline' })).getByRole('button', { name: '2. b.jpg' }));
		expect(screen.getByRole('button', { name: '2. b.jpg', pressed: true })).toBeTruthy();
	});

	it('drags a dissolve grip to give time to one picture and take it from the next', async () => {
		const user = userEvent.setup();
		const api = client({
			montage: 1,
			audio: 'song.mp3',
			timing: { policy: 'fit-audio' },
			defaults: { join: { type: 'fade', duration: 1 } },
			slots: [
				{ src: 'a.jpg', mode: 'in-center' },
				{ src: 'b.jpg', mode: 'out-center' }
			]
		});
		let saved = false;
		api.client.save = async () => {
			saved = true;
		};
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'Open' }));

		const timeline = await screen.findByRole('group', { name: 'Timeline' });
		await within(timeline).findByText('0:00');
		stubTrack();
		const grip = within(timeline).getByRole('button', { name: 'Change the lengths beside pictures 1 and 2' });
		expect(grip.style.width).toBe('28px');
		expect(timeline.querySelector('[data-grip-mark]')?.className).toContain('w-0.5');
		fireEvent.pointerDown(grip, { clientX: 0, clientY: 8, button: 0 });
		fireEvent.pointerMove(window, { clientX: 20, clientY: 8, button: 0 });
		await waitFor(() => {
			expect(parseFloat(within(timeline).getByRole('button', { name: '1. a.jpg' }).style.width)).toBeGreaterThan(50);
		});
		const draggedWidth = parseFloat(within(timeline).getByRole('button', { name: '1. a.jpg' }).style.width);
		fireEvent.pointerUp(window, { clientX: 20, clientY: 8, button: 0 });
		await tick();
		expect(parseFloat(within(timeline).getByRole('button', { name: '1. a.jpg' }).style.width)).toBeCloseTo(
			draggedWidth
		);

		await waitFor(() => {
			expect(api.plans.at(-1)?.slots?.map((slot) => slot.dur)).toEqual([12, undefined]);
		});
		expect(parseFloat(within(timeline).getByRole('button', { name: '1. a.jpg' }).style.width)).toBeCloseTo(
			draggedWidth
		);
		expect(screen.getByText('Unsaved changes')).toBeTruthy();
		expect(saved).toBe(false);
		expect(screen.getByRole('button', { name: 'Remove custom time' })).toBeTruthy();
	});

	it('leaves the center of a short picture free of the grips', async () => {
		const user = userEvent.setup();
		render(Editor, {
			client: client({
				montage: 1,
				audio: 'song.mp3',
				timing: { policy: 'fit-audio' },
				slots: [
					{ src: 'a.jpg', mode: 'in-center', dur: 10 },
					{ src: 'b.jpg', mode: 'out-center', dur: 2 },
					{ src: 'c.jpg', mode: 'in-center', dur: 10 }
				]
			}).client,
			host: browser
		});
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'Open' }));
		const timeline = await screen.findByRole('group', { name: 'Timeline' });
		await within(timeline).findByText('0:00');
		stubTrack();
		window.dispatchEvent(new Event('resize'));
		await tick();
		const picture = within(timeline).getByRole('button', { name: '2. b.jpg' });
		const pictureLeft = (parseFloat(picture.style.left) / 100) * 200;
		const grip = within(timeline).getByRole('button', { name: 'Change the lengths beside pictures 1 and 2' });
		expect(grip.style.left.endsWith('px')).toBe(true);
		const gripRight = parseFloat(grip.style.left) + parseFloat(grip.style.width);
		expect(gripRight).toBeLessThanOrEqual(pictureLeft + 0.01);
	});

	it('adds a title card you can edit, and keeps Hold still under a Look', async () => {
		const user = userEvent.setup();
		const api = client();
		render(Editor, { client: api.client, host: browser });
		await user.type(screen.getByRole('textbox', { name: 'Project file' }), '/films/piece.montage.json');
		await user.click(screen.getByRole('button', { name: 'New' }));
		await user.click(screen.getByRole('button', { name: 'Add card' }));

		expect(await screen.findByRole('button', { name: '1. Title' })).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Replace' })).toBeNull();
		const title = screen.getByRole('textbox', { name: 'Title' }) as HTMLInputElement;
		expect(title.value).toBe('Title');
		const plansBefore = api.plans.length;
		await user.clear(title);
		await user.type(title, 'Opening');
		expect(title.value).toBe('Opening');
		expect(screen.getByRole('button', { name: '1. Title' })).toBeTruthy();
		expect(api.plans.length).toBe(plansBefore);

		await user.tab();
		expect(screen.getByRole('button', { name: '1. Opening' })).toBeTruthy();
		expect(api.plans.at(-1)?.slots?.[0].card).toMatchObject({ title: 'Opening' });

		const textColor = screen.getByLabelText('Text color') as HTMLInputElement;
		const background = screen.getByLabelText('Background color') as HTMLInputElement;
		expect(textColor.type).toBe('color');
		expect(background.type).toBe('color');
		expect(textColor.closest('span')?.className).toContain('border');
		expect(textColor.parentElement?.textContent).toContain('#FFFFFF');
		expect(background.parentElement?.textContent).toContain('#000000');

		await user.click(screen.getByRole('button', { name: 'Photo film' }));
		const move = screen.getByRole('combobox', { name: 'Move' }) as HTMLSelectElement;
		expect(move.value).toBe('hold');
		await user.click(screen.getByRole('button', { name: 'Pin position' }));
		await waitFor(() => expect(api.plans.at(-1)?.slots?.[0].fixed).toBe(true));
		await user.click(screen.getByRole('button', { name: 'Unpin position' }));
		await waitFor(() => expect(api.plans.at(-1)?.slots?.[0].fixed).toBeUndefined());

		expect(api.plans.at(-1)?.slots?.[0]).toMatchObject({
			card: { title: 'Opening' },
			mode: 'hold'
		});
	});
});

function pictureLabels(timeline: HTMLElement): (string | null)[] {
	return within(timeline)
		.getAllByRole('button')
		.map((button) => button.getAttribute('aria-label'))
		.filter((label) => /^\d+\. /.test(label ?? ''));
}

function stubTrack() {
	const track = document.querySelector('[data-timeline]') as HTMLElement;
	track.getBoundingClientRect = () =>
		({
			left: 0,
			top: 0,
			width: 200,
			height: 64,
			right: 200,
			bottom: 64,
			x: 0,
			y: 0,
			toJSON() {
				return {};
			}
		}) as DOMRect;
}

<script lang="ts">
	import { untrack } from 'svelte';
	import { Button } from '#lib/components/ui/button';
	import { Card, CardContent } from '#lib/components/ui/card';
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '#lib/components/ui/tabs';
	import type { Host } from '#lib/platform/host';
	import { RuleError, type Card as TitleCard, type Client, type MontageFile, type Plan, type Slot } from './client';
	import {
		addCard,
		addPictures,
		applyLook,
		baseName,
		blocksFromLengths,
		canPlan,
		clipFromFilm,
		clearAudio,
		dragLayout,
		dropIndex,
		gripHit,
		settledLayout,
		joinSeconds,
		moveChoices,
		moveSlot,
		newProject,
		mediaPath,
		outputLength,
		projectDir,
		relativize,
		removeSlot,
		reorderSlot,
		replaceSrc,
		resizePictures,
		resizedLengths,
		scoreBlocks,
		scoreJoins,
		secondsLabel,
		setAudio,
		setCard,
		setTimeEach,
		slotLabel,
		songClock,
		setDur,
		setFixed,
		setMode,
		shuffleMoves,
		shuffleSlots,
		sortByName,
		isCard,
		type LookId
	} from './document';
	import { describePlan } from './summary';

	let { client, host }: { client: Client; host: Host } = $props();

	let filePath = $state('');
	let pathDraft = $state('');
	let doc = $state<MontageFile | null>(null);
	let look = $state<LookId | null>(null);
	/** clockTab is the open clock. Soundtrack is where a new project starts. */
	let clockTab = $state<'soundtrack' | 'silent'>('soundtrack');
	let selected = $state(0);
	let dirty = $state(false);
	let plan = $state<Plan | null>(null);
	let error = $state('');
	let errorSlot = $state(0);
	/** errorKind is plan while a plan failure is showing. A later plan may clear that, and leaves a render or save failure alone. */
	let errorKind = $state<'plan' | 'action' | ''>('');
	let busy = $state('');
	let working = $state(false);
	let elapsed = $state(0);
	let songDraft = $state('');
	let songSeconds = $state<number | null>(null);
	let pictureDraft = $state('');
	let lengthDraft = $state<string | null>(null);
	/** cardDraft holds title text until the field is left, so the thumbnail is not painted on every key. */
	let cardDraft = $state<{ title: string; subtitle: string; line: string; credits: string } | null>(null);
	let eachDraft = $state<string | null>(null);
	let thumbs = $state<Record<string, string>>({});
	let trackEl = $state<HTMLDivElement | null>(null);
	let trackWidth = $state(0);
	let preview = $state<{ lengths: number[]; dissolve: number } | null>(null);
	/** gripCommit keeps the drag on screen until the plan draws those same blocks. */
	let gripCommit = $state(false);
	/** lift is the picture following the pointer, and the gap where it will land. */
	let lift = $state<{ index: number; to: number; left: number; width: number } | null>(null);
	/** reorderHold keeps the landed order on screen until the plan draws it. */
	let reorderHold = $state<ReturnType<typeof settledLayout> | null>(null);
	let dragArmed = $state(false);
	let swallowClick = false;
	/** scan bumps when the soundtrack and pictures should be read from disk again. */
	let scan = $state(0);
	let thumbGen = $state(0);

	const title = 'kbvmaker - Montage (v1) editor, renderer';
	const thumbWidth = 320;
	const thumbHeight = 180;
	const gripWidth = 28;
	const dragSlop = 4;
	const cluster =
		'grid w-full max-w-6xl grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-6 gap-y-4';
	const rule = 'border-b border-foreground/20 pb-2';

	const looks: { id: LookId; label: string }[] = [
		{ id: 'classic', label: 'Classic' },
		{ id: 'photo-film', label: 'Photo film' },
		{ id: 'quick-slides', label: 'Quick slides' }
	];

	$effect(() => {
		const current = doc;
		const dir = projectDir(filePath);
		void scan;
		if (!current || !canPlan(current)) {
			plan = null;
			return;
		}
		let live = true;
		client
			.planBody(current, dir)
			.then((next) => {
				if (!live) {
					return;
				}
				plan = next;
				if (reorderHold && doc?.slots && sameBlocks(scoreBlocks(doc.slots, next), reorderHold)) {
					reorderHold = null;
				}
				if (gripCommit && preview && doc?.slots) {
					const fromPlan = scoreBlocks(doc.slots, next);
					const fromDrag = blocksFromLengths(
						doc.slots.map((slot) => slot.src),
						preview.lengths,
						preview.dissolve
					);
					if (sameBlocks(fromPlan, fromDrag)) {
						preview = null;
						gripCommit = false;
					}
				}
				if (current.audio && next.song != null) {
					songSeconds = next.song;
				}
				if (errorKind === 'plan') {
					clearError();
				}
			})
			.catch((err: unknown) => {
				if (!live) {
					return;
				}
				plan = null;
				reorderHold = null;
				if (err instanceof RuleError) {
					showError(err.message, err.slot, 'plan');
				} else {
					showError('Could not read the plan', 0, 'plan');
				}
			});
		return () => {
			live = false;
		};
	});

	const thumbInflight = new Set<string>();

	$effect(() => {
		const generation = thumbGen;
		const dir = projectDir(filePath);
		const slots = doc?.slots ?? [];
		const pending = untrack(() => {
			const next: { key: string; slot: Slot }[] = [];
			const seen = new Set<string>();
			for (const slot of slots) {
				const key = thumbKey(slot, dir);
				if (!key || thumbs[key] || thumbInflight.has(key) || seen.has(key)) {
					continue;
				}
				seen.add(key);
				next.push({ key, slot });
			}
			return next;
		});
		for (const item of pending) {
			thumbInflight.add(item.key);
			const req = item.slot.card
				? client.thumbCard(item.slot.card, thumbWidth, thumbHeight)
				: client.thumb(thumbKey(item.slot, dir), thumbWidth, thumbHeight);
			void req
				.then((url) => {
					if (generation !== thumbGen) {
						return;
					}
					thumbs[item.key] = url;
				})
				.catch(() => {})
				.finally(() => {
					if (generation === thumbGen) {
						thumbInflight.delete(item.key);
					}
				});
		}
	});

	function thumbKey(slot: Slot, dir = projectDir(filePath)): string {
		if (slot.card) {
			return `card:${JSON.stringify(slot.card)}`;
		}
		return mediaPath(dir, slot.src ?? '');
	}

	function thumbForSlot(slot: Slot | undefined): string {
		return slot ? (thumbs[thumbKey(slot)] ?? '') : '';
	}

	/** checkFiles reads the soundtrack and pictures from disk and loads their thumbnails again. */
	function checkFiles() {
		if (!doc || working) {
			return;
		}
		const next: Record<string, string> = {};
		for (const [key, url] of Object.entries(thumbs)) {
			if (key.startsWith('card:')) {
				next[key] = url;
			}
		}
		thumbs = next;
		for (const key of [...thumbInflight]) {
			if (!key.startsWith('card:')) {
				thumbInflight.delete(key);
			}
		}
		thumbGen += 1;
		scan += 1;
	}

	$effect(() => {
		const onFocus = () => checkFiles();
		window.addEventListener('focus', onFocus);
		return () => window.removeEventListener('focus', onFocus);
	});

	function showError(message: string, slot = 0, kind: 'plan' | 'action' = 'action') {
		error = message;
		errorSlot = slot;
		errorKind = kind;
	}

	function clearError() {
		error = '';
		errorSlot = 0;
		errorKind = '';
	}

	function edit(next: MontageFile, holdPreview = false) {
		doc = next;
		dirty = true;
		lengthDraft = null;
		eachDraft = null;
		cardDraft = null;
		if (!holdPreview) {
			preview = null;
			gripCommit = false;
		}
		reorderHold = null;
	}

	function start(path: string) {
		filePath = path;
		doc = newProject();
		look = 'classic';
		clockTab = 'soundtrack';
		selected = 0;
		dirty = true;
		clearError();
		plan = null;
		reorderHold = null;
		lift = null;
		songSeconds = null;
		eachDraft = null;
	}

	async function newProjectFile() {
		// The window names the file on Save. New does not look for one.
		if (host.pickSaveMontage) {
			start('');
			return;
		}
		start(pathDraft);
	}

	async function openProject(path = pathDraft) {
		const chosen = host.pickMontage ? await host.pickMontage() : path;
		if (!chosen) {
			return;
		}
		try {
			doc = await client.read(chosen);
			filePath = chosen;
			look = null;
			selected = 0;
			dirty = false;
			clearError();
			lengthDraft = null;
			eachDraft = null;
			songSeconds = null;
			clockTab = doc.audio ? 'soundtrack' : 'silent';
			if (doc.audio) {
				void measureSong(doc.audio);
			}
		} catch (err) {
			showError(err instanceof RuleError ? err.message : 'Could not open the project');
		}
	}

	async function save(): Promise<boolean> {
		if (!doc) {
			return false;
		}
		try {
			if (!filePath) {
				const path = host.pickSaveMontage ? await host.pickSaveMontage() : pathDraft;
				if (!path) {
					return false;
				}
				filePath = path;
			}
			const stored = relativize(doc, projectDir(filePath));
			await client.save(filePath, stored);
			doc = stored;
			dirty = false;
			clearError();
			return true;
		} catch (err) {
			showError(err instanceof Error ? err.message : 'Could not save the project');
			return false;
		}
	}

	async function measureSong(path: string) {
		try {
			const info = await client.audio(mediaPath(projectDir(filePath), path));
			songSeconds = info.seconds;
		} catch (err) {
			songSeconds = null;
			showError(err instanceof RuleError ? err.message : 'Could not read the song');
			errorSlot = 0;
		}
	}

	async function addSong(path = songDraft) {
		const chosen = host.pickAudio ? await host.pickAudio() : path;
		if (!chosen || !doc) {
			return;
		}
		edit(setAudio(doc, chosen, look));
		clockTab = 'soundtrack';
		await measureSong(chosen);
	}

	function showSoundtrack() {
		clockTab = 'soundtrack';
	}

	function selectClock(next: string) {
		if (next === 'silent') showSilent();
		else showSoundtrack();
	}

	function showSilent() {
		if (!doc) {
			return;
		}
		if (doc.audio) {
			edit(clearAudio(doc, paceKeepingPictures(doc)));
			songDraft = '';
			songSeconds = null;
		}
		clockTab = 'silent';
	}

	function paceKeepingPictures(file: MontageFile): number {
		const slots = file.slots ?? [];
		const index = slots.findIndex((slot) => slot.dur == null);
		const resolved = plan?.slots[index >= 0 ? index : 0]?.clipDur;
		if (resolved != null && resolved > 0) {
			return resolved;
		}
		if (songSeconds != null && songSeconds > 0) {
			return clipFromFilm(Math.max(slots.length, 1), songSeconds, joinSeconds(file));
		}
		return file.timing?.clip ?? 5;
	}

	function commitTimeEach(raw: string) {
		if (!doc) {
			return;
		}
		const n = Number(raw.trim());
		if (!Number.isFinite(n) || n <= 0) {
			showError('Enter a time for each picture');
			eachDraft = null;
			return;
		}
		edit(setTimeEach(doc, n));
	}

	async function addPicture(path = pictureDraft) {
		const chosen = host.pickImages ? await host.pickImages() : path ? [path] : [];
		if (!chosen.length || !doc) {
			return;
		}
		edit(addPictures(doc, chosen));
		pictureDraft = '';
	}

	function addTitleCard() {
		if (!doc) {
			return;
		}
		edit(addCard(doc));
		selected = (doc.slots?.length ?? 1) - 1;
	}

	function editCard(patch: Partial<TitleCard>) {
		if (!doc) {
			return;
		}
		const slot = doc.slots?.[selected];
		if (!slot?.card) {
			return;
		}
		edit(setCard(doc, selected, { ...slot.card, ...patch }));
	}

	function cardText(slot: Slot, field: 'title' | 'subtitle' | 'line' | 'credits'): string {
		if (cardDraft) {
			return cardDraft[field];
		}
		if (field === 'credits') {
			return (slot.card?.credits ?? []).join('\n');
		}
		return slot.card?.[field] ?? '';
	}

	function draftCard(slot: Slot, field: 'title' | 'subtitle' | 'line' | 'credits', value: string) {
		cardDraft = {
			title: slot.card?.title ?? '',
			subtitle: slot.card?.subtitle ?? '',
			line: slot.card?.line ?? '',
			credits: (slot.card?.credits ?? []).join('\n'),
			...cardDraft,
			[field]: value
		};
	}

	function commitCardText(slot: Slot) {
		if (!cardDraft || !slot.card) {
			return;
		}
		const next = cardDraft;
		cardDraft = null;
		editCard({
			title: next.title,
			subtitle: next.subtitle,
			line: next.line,
			credits: next.credits.split('\n')
		});
	}

	async function replacePicture() {
		if (!doc) {
			return;
		}
		const chosen = host.pickImages ? await host.pickImages() : pictureDraft ? [pictureDraft] : [];
		if (!chosen[0]) {
			return;
		}
		edit(replaceSrc(doc, selected, chosen[0]));
	}

	function commitLength(raw: string) {
		if (!doc) {
			return;
		}
		const text = raw.trim();
		if (!text) {
			if (policyOf(doc) === 'fit-audio' || (policyOf(doc) === 'fixed-clip' && !doc.audio && !uniformClip(doc))) {
				edit(setDur(doc, selected, null));
			}
			return;
		}
		const n = Number(text);
		if (!Number.isFinite(n) || n <= 0) {
			showError('Enter a length in seconds', selected + 1);
			return;
		}
		if (!doc.audio && policyOf(doc) === 'fixed-clip' && (doc.slots?.length ?? 0) < 2) {
			edit(setTimeEach(doc, n));
			return;
		}
		const resolved = plan?.slots[selected]?.clipDur;
		const pinned = doc.slots?.[selected]?.dur != null;
		if (!pinned && resolved != null && Math.abs(n - resolved) < 0.05) {
			lengthDraft = null;
			return;
		}
		edit(setDur(doc, selected, n));
	}

	function lengthValue(): string {
		if (lengthDraft != null) {
			return lengthDraft;
		}
		const pin = doc?.slots?.[selected]?.dur;
		if (pin != null) {
			return secondsLabel(pin);
		}
		const resolved = plan?.slots[selected]?.clipDur;
		return resolved == null ? '' : secondsLabel(resolved);
	}

	async function exportScript() {
		if (!doc || !plan) {
			return;
		}
		const dest = host.pickSaveScript ? await host.pickSaveScript() : '';
		if (!dest && host.pickSaveScript) {
			return;
		}
		const target = dest || `${projectDir(filePath)}/render.sh`;
		busy = 'Writing the shell script…';
		clearError();
		const stop = startWork();
		try {
			if (dirty || !filePath) {
				if (!(await save())) {
					busy = '';
					return;
				}
			}
			await client.exportScript(filePath, target);
			busy = `Wrote ${baseName(target)}`;
		} catch (err) {
			busy = '';
			showError(err instanceof Error ? err.message : 'Could not write the shell script');
		} finally {
			stop();
		}
	}

	async function render() {
		if (!doc || !plan) {
			return;
		}
		clearError();
		try {
			const dest = host.pickSaveVideo ? await host.pickSaveVideo() : '';
			if (host.pickSaveVideo && !dest) {
				return;
			}
			const video = dest || doc.output?.file || 'out.mp4';
			edit({
				...doc,
				output: { ...doc.output, width: doc.output?.width ?? 1920, height: doc.output?.height ?? 1080, file: video }
			});
			busy = 'Rendering…';
			const stop = startWork();
			try {
				if (!(await save())) {
					busy = '';
					return;
				}
				await client.render(filePath);
			} finally {
				stop();
			}
			busy = `Wrote ${baseName(video)} in ${clock(elapsed)}`;
		} catch (err) {
			busy = '';
			showError(err instanceof Error ? err.message : 'Could not render the video');
		}
	}

	/** startWork locks the editor and counts seconds until the returned stop runs. */
	function startWork(): () => void {
		working = true;
		elapsed = 0;
		const started = Date.now();
		const timer = setInterval(() => {
			elapsed = Math.floor((Date.now() - started) / 1000);
		}, 1000);
		return () => {
			clearInterval(timer);
			elapsed = Math.floor((Date.now() - started) / 1000);
			working = false;
		};
	}

	function clock(seconds: number): string {
		return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
	}

	let blocks = $derived(scoreBlocks(doc?.slots, plan));

	function sameBlocks(a: { left: number; width: number }[], b: { left: number; width: number }[]): boolean {
		return (
			a.length === b.length &&
			a.every(
				(block, i) => Math.abs(block.left - b[i].left) < 0.2 && Math.abs(block.width - b[i].width) < 0.2
			)
		);
	}
	let placed = $derived.by(() => {
		if (lift && blocks.length) {
			return dragLayout(blocks, lift.index, lift.to);
		}
		if (reorderHold) {
			return reorderHold.map((block, slot) => ({ ...block, slot }));
		}
		const source =
			preview && doc?.slots
				? blocksFromLengths(
						doc.slots.map((slot) => slotLabel(slot)),
						preview.lengths,
						preview.dissolve
					)
				: blocks;
		return source.map((block, slot) => ({ ...block, slot }));
	});
	let joins = $derived(scoreJoins(placed));

	$effect(() => {
		const el = trackEl;
		const layout = placed.map((block) => `${block.left}:${block.width}`).join('|');
		if (!el || !layout) {
			return;
		}
		const read = () => {
			const width = el.getBoundingClientRect().width;
			if (width > 0) {
				trackWidth = width;
			}
		};
		read();
		let observer: ResizeObserver | undefined;
		if (typeof ResizeObserver !== 'undefined') {
			observer = new ResizeObserver(read);
			observer.observe(el);
		}
		window.addEventListener('resize', read);
		return () => {
			observer?.disconnect();
			window.removeEventListener('resize', read);
		};
	});

	function policyOf(file: MontageFile): string {
		return file.timing?.policy ?? 'fit-audio';
	}

	function uniformClip(file: MontageFile): boolean {
		if (policyOf(file) !== 'fixed-clip') {
			return false;
		}
		if (file.audio || look === 'quick-slides') {
			return true;
		}
		return look == null && (file.defaults?.hold ?? 0) === 0;
	}

	function lengthLocked(file: MontageFile): boolean {
		if (uniformClip(file)) {
			return true;
		}
		return policyOf(file) === 'fit-audio' && (file.slots?.length ?? 0) < 2;
	}

	function timeEachEditable(file: MontageFile): boolean {
		if (file.audio || uniformClip(file)) {
			return false;
		}
		const policy = policyOf(file);
		return policy === 'fixed-clip' || policy === 'fit-audio';
	}

	function filmReadout(): string {
		if (!doc) {
			return '';
		}
		const seconds = plan && (doc.slots?.length ?? 0) > 0 ? plan.duration : outputLength(doc);
		return songClock(seconds);
	}

	function timeEachValue(): string {
		if (eachDraft != null) {
			return eachDraft;
		}
		if (!doc) {
			return '';
		}
		if (policyOf(doc) === 'fixed-clip' && doc.timing?.clip != null) {
			return secondsLabel(doc.timing.clip);
		}
		const slots = doc.slots ?? [];
		const index = slots.findIndex((slot) => slot.dur == null);
		const fromPlan = plan?.slots[index >= 0 ? index : 0]?.clipDur;
		if (fromPlan != null && (policyOf(doc) === 'fit-audio' || index >= 0)) {
			return secondsLabel(fromPlan);
		}
		if (doc.timing?.duration != null) {
			return secondsLabel(clipFromFilm(Math.max(slots.length, 1), doc.timing.duration, joinSeconds(doc)));
		}
		return '';
	}

	function choose(index: number) {
		selected = index;
		lengthDraft = null;
		cardDraft = null;
	}

	function onPictureClick(index: number) {
		if (swallowClick) {
			swallowClick = false;
			return;
		}
		choose(index);
	}

	function pictureCenters(): number[] {
		const rect = trackEl?.getBoundingClientRect();
		if (!rect || rect.width <= 0) {
			return [];
		}
		return blocks.map((block) => rect.left + ((block.left + block.width / 2) / 100) * rect.width);
	}

	function dissolveOf(file: MontageFile, join: number): number {
		return plan?.slots[join]?.join?.duration ?? file.defaults?.join?.duration ?? 1.5;
	}

	function startPicture(event: PointerEvent, index: number) {
		if (event.button !== 0 || !doc) {
			return;
		}
		swallowClick = false;
		dragArmed = true;
		try {
			trackEl?.setPointerCapture(event.pointerId);
		} catch {
			// The track keeps the gesture after the picture lifts off it.
		}
		const originX = event.clientX;
		const originY = event.clientY;
		let moved = false;
		let grabOffset = 0;
		const move = (e: PointerEvent) => {
			const rect = trackEl?.getBoundingClientRect();
			const block = blocks[index];
			if (!rect || rect.width <= 0 || !block) {
				return;
			}
			if (!moved) {
				if (Math.hypot(e.clientX - originX, e.clientY - originY) < dragSlop) {
					return;
				}
				moved = true;
				grabOffset = e.clientX - (rect.left + (block.left / 100) * rect.width);
				document.body.style.cursor = 'grabbing';
			}
			const width = (block.width / 100) * rect.width;
			let left = e.clientX - rect.left - grabOffset;
			if (left < 0) {
				left = 0;
			}
			if (left > rect.width - width) {
				left = Math.max(0, rect.width - width);
			}
			lift = { index, to: dropIndex(pictureCenters(), index, e.clientX), left, width };
		};
		const up = () => {
			window.removeEventListener('pointermove', move);
			window.removeEventListener('pointerup', up);
			window.removeEventListener('pointercancel', up);
			document.body.style.cursor = '';
			const to = lift?.to;
			const landed = moved && to != null && plan && plan.duration > 0 ? settledLayout(blocks, index, to) : null;
			lift = null;
			dragArmed = false;
			if (!moved || !doc || to == null) {
				return;
			}
			swallowClick = true;
			if (!pictureCenters().length) {
				return;
			}
			selected = to;
			const next = reorderSlot(doc, index, to);
			if (next !== doc) {
				edit(next);
				reorderHold = landed;
			}
		};
		window.addEventListener('pointermove', move);
		window.addEventListener('pointerup', up);
		window.addEventListener('pointercancel', up);
	}

	function startGrip(event: PointerEvent, join: number) {
		if (event.button !== 0 || !doc || !plan || plan.duration <= 0) {
			return;
		}
		event.stopPropagation();
		const rect = trackEl?.getBoundingClientRect();
		if (!rect || rect.width <= 0) {
			return;
		}
		const file = doc;
		const originX = event.clientX;
		const film = plan.duration;
		const width = rect.width;
		const lengths = plan.slots.map((slot) => slot.clipDur);
		const dissolve = dissolveOf(file, join);
		const move = (e: PointerEvent) => {
			const delta = ((e.clientX - originX) / width) * film;
			const next = resizedLengths(lengths, join, delta, dissolve);
			preview = next ? { lengths: next, dissolve } : null;
		};
		const up = (e: PointerEvent) => {
			window.removeEventListener('pointermove', move);
			window.removeEventListener('pointerup', up);
			window.removeEventListener('pointercancel', up);
			const delta = ((e.clientX - originX) / width) * film;
			const next = resizePictures(file, lengths, join, delta, dissolve);
			if (next !== file) {
				gripCommit = true;
				edit(next, true);
				return;
			}
			preview = null;
			gripCommit = false;
		};
		window.addEventListener('pointermove', move);
		window.addEventListener('pointerup', up);
		window.addEventListener('pointercancel', up);
	}
</script>

{#snippet pictureIcon(name: 'up' | 'down' | 'replace' | 'remove')}
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
		{#if name === 'up'}
			<path d="m5 12 7-7 7 7" />
			<path d="M12 19V5" />
		{:else if name === 'down'}
			<path d="M12 5v14" />
			<path d="m19 12-7 7-7-7" />
		{:else if name === 'replace'}
			<path d="M8 3 4 7l4 4" />
			<path d="M4 7h16" />
			<path d="m16 21 4-4-4-4" />
			<path d="M20 17H4" />
		{:else}
			<path d="M10 11v6" />
			<path d="M14 11v6" />
			<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" />
			<path d="M3 6h18" />
			<path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
		{/if}
	</svg>
{/snippet}

{#snippet pictureAction(label: string, onclick: () => void, disabled: boolean, icon: 'up' | 'down' | 'replace' | 'remove')}
	<span class="group relative inline-flex">
		<Button variant="outline" size="icon" aria-label={label} {onclick} {disabled}>
			{@render pictureIcon(icon)}
		</Button>
		<span
			role="tooltip"
			class="bg-foreground text-background pointer-events-none absolute bottom-full left-1/2 z-20 mb-1 -translate-x-1/2 rounded px-2 py-1 text-xs whitespace-nowrap opacity-0 group-hover:opacity-100 group-focus-within:opacity-100"
		>{label}</span>
	</span>
{/snippet}

<svelte:head>
	<title>{title}</title>
</svelte:head>

<main class="mx-auto flex w-full min-w-0 flex-col gap-4 p-4 sm:p-6 lg:p-8">
	<fieldset class="flex min-w-0 flex-col gap-4" disabled={working} aria-busy={working}>
		<section class="flex flex-col gap-2 {rule}">
			<header>
				<h1 class="text-2xl font-semibold">{title}</h1>
			</header>
			<div class={cluster}>
				<div class="flex min-w-0 flex-wrap items-center gap-2">
					{#if !host.pickSaveMontage}
						<input
							class="border-input bg-background h-9 w-96 max-w-full min-w-0 rounded-md border px-3"
							aria-label="Project file"
							bind:value={pathDraft}
						/>
					{/if}
					<div class="flex flex-wrap gap-2">
						<Button onclick={newProjectFile} disabled={working}>New</Button>
						<Button variant="outline" onclick={() => openProject()} disabled={working}>Open</Button>
						<Button variant="outline" onclick={save} disabled={working || !doc}>Save</Button>
					</div>
				</div>
				<p class="text-muted-foreground w-40 text-sm whitespace-nowrap" data-aside>
					{dirty ? 'Unsaved changes' : filePath ? 'Saved' : ''}
				</p>
				{#if filePath}
					<p class="col-span-2 min-w-0 text-sm break-all">{filePath}</p>
				{/if}
			</div>
		</section>

	{#if doc}
		<section class="flex flex-col gap-2 {rule}">
			<Tabs class="w-[400px]" bind:value={clockTab} onValueChange={selectClock}>
				<TabsList aria-label="Clock">
					<TabsTrigger value="soundtrack">Soundtrack</TabsTrigger>
					<TabsTrigger value="silent">No Soundtrack</TabsTrigger>
				</TabsList>
				<TabsContent value="soundtrack">
					<Card>
						<CardContent>
							<div class="flex flex-wrap gap-2">
								{#if !host.pickAudio}
									<input
										class="border-input bg-background h-9 w-full min-w-0 rounded-md border px-3"
										aria-label="Song file"
										bind:value={songDraft}
									/>
								{/if}
								<Button variant="outline" onclick={() => addSong()}>
									{doc.audio ? 'Change soundtrack' : 'Add soundtrack'}
								</Button>
							</div>
							{#if doc.audio}
								<div class="flex flex-wrap items-baseline justify-between gap-4">
									<p class="min-w-0 text-sm break-all">{baseName(doc.audio)}</p>
									<p class="w-40 text-sm whitespace-nowrap tabular-nums" data-aside>
										{songSeconds != null ? songClock(songSeconds) : ''}
									</p>
								</div>
							{/if}
						</CardContent>
					</Card>
				</TabsContent>
				<TabsContent value="silent">
					<Card>
						<CardContent>
							<div class="flex flex-wrap items-end gap-6">
								<p class="flex flex-col gap-1 text-sm">
									<span>Film length</span>
									<span class="tabular-nums">{filmReadout()}</span>
								</p>
								{#if policyOf(doc) === 'fixed-clip' || policyOf(doc) === 'fit-audio'}
									<label class="flex flex-col gap-1 text-sm">
										Time for each picture
										<input
											class="border-input bg-background h-9 w-28 rounded-md border px-3"
											aria-label="Time for each picture"
											disabled={!timeEachEditable(doc)}
											value={timeEachValue()}
											oninput={(event) => (eachDraft = event.currentTarget.value)}
											onchange={(event) => commitTimeEach(event.currentTarget.value)}
										/>
									</label>
								{/if}
							</div>
						</CardContent>
					</Card>
				</TabsContent>
			</Tabs>
		</section>

		<section class="flex flex-col gap-2 {rule}">
			<h2 class="font-medium">Look</h2>
			<div class="flex flex-wrap gap-2">
				{#each looks as item (item.id)}
					<Button
						variant={look === item.id ? 'default' : 'outline'}
						aria-pressed={look === item.id}
						onclick={() => {
							look = item.id;
							edit(applyLook(doc!, item.id));
						}}
					>
						{item.label}
					</Button>
				{/each}
			</div>
		</section>

		<section class="flex flex-col gap-2 {rule}">
			<h2 class="font-medium">Pictures</h2>
			<div class="flex flex-wrap gap-2">
				{#if !host.pickImages}
					<input
						class="border-input bg-background h-9 w-full min-w-0 rounded-md border px-3 sm:w-96"
						aria-label="Picture file"
						bind:value={pictureDraft}
					/>
				{/if}
				<Button variant="outline" onclick={() => addPicture()}>Add pictures</Button>
				<Button variant="outline" onclick={addTitleCard}>Add card</Button>
				<Button variant="outline" onclick={() => edit(sortByName(doc!))}>Sort by name</Button>
				<Button variant="outline" onclick={() => edit(shuffleSlots(doc!))}>Shuffle pictures</Button>
				<Button variant="outline" onclick={() => edit(shuffleMoves(doc!))}>Shuffle moves</Button>
				<Button variant="outline" onclick={checkFiles} disabled={working}>Re-scan Pictures/Audio</Button>
			</div>

			<div class="flex flex-col gap-1" role="group" aria-labelledby="timeline-label">
				<p id="timeline-label" class="text-sm font-medium">Timeline</p>
				<div class="relative">
				<div
					bind:this={trackEl}
					data-timeline
					class="border-foreground/15 bg-foreground/10 relative flex h-20 items-center touch-none overflow-hidden rounded-md border {lift ? 'cursor-grabbing' : ''}"
				>
					<div class="film-rail pointer-events-none absolute inset-x-0 top-0 z-40 h-2.5" aria-hidden="true"></div>
					<div class="film-rail pointer-events-none absolute inset-x-0 bottom-0 z-40 h-2.5" aria-hidden="true"></div>
					{#each placed as block (block.src + block.slot)}
						<button
							type="button"
							class="mark absolute top-2.5 bottom-2.5 flex cursor-grab items-center justify-center overflow-hidden text-xs {selected === block.slot
								? 'ring-primary z-10 ring-2 ring-inset'
								: 'z-0'} bg-[oklch(0.78_0_0)] {dragArmed ? 'slot-move' : ''}"
							style="left: {block.left}%; width: {block.width}%"
							title={block.src}
							aria-label="{block.slot + 1}. {block.src}"
							aria-pressed={selected === block.slot}
							onpointerdown={(event) => startPicture(event, block.slot)}
							onclick={() => onPictureClick(block.slot)}
						>
							{#if thumbForSlot(doc.slots?.[block.slot])}
								<img alt="" src={thumbForSlot(doc.slots?.[block.slot])} draggable="false" class="pointer-events-none absolute inset-0 h-full w-full object-cover" />
							{/if}
							<span class="mark-n pointer-events-none relative z-10 rounded bg-black/60 px-1 text-white">{block.slot + 1}</span>
						</button>
					{/each}
					{#if placed.length === 0 && !lift}
						<p class="text-muted-foreground p-2 text-sm">Each picture shows up here as you add it.</p>
					{/if}
					{#each joins as join, index (join.left + '-' + index)}
						<div
							class="pointer-events-none absolute top-2.5 bottom-2.5 z-20 bg-[oklch(0.5_0_0)] {dragArmed ? 'slot-move' : ''}"
							style="left: {join.left}%; width: {join.width}%"
							data-join
						></div>
						{#if !lift}
							{@const center = join.left + join.width / 2}
							{@const hit = trackWidth > 0 ? gripHit(placed[index], placed[index + 1], center, trackWidth, gripWidth) : null}
							<div class="pointer-events-none absolute top-2.5 bottom-2.5 z-30" style="left: {center}%; width: 0">
								<span data-grip-mark class="pointer-events-none absolute inset-y-3 left-1/2 w-0.5 -translate-x-1/2 rounded bg-white"></span>
							</div>
							{#if hit == null || hit.width > 0}
								<button
									type="button"
									class="absolute top-2.5 bottom-2.5 z-30 cursor-ew-resize"
									style={hit ? `left: ${hit.left}px; width: ${hit.width}px` : `left: ${center}%; width: ${gripWidth}px; transform: translateX(-50%)`}
									aria-label="Change the lengths beside pictures {index + 1} and {index + 2}"
									data-grip
									onpointerdown={(event) => startGrip(event, index)}
								></button>
							{/if}
						{/if}
					{/each}
				</div>
				{#if lift && blocks[lift.index]}
					<div
						class="pointer-events-none absolute -top-1.5 -bottom-1.5 z-50 overflow-hidden rounded-sm shadow-[0_8px_18px_rgba(0,0,0,0.45)]"
						style="left: {lift.left}px; width: {lift.width}px"
						data-lift
						aria-hidden="true"
					>
						{#if thumbForSlot(doc.slots?.[lift.index])}
							<img alt="" src={thumbForSlot(doc.slots?.[lift.index])} draggable="false" class="absolute inset-0 h-full w-full object-cover" />
						{:else}
							<div class="absolute inset-0 bg-[oklch(0.78_0_0)]"></div>
						{/if}
						<span class="pointer-events-none absolute top-1/2 left-1/2 z-10 -translate-x-1/2 -translate-y-1/2 rounded bg-black/60 px-1 text-xs text-white">{lift.index + 1}</span>
					</div>
				{/if}
				</div>
				{#if plan}
					<div class="text-muted-foreground flex justify-between text-xs tabular-nums">
						<span>0:00</span>
						<span>{clock(Math.round(plan.duration))}</span>
					</div>
				{/if}
			</div>
			{#if plan}
				<p>{describePlan(plan)}</p>
			{/if}

			<ul class="flex flex-col gap-2">
				{#each doc.slots ?? [] as slot, index (`${slot.src ?? 'card'}:${index}`)}
					<li class="rounded-md border p-3 {selected === index ? 'border-primary' : ''}">
						<button type="button" class="flex w-full min-w-0 items-center gap-2 text-left" onclick={() => choose(index)}>
							{#if thumbForSlot(slot)}
								<img alt="" src={thumbForSlot(slot)} class="h-10 w-16 shrink-0 rounded object-cover" />
							{/if}
							<span class="min-w-0 break-all">
								{index + 1}. {slotLabel(slot)}
								{#if plan?.slots[index]}
									<span class="text-muted-foreground"> · {secondsLabel(plan.slots[index].clipDur)}s</span>
								{/if}
							</span>
						</button>
						{#if error && errorSlot === index + 1}
							<p class="text-destructive text-sm">{error}</p>
						{/if}
						{#if selected === index}
							<div class="mt-3 flex flex-col gap-2">
								{#if slot.card}
									<label class="flex flex-col gap-1 text-sm">
										Title
										<input
											class="border-input bg-background h-9 rounded-md border px-3"
											aria-label="Title"
											value={cardText(slot, 'title')}
											oninput={(event) => draftCard(slot, 'title', event.currentTarget.value)}
											onchange={() => commitCardText(slot)}
										/>
									</label>
									<label class="flex flex-col gap-1 text-sm">
										Subtitle
										<input
											class="border-input bg-background h-9 rounded-md border px-3"
											aria-label="Subtitle"
											value={cardText(slot, 'subtitle')}
											oninput={(event) => draftCard(slot, 'subtitle', event.currentTarget.value)}
											onchange={() => commitCardText(slot)}
										/>
									</label>
									<label class="flex flex-col gap-1 text-sm">
										Third line
										<input
											class="border-input bg-background h-9 rounded-md border px-3"
											aria-label="Third line"
											value={cardText(slot, 'line')}
											oninput={(event) => draftCard(slot, 'line', event.currentTarget.value)}
											onchange={() => commitCardText(slot)}
										/>
									</label>
									<label class="flex flex-col gap-1 text-sm">
										Credits
										<textarea
											class="border-input bg-background min-h-20 rounded-md border px-3 py-2"
											aria-label="Credits"
											value={cardText(slot, 'credits')}
											oninput={(event) => draftCard(slot, 'credits', event.currentTarget.value)}
											onchange={() => commitCardText(slot)}
										></textarea>
									</label>
									<div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
										<label class="flex flex-col gap-1 text-sm">
											Align
											<select
												class="border-input bg-background h-9 rounded-md border px-2"
												aria-label="Align"
												value={slot.card.align ?? 'center'}
												onchange={(event) =>
													editCard({ align: event.currentTarget.value as TitleCard['align'] })}
											>
												<option value="left">Left</option>
												<option value="center">Center</option>
												<option value="right">Right</option>
											</select>
										</label>
										<label class="flex flex-col gap-1 text-sm">
											Vertical
											<select
												class="border-input bg-background h-9 rounded-md border px-2"
												aria-label="Vertical"
												value={slot.card.vAlign ?? 'middle'}
												onchange={(event) =>
													editCard({ vAlign: event.currentTarget.value as TitleCard['vAlign'] })}
											>
												<option value="upper">Upper</option>
												<option value="middle">Middle</option>
												<option value="lower">Lower</option>
											</select>
										</label>
										<label class="flex flex-col gap-1 text-sm">
											Size
											<select
												class="border-input bg-background h-9 rounded-md border px-2"
												aria-label="Size"
												value={slot.card.size ?? 'medium'}
												onchange={(event) =>
													editCard({ size: event.currentTarget.value as TitleCard['size'] })}
											>
												<option value="small">Small</option>
												<option value="medium">Medium</option>
												<option value="large">Large</option>
											</select>
										</label>
										<label class="flex flex-col gap-1 text-sm">
											Weight
											<select
												class="border-input bg-background h-9 rounded-md border px-2"
												aria-label="Weight"
												value={slot.card.weight ?? 'bold'}
												onchange={(event) =>
													editCard({ weight: event.currentTarget.value as TitleCard['weight'] })}
											>
												<option value="regular">Regular</option>
												<option value="bold">Bold</option>
											</select>
										</label>
										<label class="flex flex-col gap-1 text-sm">
											Face
											<select
												class="border-input bg-background h-9 rounded-md border px-2"
												aria-label="Face"
												value={slot.card.face ?? 'sans'}
												onchange={(event) =>
													editCard({ face: event.currentTarget.value as TitleCard['face'] })}
											>
												<option value="sans">Sans</option>
												<option value="serif">Serif</option>
											</select>
										</label>
										<label class="flex items-center gap-2 text-sm">
											<input
												type="checkbox"
												checked={slot.card.box ?? false}
												onchange={(event) => editCard({ box: event.currentTarget.checked })}
											/>
											Faint box
										</label>
									</div>
									<div class="grid grid-cols-2 gap-2">
										<label class="flex flex-col gap-1 text-sm">
											Text color
											<span class="border-input bg-background flex h-9 items-center gap-2 rounded-md border px-2">
												<input
													type="color"
													class="color-chip"
													aria-label="Text color"
													value={slot.card.color ?? '#FFFFFF'}
													onchange={(event) => editCard({ color: event.currentTarget.value.toUpperCase() })}
												/>
												<span class="font-mono text-xs">{(slot.card.color ?? '#FFFFFF').toUpperCase()}</span>
											</span>
										</label>
										<label class="flex flex-col gap-1 text-sm">
											Background
											<span class="border-input bg-background flex h-9 items-center gap-2 rounded-md border px-2">
												<input
													type="color"
													class="color-chip"
													aria-label="Background color"
													value={slot.card.fill ?? '#000000'}
													onchange={(event) => editCard({ fill: event.currentTarget.value.toUpperCase() })}
												/>
												<span class="font-mono text-xs">{(slot.card.fill ?? '#000000').toUpperCase()}</span>
											</span>
										</label>
									</div>
								{/if}
								<label class="flex flex-col gap-1 text-sm">
									Move
									<select
										class="border-input bg-background h-9 rounded-md border px-2"
										value={slot.mode ?? 'in-center'}
										onchange={(event) => edit(setMode(doc!, index, event.currentTarget.value))}
									>
										{#each moveChoices(slot.mode) as move (move.id)}
											<option value={move.id}>{move.label}</option>
										{/each}
									</select>
								</label>
								<label class="flex flex-col gap-1 text-sm">
									Length in seconds
									<input
										class="border-input bg-background h-9 rounded-md border px-3"
										aria-label="Length in seconds"
										disabled={lengthLocked(doc)}
										value={lengthValue()}
										oninput={(event) => (lengthDraft = event.currentTarget.value)}
										onchange={(event) => commitLength(event.currentTarget.value)}
									/>
								</label>
								{#if uniformClip(doc)}
									<p class="text-muted-foreground text-sm">This look uses the same length for every picture.</p>
								{:else if policyOf(doc) === 'fit-audio' && (doc.slots?.length ?? 0) < 2}
									<p class="text-muted-foreground text-sm">
										This picture runs for the whole {doc.audio ? 'song' : 'film'}.
									</p>
								{/if}
								<div class="flex flex-wrap items-center gap-2">
									{#if slot.dur != null && (policyOf(doc) === 'fit-audio' || (policyOf(doc) === 'fixed-clip' && !uniformClip(doc)))}
										<Button variant="outline" onclick={() => edit(setDur(doc!, index, null))}>Remove custom time</Button>
									{/if}
									{#if slot.fixed}
										<Button variant="outline" onclick={() => edit(setFixed(doc!, index, false))}>Unpin position</Button>
									{:else}
										<Button variant="outline" onclick={() => edit(setFixed(doc!, index, true))}>Pin position</Button>
									{/if}
									{@render pictureAction('Move up', () => edit(moveSlot(doc!, index, -1)), index === 0, 'up')}
									{@render pictureAction(
										'Move down',
										() => edit(moveSlot(doc!, index, 1)),
										index === (doc.slots?.length ?? 1) - 1,
										'down'
									)}
									{#if !isCard(slot)}
										{@render pictureAction('Replace', replacePicture, false, 'replace')}
									{/if}
									{@render pictureAction(
										'Remove',
										() => {
											edit(removeSlot(doc!, index));
											selected = Math.max(0, index - 1);
										},
										false,
										'remove'
									)}
								</div>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="font-medium">Render</h2>
			{#if error}
				<p role="alert" class="text-destructive text-sm whitespace-pre-line">{error}</p>
			{/if}
			<div class="flex flex-wrap items-center gap-2">
				<Button variant="outline" onclick={exportScript} disabled={working || !plan}>
					Export shell script
				</Button>
				<Button onclick={render} disabled={working || !plan}>Render video</Button>
				{#if busy}
					<p role="status" class="min-w-0 text-sm break-all">
						{busy}
						{#if working}
							<span class="text-muted-foreground"> {clock(elapsed)}</span>
						{/if}
					</p>
				{/if}
			</div>
		</section>
	{/if}
	</fieldset>
	{#if error && !doc}
		<p role="alert" class="text-destructive text-sm whitespace-pre-line">{error}</p>
	{/if}
</main>

<style>
	.film-rail {
		background: #000;
		-webkit-mask-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='10'%3E%3Cpath fill='white' fill-rule='evenodd' d='M0 0h16v10H0zM5.5 2h5a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-5a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1z'/%3E%3C/svg%3E");
		mask-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='10'%3E%3Cpath fill='white' fill-rule='evenodd' d='M0 0h16v10H0zM5.5 2h5a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-5a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1z'/%3E%3C/svg%3E");
		-webkit-mask-size: 16px 10px;
		mask-size: 16px 10px;
		-webkit-mask-repeat: repeat-x;
		mask-repeat: repeat-x;
		-webkit-mask-position: left center;
		mask-position: left center;
	}
	.slot-move {
		transition:
			left 120ms ease-out,
			width 120ms ease-out;
	}
	.mark {
		container-type: inline-size;
	}
	.color-chip {
		-webkit-appearance: none;
		appearance: none;
		width: 1.75rem;
		height: 1.75rem;
		margin: 0;
		padding: 2px;
		border: 1px solid #111;
		border-radius: 0.25rem;
		background-color: #fff;
		background-image:
			linear-gradient(45deg, #bdbdbd 25%, transparent 25%),
			linear-gradient(-45deg, #bdbdbd 25%, transparent 25%),
			linear-gradient(45deg, transparent 75%, #bdbdbd 75%),
			linear-gradient(-45deg, transparent 75%, #bdbdbd 75%);
		background-size: 8px 8px;
		background-position:
			0 0,
			0 4px,
			4px -4px,
			-4px 0;
		cursor: pointer;
	}
	.color-chip::-webkit-color-swatch-wrapper {
		padding: 0;
	}
	.color-chip::-webkit-color-swatch,
	.color-chip::-moz-color-swatch {
		border: 1px solid rgba(0, 0, 0, 0.55);
		border-radius: 2px;
	}
	.mark-n {
		display: none;
	}
	@container (min-width: 1.25rem) {
		.mark-n {
			display: block;
		}
	}
</style>

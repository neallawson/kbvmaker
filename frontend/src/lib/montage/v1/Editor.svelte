<script lang="ts">
	import { untrack } from 'svelte';
	import { Button } from '#lib/components/ui/button';
	import type { Host } from '#lib/platform/host';
	import { RuleError, type Client, type MontageFile, type Plan } from './client';
	import {
		addPictures,
		applyLook,
		baseName,
		blocksFromLengths,
		dragLayout,
		dropIndex,
		gripHit,
		settledLayout,
		moveSlot,
		newProject,
		mediaPath,
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
		songClock,
		setDur,
		setMode,
		shuffleMoves,
		shuffleSlots,
		sortByName,
		type LookId
	} from './document';
	import { describePlan } from './summary';

	let { client, host }: { client: Client; host: Host } = $props();

	let filePath = $state('');
	let pathDraft = $state('');
	let doc = $state<MontageFile | null>(null);
	let look = $state<LookId | null>(null);
	let selected = $state(0);
	let dirty = $state(false);
	let plan = $state<Plan | null>(null);
	let error = $state('');
	let errorSlot = $state(0);
	let busy = $state('');
	let working = $state(false);
	let elapsed = $state(0);
	let songDraft = $state('');
	let songSeconds = $state<number | null>(null);
	let pictureDraft = $state('');
	let lengthDraft = $state<string | null>(null);
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

	const title = 'kbvmaker - Montage (v1) editor, renderer';
	const thumbWidth = 320;
	const thumbHeight = 180;
	const gripWidth = 28;
	const dragSlop = 4;

	const looks: { id: LookId; label: string }[] = [
		{ id: 'classic', label: 'Classic' },
		{ id: 'photo-film', label: 'Photo film' },
		{ id: 'quick-slides', label: 'Quick slides' }
	];

	$effect(() => {
		const current = doc;
		const dir = projectDir(filePath);
		if (!current?.audio || !current.slots?.length) {
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
				if (next.song != null) {
					songSeconds = next.song;
				}
				error = '';
				errorSlot = 0;
			})
			.catch((err: unknown) => {
				if (!live) {
					return;
				}
				plan = null;
				reorderHold = null;
				if (err instanceof RuleError) {
					error = err.message;
					errorSlot = err.slot;
				} else {
					error = 'Could not read the plan';
				}
			});
		return () => {
			live = false;
		};
	});

	const thumbInflight = new Set<string>();

	$effect(() => {
		const dir = projectDir(filePath);
		const paths = (doc?.slots ?? []).map((slot) => mediaPath(dir, slot.src));
		const pending = untrack(() => {
			const next: string[] = [];
			for (const path of paths) {
				if (!path || thumbs[path] || thumbInflight.has(path) || next.includes(path)) {
					continue;
				}
				next.push(path);
			}
			return next;
		});
		for (const path of pending) {
			thumbInflight.add(path);
			void client
				.thumb(path, thumbWidth, thumbHeight)
				.then((url) => {
					thumbs[path] = url;
				})
				.catch(() => {})
				.finally(() => {
					thumbInflight.delete(path);
				});
		}
	});

	function thumbFor(src: string): string {
		return thumbs[mediaPath(projectDir(filePath), src)] ?? '';
	}

	function edit(next: MontageFile, holdPreview = false) {
		doc = next;
		dirty = true;
		lengthDraft = null;
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
		selected = 0;
		dirty = true;
		error = '';
		plan = null;
		reorderHold = null;
		lift = null;
		songSeconds = null;
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
			error = '';
			lengthDraft = null;
			songSeconds = null;
			if (doc.audio) {
				void measureSong(doc.audio);
			}
		} catch (err) {
			error = err instanceof RuleError ? err.message : 'Could not open the project';
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
			error = '';
			return true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not save the project';
			errorSlot = 0;
			return false;
		}
	}

	async function measureSong(path: string) {
		try {
			const info = await client.audio(mediaPath(projectDir(filePath), path));
			songSeconds = info.seconds;
		} catch (err) {
			songSeconds = null;
			error = err instanceof RuleError ? err.message : 'Could not read the song';
			errorSlot = 0;
		}
	}

	async function addSong(path = songDraft) {
		const chosen = host.pickAudio ? await host.pickAudio() : path;
		if (!chosen || !doc) {
			return;
		}
		edit(setAudio(doc, chosen));
		await measureSong(chosen);
	}

	async function addPicture(path = pictureDraft) {
		const chosen = host.pickImages ? await host.pickImages() : path ? [path] : [];
		if (!chosen.length || !doc) {
			return;
		}
		edit(addPictures(doc, chosen, look));
		pictureDraft = '';
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
			if ((doc.timing?.policy ?? 'fit-audio') === 'fit-audio') {
				edit(setDur(doc, selected, null));
			}
			return;
		}
		const n = Number(text);
		if (!Number.isFinite(n) || n <= 0) {
			error = 'Enter a length in seconds';
			errorSlot = selected + 1;
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
		if (!doc) {
			return;
		}
		const dest = host.pickSaveScript ? await host.pickSaveScript() : '';
		if (!dest && host.pickSaveScript) {
			return;
		}
		const target = dest || `${projectDir(filePath)}/render.sh`;
		busy = 'Writing the shell script…';
		error = '';
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
			error = err instanceof Error ? err.message : 'Could not write the shell script';
		} finally {
			stop();
		}
	}

	async function render() {
		if (!doc) {
			return;
		}
		error = '';
		errorSlot = 0;
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
			error = err instanceof Error ? err.message : 'Could not render the video';
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
						doc.slots.map((slot) => slot.src),
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

	function lengthLocked(file: MontageFile): boolean {
		const policy = policyOf(file);
		if (policy === 'fixed-clip') {
			return true;
		}
		return policy === 'fit-audio' && (file.slots?.length ?? 0) < 2;
	}

	function choose(index: number) {
		selected = index;
		lengthDraft = null;
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

<svelte:head>
	<title>{title}</title>
</svelte:head>

<main class="mx-auto flex max-w-3xl flex-col gap-4 p-6">
	<header>
		<h1 class="text-2xl font-semibold">{title}</h1>
	</header>

	<fieldset class="flex min-w-0 flex-col gap-4" disabled={working} aria-busy={working}>
	<div class="flex flex-wrap items-center gap-2">
		{#if !host.pickSaveMontage}
			<input
				class="border-input bg-background h-9 min-w-64 rounded-md border px-3"
				aria-label="Project file"
				bind:value={pathDraft}
			/>
		{/if}
		<div class="flex flex-wrap gap-2">
			<Button onclick={newProjectFile} disabled={working}>New</Button>
			<Button variant="outline" onclick={() => openProject()} disabled={working}>Open</Button>
			<Button variant="outline" onclick={save} disabled={working || !doc}>Save</Button>
		</div>
		<p class="text-muted-foreground ml-auto text-sm">{dirty ? 'Unsaved changes' : filePath ? 'Saved' : ''}</p>
	</div>
	{#if filePath}
		<p class="text-sm">{filePath}</p>
	{/if}

	{#if doc}
		<section class="flex flex-col gap-2">
			<h2 class="font-medium">Soundtrack</h2>
			<div class="flex flex-wrap gap-2">
				{#if !host.pickAudio}
					<input
						class="border-input bg-background h-9 min-w-64 rounded-md border px-3"
						aria-label="Song file"
						bind:value={songDraft}
					/>
				{/if}
				<Button variant="outline" onclick={() => addSong()}>
					{doc.audio ? 'Change soundtrack' : 'Add soundtrack'}
				</Button>
			</div>
			{#if doc.audio}
				<p class="flex items-baseline justify-between text-sm">
					<span>{baseName(doc.audio)}</span>
					{#if songSeconds != null}
						<span>{songClock(songSeconds)}</span>
					{/if}
				</p>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
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

		<section class="flex flex-col gap-2">
			<h2 class="font-medium">Pictures</h2>
			<div class="flex flex-wrap gap-2">
				{#if !host.pickImages}
					<input
						class="border-input bg-background h-9 min-w-64 rounded-md border px-3"
						aria-label="Picture file"
						bind:value={pictureDraft}
					/>
				{/if}
				<Button variant="outline" onclick={() => addPicture()}>Add pictures</Button>
				<Button variant="outline" onclick={() => edit(sortByName(doc!))}>Sort by name</Button>
				<Button variant="outline" onclick={() => edit(shuffleSlots(doc!))}>Shuffle pictures</Button>
				<Button variant="outline" onclick={() => edit(shuffleMoves(doc!))}>Shuffle moves</Button>
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
							title={baseName(block.src)}
							aria-label="{block.slot + 1}. {baseName(block.src)}"
							aria-pressed={selected === block.slot}
							onpointerdown={(event) => startPicture(event, block.slot)}
							onclick={() => onPictureClick(block.slot)}
						>
							{#if thumbFor(block.src)}
								<img alt="" src={thumbFor(block.src)} draggable="false" class="pointer-events-none absolute inset-0 h-full w-full object-cover" />
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
						{#if thumbFor(blocks[lift.index].src)}
							<img alt="" src={thumbFor(blocks[lift.index].src)} draggable="false" class="absolute inset-0 h-full w-full object-cover" />
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
				{#each doc.slots ?? [] as slot, index (slot.src + index)}
					<li class="rounded-md border p-3 {selected === index ? 'border-primary' : ''}">
						<button type="button" class="flex w-full items-center gap-2 text-left" onclick={() => choose(index)}>
							{#if thumbFor(slot.src)}
								<img alt="" src={thumbFor(slot.src)} class="h-10 w-16 shrink-0 rounded object-cover" />
							{/if}
							<span>
								{index + 1}. {baseName(slot.src)}
								{#if plan?.slots[index]}
									<span class="text-muted-foreground"> · {secondsLabel(plan.slots[index].clipDur)}s</span>
								{/if}
							</span>
						</button>
						{#if error && errorSlot === index + 1}
							<p role="alert">{error}</p>
						{/if}
						{#if selected === index}
							<div class="mt-3 flex flex-col gap-2">
								<label class="flex flex-col gap-1 text-sm">
									Move
									<select
										class="border-input bg-background h-9 rounded-md border px-2"
										value={slot.mode ?? 'in-center'}
										onchange={(event) => edit(setMode(doc!, index, event.currentTarget.value))}
									>
										{#each [{ id: 'in-center', label: 'Slow zoom in' }, { id: 'in-left-right', label: 'Pan across' }, { id: 'in-top-bottom', label: 'Drift down' }, { id: 'out-center', label: 'Slow zoom out' }, { id: 'hold', label: 'Hold still' }] as move (move.id)}
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
								{#if policyOf(doc) === 'fixed-clip'}
									<p class="text-muted-foreground text-sm">This look uses the same length for every picture.</p>
								{:else if policyOf(doc) === 'fit-audio' && (doc.slots?.length ?? 0) < 2}
									<p class="text-muted-foreground text-sm">This picture runs for the whole song.</p>
								{/if}
								<div class="flex flex-wrap gap-2">
									{#if policyOf(doc) === 'fit-audio' && slot.dur != null}
										<Button variant="outline" onclick={() => edit(setDur(doc!, index, null))}>Remove custom time</Button>
									{/if}
									<Button variant="outline" onclick={() => edit(moveSlot(doc!, index, -1))} disabled={index === 0}>Move up</Button>
									<Button
										variant="outline"
										onclick={() => edit(moveSlot(doc!, index, 1))}
										disabled={index === (doc.slots?.length ?? 1) - 1}
									>
										Move down
									</Button>
									<Button variant="outline" onclick={replacePicture}>Replace</Button>
									<Button
										variant="outline"
										onclick={() => {
											edit(removeSlot(doc!, index));
											selected = Math.max(0, index - 1);
										}}
									>
										Remove
									</Button>
								</div>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		</section>

		<div class="flex flex-wrap items-center gap-2">
			<Button variant="outline" onclick={exportScript} disabled={working || !doc.audio || !doc.slots?.length}>
				Export shell script
			</Button>
			<Button onclick={render} disabled={working || !doc.audio || !doc.slots?.length}>Render video</Button>
			{#if busy}
				<p role="status" class="text-sm">
					{busy}
					{#if working}
						<span class="text-muted-foreground"> {clock(elapsed)}</span>
					{/if}
				</p>
			{/if}
		</div>
	{/if}
	</fieldset>
	{#if error && errorSlot === 0}
		<p role="alert">{error}</p>
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
	.mark-n {
		display: none;
	}
	@container (min-width: 1.25rem) {
		.mark-n {
			display: block;
		}
	}
</style>

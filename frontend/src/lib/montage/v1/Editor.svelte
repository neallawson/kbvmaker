<script lang="ts">
	import { Button } from '#lib/components/ui/button';
	import type { Host } from '#lib/platform/host';
	import { RuleError, type Client, type MontageFile, type Plan } from './client';
	import {
		addPictures,
		applyLook,
		baseName,
		moveSlot,
		newProject,
		mediaPath,
		projectDir,
		relativize,
		removeSlot,
		replaceSrc,
		scoreBlocks,
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
				if (live) {
					plan = next;
					if (next.song != null) {
						songSeconds = next.song;
					}
					error = '';
					errorSlot = 0;
				}
			})
			.catch((err: unknown) => {
				if (!live) {
					return;
				}
				plan = null;
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

	function edit(next: MontageFile) {
		doc = next;
		dirty = true;
		lengthDraft = null;
	}

	function start(path: string) {
		filePath = path;
		doc = newProject();
		look = 'classic';
		selected = 0;
		dirty = true;
		error = '';
		plan = null;
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
			edit(setDur(doc, selected, null));
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

	function sharesSong(file: MontageFile): boolean {
		return (file.timing?.policy ?? 'fit-audio') === 'fit-audio';
	}

	function choose(index: number) {
		selected = index;
		lengthDraft = null;
	}
</script>

<main class="mx-auto flex max-w-3xl flex-col gap-4 p-6">
	<header class="flex flex-wrap items-baseline justify-between gap-2">
		<h1 class="text-2xl font-semibold">kbvmaker</h1>
		<p class="text-muted-foreground text-sm">{dirty ? 'Unsaved changes' : filePath ? 'Saved' : ''}</p>
	</header>

	<fieldset class="flex min-w-0 flex-col gap-4" disabled={working} aria-busy={working}>
	<div class="flex flex-wrap gap-2">
		{#if !host.pickSaveMontage}
			<input
				class="border-input bg-background h-9 min-w-64 rounded-md border px-3"
				aria-label="Project file"
				bind:value={pathDraft}
			/>
		{/if}
		<Button onclick={newProjectFile} disabled={working}>New</Button>
		<Button variant="outline" onclick={() => openProject()} disabled={working}>Open</Button>
		<Button variant="outline" onclick={save} disabled={working || !doc}>Save</Button>
	</div>
	{#if filePath}
		<p class="text-sm">{filePath}</p>
	{/if}

	{#if doc}
		<section class="flex flex-col gap-2">
			<h2 class="font-medium">Song</h2>
			<div class="flex flex-wrap gap-2">
				{#if !host.pickAudio}
					<input
						class="border-input bg-background h-9 min-w-64 rounded-md border px-3"
						aria-label="Song file"
						bind:value={songDraft}
					/>
				{/if}
				<Button variant="outline" onclick={() => addSong()}>
					{doc.audio ? 'Change song' : 'Add song'}
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

			<div class="bg-muted relative h-16 overflow-hidden rounded-md" role="group" aria-label="Score">
				{#each scoreBlocks(doc.slots, plan) as block, index (block.src + index)}
					<button
						type="button"
						class="absolute top-1 h-14 overflow-hidden rounded border px-1 text-left text-xs {selected === index
							? 'border-primary bg-background z-10'
							: 'bg-background/80 border-transparent'}"
						style="left: {block.left}%; width: {block.width}%"
						aria-pressed={selected === index}
						onclick={() => choose(index)}
					>
						{baseName(block.src)}
					</button>
				{:else}
					<p class="text-muted-foreground p-2 text-sm">Each picture shows up here as you add it.</p>
				{/each}
			</div>
			{#if plan}
				<p>{describePlan(plan)}</p>
			{/if}

			<ul class="flex flex-col gap-2">
				{#each doc.slots ?? [] as slot, index (slot.src + index)}
					<li class="rounded-md border p-3 {selected === index ? 'border-primary' : ''}">
						<button type="button" class="w-full text-left" onclick={() => choose(index)}>
							{index + 1}. {baseName(slot.src)}
							{#if plan?.slots[index]}
								<span class="text-muted-foreground"> · {secondsLabel(plan.slots[index].clipDur)}s</span>
							{/if}
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
										disabled={(doc.slots?.length ?? 0) < 2 || !sharesSong(doc)}
										value={lengthValue()}
										oninput={(event) => (lengthDraft = event.currentTarget.value)}
										onchange={(event) => commitLength(event.currentTarget.value)}
									/>
								</label>
								{#if !sharesSong(doc)}
									<p class="text-muted-foreground text-sm">This look uses the same length for every picture.</p>
								{:else if (doc.slots?.length ?? 0) < 2}
									<p class="text-muted-foreground text-sm">This picture runs for the whole song.</p>
								{/if}
								<div class="flex flex-wrap gap-2">
									{#if slot.dur != null}
										<Button variant="outline" onclick={() => edit(setDur(doc!, index, null))}>Share the rest</Button>
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

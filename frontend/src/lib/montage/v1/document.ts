import type { MontageFile, Picture } from './client';

export type LookId = 'classic' | 'photo-film' | 'quick-slides';

export const moves = [
	{ id: 'in-center', label: 'Slow zoom in' },
	{ id: 'in-left-right', label: 'Pan across' },
	{ id: 'in-top-bottom', label: 'Drift down' },
	{ id: 'out-center', label: 'Slow zoom out' },
	{ id: 'hold', label: 'Hold still' }
] as const;

interface Look {
	canvas: NonNullable<MontageFile['canvas']>;
	defaults: NonNullable<MontageFile['defaults']>;
	timing: NonNullable<MontageFile['timing']>;
	cycle: string[];
}

const black = { kind: 'color' as const, color: '#000000' };
const canvas = { width: 3840, height: 2160 };

const looks: Record<LookId, Look> = {
	classic: {
		canvas: { ...canvas, backdrop: black },
		defaults: { join: { type: 'fade', duration: 1 }, hold: 0.5, zoomSpan: 0.25, ease: 'linear' },
		timing: { policy: 'fit-audio' },
		cycle: ['in-center', 'out-center']
	},
	'photo-film': {
		canvas: { ...canvas, backdrop: { kind: 'blur', blur: 40 } },
		defaults: { join: { type: 'fade', duration: 1.5 }, hold: 1, zoomSpan: 0.4, ease: 'linear' },
		timing: { policy: 'fit-audio' },
		cycle: ['in-center', 'in-left-right', 'in-top-bottom', 'out-center']
	},
	'quick-slides': {
		canvas: { ...canvas, backdrop: black },
		defaults: { join: { type: 'fade', duration: 0.5 }, hold: 0, zoomSpan: 0.2, ease: 'linear' },
		timing: { policy: 'fixed-clip', clip: 5 },
		cycle: ['in-center']
	}
};

export function newProject(): MontageFile {
	return applyLook({ montage: 1, slots: [] }, 'classic');
}

export function applyLook(file: MontageFile, id: LookId): MontageFile {
	const look = looks[id];
	const slots = (file.slots ?? []).map((slot, index) => {
		const next: Picture = { ...slot, mode: look.cycle[index % look.cycle.length] };
		if (look.timing.policy !== 'fit-audio') {
			delete next.dur;
		}
		return next;
	});
	return {
		...file,
		canvas: look.canvas,
		output: { ...file.output, width: 1920, height: 1080 },
		defaults: look.defaults,
		timing: look.timing,
		slots
	};
}

export function modeForNewPicture(id: LookId | null, index: number): string {
	if (!id) {
		return 'in-center';
	}
	const cycle = looks[id].cycle;
	return cycle[index % cycle.length];
}

export function addPictures(file: MontageFile, paths: string[], look: LookId | null): MontageFile {
	const slots = [...(file.slots ?? [])];
	for (const src of paths) {
		if (!src) {
			continue;
		}
		slots.push({ src, mode: modeForNewPicture(look, slots.length) });
	}
	return { ...file, slots };
}

export function setAudio(file: MontageFile, audio: string): MontageFile {
	return { ...file, audio };
}

export function moveSlot(file: MontageFile, index: number, delta: -1 | 1): MontageFile {
	const slots = [...(file.slots ?? [])];
	const next = index + delta;
	if (next < 0 || next >= slots.length) {
		return file;
	}
	const [row] = slots.splice(index, 1);
	slots.splice(next, 0, row);
	return { ...file, slots };
}

export function removeSlot(file: MontageFile, index: number): MontageFile {
	return { ...file, slots: (file.slots ?? []).filter((_, i) => i !== index) };
}

export function replaceSrc(file: MontageFile, index: number, src: string): MontageFile {
	return {
		...file,
		slots: (file.slots ?? []).map((slot, i) => (i === index ? { ...slot, src } : slot))
	};
}

export function setMode(file: MontageFile, index: number, mode: string): MontageFile {
	return {
		...file,
		slots: (file.slots ?? []).map((slot, i) => (i === index ? { ...slot, mode } : slot))
	};
}

/** setDur stores a pin. Null removes it so the picture shares the song again. */
export function setDur(file: MontageFile, index: number, dur: number | null): MontageFile {
	return {
		...file,
		slots: (file.slots ?? []).map((slot, i) => {
			if (i !== index) {
				return slot;
			}
			if (dur == null) {
				const { dur: _ignored, ...rest } = slot;
				return rest;
			}
			return { ...slot, dur };
		})
	};
}

export function sortByName(file: MontageFile): MontageFile {
	const slots = [...(file.slots ?? [])].sort((a, b) =>
		baseName(a.src).localeCompare(baseName(b.src), undefined, { numeric: true })
	);
	return { ...file, slots };
}

export function shuffleSlots(file: MontageFile, random: () => number = Math.random): MontageFile {
	const slots = [...(file.slots ?? [])];
	for (let i = slots.length - 1; i > 0; i--) {
		const j = Math.floor(random() * (i + 1));
		[slots[i], slots[j]] = [slots[j], slots[i]];
	}
	return { ...file, slots };
}

export function shuffleMoves(file: MontageFile, random: () => number = Math.random): MontageFile {
	return {
		...file,
		slots: (file.slots ?? []).map((slot) => ({
			...slot,
			mode: moves[Math.floor(random() * moves.length)].id
		}))
	};
}

export function projectDir(path: string): string {
	const i = path.lastIndexOf('/');
	return i >= 0 ? path.slice(0, i) : '';
}

export function baseName(path: string): string {
	const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
	return i >= 0 ? path.slice(i + 1) : path;
}

/** relativize stores paths inside the project directory as relative paths. */
export function relativize(file: MontageFile, dir: string): MontageFile {
	if (!dir) {
		return file;
	}
	const prefix = dir.endsWith('/') ? dir : `${dir}/`;
	const rel = (path: string) => (path.startsWith(prefix) ? path.slice(prefix.length) : path);
	return {
		...file,
		audio: file.audio ? rel(file.audio) : file.audio,
		output: file.output?.file ? { ...file.output, file: rel(file.output.file) } : file.output,
		slots: file.slots?.map((slot) => ({ ...slot, src: rel(slot.src) }))
	};
}

export interface ScoreBlock {
	src: string;
	left: number;
	width: number;
}

/** scoreBlocks lays out the pictures already added. A plan replaces equal widths with clip lengths. */
export function scoreBlocks(
	slots: { src: string }[] | undefined,
	plan: { duration: number; slots: { src: string; clipStart?: number; clipDur: number }[] } | null
): ScoreBlock[] {
	const list = slots ?? [];
	if (!list.length) {
		return [];
	}
	if (plan && plan.duration > 0 && plan.slots.length === list.length) {
		return plan.slots.map((slot) => ({
			src: slot.src,
			left: ((slot.clipStart ?? 0) / plan.duration) * 100,
			width: (slot.clipDur / plan.duration) * 100
		}));
	}
	const width = 100 / list.length;
	return list.map((slot, index) => ({ src: slot.src, left: index * width, width }));
}

export interface ScoreJoin {
	left: number;
	width: number;
}

/** scoreJoins is the overlap of neighboring pictures: the dissolve on the timeline. */
export function scoreJoins(blocks: ScoreBlock[]): ScoreJoin[] {
	const joins: ScoreJoin[] = [];
	for (let i = 0; i < blocks.length - 1; i++) {
		const start = blocks[i + 1].left;
		const end = blocks[i].left + blocks[i].width;
		if (end > start + 0.05) {
			joins.push({ left: start, width: end - start });
		}
	}
	return joins;
}

/** songClock is minutes:seconds and the length in seconds. */
export function songClock(seconds: number): string {
	const total = Math.max(0, Math.round(seconds));
	const min = Math.floor(total / 60);
	const sec = total % 60;
	return `${min}:${String(sec).padStart(2, '0')} · ${total}s`;
}

export function mediaPath(dir: string, path: string): string {
	if (!path || path.startsWith('/') || /^[A-Za-z]:[\\/]/.test(path)) {
		return path;
	}
	if (!dir) {
		return path;
	}
	return `${dir.replace(/\/$/, '')}/${path}`;
}

export function secondsLabel(n: number): string {
	const rounded = Math.round(n * 10) / 10;
	return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
}

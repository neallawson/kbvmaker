import type { MontageFile, Picture } from './client';

export type LookId = 'classic' | 'photo-film' | 'quick-slides';

export const moves = [
	{ id: 'in-center', label: 'Slow zoom in' },
	{ id: 'in-left-right', label: 'Pan right' },
	{ id: 'in-right-left', label: 'Pan left' },
	{ id: 'in-top-bottom', label: 'Drift down' },
	{ id: 'in-bottom-top', label: 'Drift up' },
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
		cycle: ['in-center', 'in-left-right', 'out-center']
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
	const pacedBySong = look.timing.policy === 'fit-audio' && !!file.audio;
	const slots = (file.slots ?? []).map((slot, index) => {
		const next: Picture = { ...slot, mode: look.cycle[index % look.cycle.length] };
		if (!pacedBySong && look.timing.policy !== 'fit-audio') {
			delete next.dur;
		}
		return next;
	});
	if (pacedBySong && slots.length > 1 && slots.every((slot) => slot.dur != null)) {
		const last = slots.length - 1;
		slots[last] = withoutDur(slots[last]);
	}
	const timing =
		pacedBySong || look.timing.policy !== 'fit-audio' ? { ...look.timing } : { policy: 'fixed-clip' as const, clip: 5 };
	return {
		...file,
		canvas: look.canvas,
		output: { ...file.output, width: 1920, height: 1080 },
		defaults: look.defaults,
		timing,
		moves: [...look.cycle],
		slots
	};
}

function modeForNewPicture(file: MontageFile, index: number): string {
	const cycle = file.moves?.length ? file.moves : ['in-center'];
	return cycle[index % cycle.length];
}

export function addPictures(file: MontageFile, paths: string[]): MontageFile {
	const slots = [...(file.slots ?? [])];
	for (const src of paths) {
		if (!src) {
			continue;
		}
		slots.push({ src, mode: modeForNewPicture(file, slots.length) });
	}
	return { ...file, slots };
}

/** moveChoices is every built-in move, plus a picture's current move when the file named another one. */
export function moveChoices(mode?: string): { id: string; label: string }[] {
	const list = moves.map((move) => ({ id: move.id, label: move.label }));
	if (mode && !list.some((move) => move.id === mode)) {
		list.push(moveLabel(mode));
	}
	return list;
}

function moveLabel(id: string): { id: string; label: string } {
	return moves.find((move) => move.id === id) ?? { id, label: id };
}

function movePool(file: MontageFile): readonly string[] {
	return file.moves?.length ? file.moves : moves.map((move) => move.id);
}

/**
 * setAudio attaches a soundtrack.
 * Classic and Photo film, chosen in this session, hand the clock to the song.
 * Quick slides, and a file opened already on a picture clock, keep that clock.
 * A typed film length steps aside so the song and the film are not both the clock.
 */
export function setAudio(file: MontageFile, audio: string, look?: LookId | null): MontageFile {
	const policy = file.timing?.policy ?? 'fit-audio';
	if (look === 'quick-slides' || (look == null && (policy === 'fixed-clip' || policy === 'explicit'))) {
		if (file.timing?.duration == null) {
			return { ...file, audio };
		}
		const timing = { ...file.timing };
		delete timing.duration;
		return { ...file, audio, timing };
	}
	return { ...file, audio, timing: { policy: 'fit-audio' } };
}

/**
 * clearAudio drops the song.
 * A song-paced film keeps each picture's length: `sharedSeconds` is the pace
 * the pictures without their own time were using, and pins stay.
 */
export function clearAudio(file: MontageFile, sharedSeconds: number): MontageFile {
	const { audio: _audio, ...rest } = file;
	if ((file.timing?.policy ?? 'fit-audio') !== 'fit-audio') {
		return rest;
	}
	return { ...rest, timing: { policy: 'fixed-clip', clip: roundSeconds(sharedSeconds) } };
}

/** setTimeEach is the unlocked pace. Pictures without their own length use it, and the film grows with them. */
export function setTimeEach(file: MontageFile, seconds: number): MontageFile {
	return { ...file, timing: { policy: 'fixed-clip', clip: roundSeconds(seconds) } };
}

/**
 * setFilmLength edits the finished video.
 * With no custom times and the lock off, it updates the shared pace and stays unlocked.
 * With the lock on, it stores that length. A custom time leaves the field alone.
 */
export function setFilmLength(file: MontageFile, seconds: number): MontageFile {
	const locked = (file.timing?.policy ?? 'fit-audio') === 'fit-audio';
	if (!locked && hasCustomTime(file)) {
		return file;
	}
	if (!locked) {
		const count = Math.max(file.slots?.length ?? 0, 1);
		return setTimeEach(file, clipFromFilm(count, seconds, joinSeconds(file)));
	}
	return lockFilm(file, seconds);
}

/** lockFilm keeps the finished length. Pins stay, and one picture keeps sharing that length. */
export function lockFilm(file: MontageFile, filmSeconds: number): MontageFile {
	let slots = [...(file.slots ?? [])];
	if (slots.length > 1 && slots.every((slot) => slot.dur != null)) {
		const last = slots.length - 1;
		slots[last] = withoutDur(slots[last]);
	}
	return {
		...file,
		slots,
		timing: { policy: 'fit-audio', duration: roundSeconds(filmSeconds) }
	};
}

/** unlockFilm returns to a shared pace. Pins stay as their own lengths, and the film becomes their sum. */
export function unlockFilm(file: MontageFile, shared: number): MontageFile {
	return { ...file, timing: { policy: 'fixed-clip', clip: roundSeconds(shared) } };
}

export function joinSeconds(file: MontageFile): number {
	return file.defaults?.join?.duration ?? 1;
}

export function hasCustomTime(file: MontageFile): boolean {
	return (file.slots ?? []).some((slot) => slot.dur != null);
}

/** filmFromClips is the finished length after dissolves overlap. */
export function filmFromClips(count: number, lengths: number[], join: number): number {
	if (count <= 0) {
		return 0;
	}
	let sum = 0;
	for (let i = 0; i < count; i++) {
		sum += lengths[i] ?? 0;
		if (i < count - 1) {
			sum -= join;
		}
	}
	return roundSeconds(sum);
}

/** clipFromFilm is the even picture length that finishes at `film` after overlaps. */
export function clipFromFilm(count: number, film: number, join: number): number {
	if (count <= 1) {
		return roundSeconds(film);
	}
	return roundSeconds((film + (count - 1) * join) / count);
}

/** outputLength is the finished film from the file alone, before a plan comes back. */
export function outputLength(file: MontageFile): number {
	if ((file.timing?.policy ?? 'fit-audio') === 'fit-audio' && file.timing?.duration != null) {
		return file.timing.duration;
	}
	const count = file.slots?.length ?? 0;
	if (count === 0) {
		return file.timing?.clip ?? file.timing?.duration ?? 5;
	}
	const clip = file.timing?.clip ?? 5;
	return filmFromClips(
		count,
		(file.slots ?? []).map((slot) => slot.dur ?? clip),
		joinSeconds(file)
	);
}

/** canPlan is true once the pictures and their clock are both in the file. */
export function canPlan(file: MontageFile): boolean {
	if (!file.slots?.length) {
		return false;
	}
	if (file.audio) {
		return true;
	}
	const policy = file.timing?.policy ?? 'fit-audio';
	if (policy === 'fixed-clip' || policy === 'explicit') {
		return true;
	}
	return policy === 'fit-audio' && file.timing?.duration != null;
}

export function moveSlot(file: MontageFile, index: number, delta: -1 | 1): MontageFile {
	return reorderSlot(file, index, index + delta);
}

/** reorderSlot moves one picture to a new index. The picture keeps its pin. */
export function reorderSlot(file: MontageFile, from: number, to: number): MontageFile {
	const slots = [...(file.slots ?? [])];
	if (from === to || from < 0 || to < 0 || from >= slots.length || to >= slots.length) {
		return file;
	}
	const [row] = slots.splice(from, 1);
	slots.splice(to, 0, row);
	return { ...file, slots };
}

/**
 * dropIndex is the index the picture at `from` lands on when released at `at`.
 * `centers` and `at` share one horizontal axis. A point on a center counts as
 * the left side of that center. At or left of the first center lands first.
 * Right of the last center lands last.
 */
export function dropIndex(centers: number[], from: number, at: number): number {
	const n = centers.length;
	if (n === 0 || from < 0 || from >= n) {
		return from;
	}
	let before = n;
	if (at <= centers[0]) {
		before = 0;
	} else if (at > centers[n - 1]) {
		before = n;
	} else {
		for (let i = 0; i < n - 1; i++) {
			if (at <= centers[i + 1]) {
				before = i + 1;
				break;
			}
		}
	}
	let to = before > from ? before - 1 : before;
	if (to < 0) {
		to = 0;
	}
	if (to > n - 1) {
		to = n - 1;
	}
	return to;
}

/**
 * dragLayout lifts `from` out of the strip and leaves a gap of its width at `to`.
 * The other pictures keep their widths and their dissolve overlap, and slide together.
 */
export function dragLayout(blocks: ScoreBlock[], from: number, to: number): DragBlock[] {
	const order = movedOrder(blocks, from, to);
	if (!order) {
		return blocks.map((block, slot) => ({ ...block, slot }));
	}
	return packBlocks(blocks, order).flatMap((block, index) =>
		order[index] === from ? [] : [{ ...block, slot: order[index] }]
	);
}

/** settledLayout is the strip after `from` lands on `to`, with the held picture back in the gap. */
export function settledLayout(blocks: ScoreBlock[], from: number, to: number): ScoreBlock[] {
	const order = movedOrder(blocks, from, to);
	if (!order || from === to) {
		return blocks.map((block) => ({ ...block }));
	}
	return packBlocks(blocks, order);
}

function movedOrder(blocks: ScoreBlock[], from: number, to: number): number[] | null {
	const n = blocks.length;
	if (n === 0 || from < 0 || from >= n || to < 0 || to >= n) {
		return null;
	}
	const order = blocks.map((_, index) => index);
	const [held] = order.splice(from, 1);
	order.splice(to, 0, held);
	return order;
}

function packBlocks(blocks: ScoreBlock[], order: number[]): ScoreBlock[] {
	const overlap = blocks.length > 1 ? Math.max(0, blocks[0].left + blocks[0].width - blocks[1].left) : 0;
	let cursor = 0;
	return order.map((slot, index) => {
		const width = blocks[slot].width;
		const block = { src: blocks[slot].src, left: cursor, width };
		if (index < order.length - 1) {
			cursor += width - overlap;
		}
		return block;
	});
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
		slots: (file.slots ?? []).map((slot, i) => (i === index ? withDur(slot, dur) : slot))
	};
}

function withDur(slot: Picture, dur: number | null): Picture {
	if (dur == null) {
		return withoutDur(slot);
	}
	return { ...slot, dur };
}

function withoutDur(slot: Picture): Picture {
	const { dur: _ignored, ...rest } = slot;
	return rest;
}

const minPastDissolve = 0.001;

/** roundSeconds keeps a stored length to the millisecond. */
export function roundSeconds(n: number): number {
	return Math.round(n * 1000) / 1000;
}

/** resizedLengths is the clip lengths after a divider drag. Null means the drag did not move. */
export function resizedLengths(lengths: number[], join: number, delta: number, dissolve: number): number[] | null {
	if (join < 0 || join + 1 >= lengths.length) {
		return null;
	}
	const applied = clampDelta(lengths[join], lengths[join + 1], delta, dissolve);
	if (Math.abs(applied) < 1e-9) {
		return null;
	}
	return lengths.map((length, i) =>
		roundSeconds(i === join ? lengths[join] + applied : i === join + 1 ? lengths[join + 1] - applied : length)
	);
}

/**
 * resizePictures gives `delta` seconds to the picture left of `join` and takes
 * them from the picture on the right. The film's length stays. The drag stops
 * when either picture would become as short as the dissolve. fit-audio stores
 * pins and keeps one picture unpinned. fixed-clip becomes explicit, with every
 * current length written, then the same exchange.
 */
export function resizePictures(
	file: MontageFile,
	lengths: number[],
	join: number,
	delta: number,
	dissolve: number
): MontageFile {
	const slots = file.slots ?? [];
	const next = resizedLengths(lengths, join, delta, dissolve);
	if (!next || next.length !== slots.length) {
		return file;
	}
	const left = next[join];
	const right = next[join + 1];
	const policy = file.timing?.policy ?? 'fit-audio';
	if (policy === 'fixed-clip' || policy === 'explicit') {
		return {
			...file,
			timing: { policy: 'explicit' },
			slots: slots.map((slot, i) => withDur(slot, next[i]))
		};
	}
	if (policy !== 'fit-audio') {
		return file;
	}
	return { ...file, slots: pinDivider(slots, join, left, right) };
}

function clampDelta(left: number, right: number, delta: number, dissolve: number): number {
	const floor = dissolve + minPastDissolve;
	const lo = floor - left;
	const hi = right - floor;
	if (!(hi > lo)) {
		return 0;
	}
	return Math.min(hi, Math.max(lo, delta));
}

function pinDivider(slots: Picture[], join: number, left: number, right: number): Picture[] {
	const unpinned = slots.filter((slot) => slot.dur == null).length;
	const leftIsSlack = slots[join].dur == null;
	const rightIsSlack = slots[join + 1].dur == null;
	let pinLeft = true;
	let pinRight = true;
	if (slots.length === 2) {
		pinRight = false;
	} else if (leftIsSlack && unpinned === 1) {
		pinLeft = false;
	} else if (rightIsSlack && (unpinned === 1 || (leftIsSlack && unpinned === 2))) {
		pinRight = false;
	}
	return slots.map((slot, i) => {
		if (i === join) {
			return pinLeft ? withDur(slot, left) : withoutDur(slot);
		}
		if (i === join + 1) {
			return pinRight ? withDur(slot, right) : withoutDur(slot);
		}
		return slot;
	});
}

/** blocksFromLengths draws clip lengths that overlap by the dissolve. */
export function blocksFromLengths(srcs: string[], lengths: number[], dissolve: number): ScoreBlock[] {
	if (!lengths.length || lengths.length !== srcs.length) {
		return [];
	}
	const starts = [0];
	for (let i = 0; i < lengths.length - 1; i++) {
		starts.push(starts[i] + lengths[i] - dissolve);
	}
	const total = starts[starts.length - 1] + lengths[lengths.length - 1];
	if (total <= 0) {
		return [];
	}
	return srcs.map((src, i) => ({
		src,
		left: (starts[i] / total) * 100,
		width: (lengths[i] / total) * 100
	}));
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
	const pool = movePool(file);
	return {
		...file,
		slots: (file.slots ?? []).map((slot) => ({
			...slot,
			mode: pool[Math.floor(random() * pool.length)]
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

/** DragBlock is a picture that stays on the strip while another is held. `slot` is its place in the film. */
export interface DragBlock extends ScoreBlock {
	slot: number;
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

/**
 * gripHit is the grab area for the divider between two pictures.
 * Each picture keeps a clickable center, so a short picture is not covered by both grips.
 * Lengths are percentages of the track. The result is in pixels. Null means there is no room to grab.
 */
export function gripHit(
	left: { left: number; width: number },
	right: { left: number; width: number },
	joinCenter: number,
	trackWidth: number,
	wanted = 28,
	minSelect = 24
): { left: number; width: number } | null {
	if (trackWidth <= 0) {
		return null;
	}
	const px = (percent: number) => (percent / 100) * trackWidth;
	const leftBox = selectBand(px(left.left), px(left.left + left.width), minSelect);
	const rightBox = selectBand(px(right.left), px(right.left + right.width), minSelect);
	const corridorStart = leftBox.reservedEnd;
	const corridorEnd = rightBox.reservedStart;
	const room = corridorEnd - corridorStart;
	if (room < 8) {
		return null;
	}
	const width = Math.min(wanted, room);
	let hitLeft = px(joinCenter) - width / 2;
	if (hitLeft < corridorStart) {
		hitLeft = corridorStart;
	}
	if (hitLeft + width > corridorEnd) {
		hitLeft = corridorEnd - width;
	}
	return { left: hitLeft, width };
}

function selectBand(start: number, end: number, minSelect: number): { reservedStart: number; reservedEnd: number } {
	const width = end - start;
	if (width <= minSelect) {
		return { reservedStart: start, reservedEnd: end };
	}
	const pad = (width - minSelect) / 2;
	return { reservedStart: start + pad, reservedEnd: end - pad };
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

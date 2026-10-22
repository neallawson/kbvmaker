import { describe, expect, it } from 'vitest';
import type { MontageFile } from './client';
import {
	addPictures,
	applyLook,
	canPlan,
	clipFromFilm,
	clearAudio,
	dragLayout,
	filmFromClips,
	settledLayout,
	dropIndex,
	gripHit,
	moveSlot,
	newProject,
	relativize,
	reorderSlot,
	resizePictures,
	scoreBlocks,
	scoreJoins,
	lockFilm,
	setAudio,
	setDur,
	setFilmLength,
	setTimeEach,
	unlockFilm,
	shuffleMoves,
	shuffleSlots,
	songClock,
	sortByName
} from './document';

function songFilm(): MontageFile {
	return { ...newProject(), audio: 'song.mp3', timing: { policy: 'fit-audio' } };
}

describe('score and song length', () => {
	it('gives each new picture an equal block until a plan exists', () => {
		const blocks = scoreBlocks([{ src: 'a.jpg' }, { src: 'b.jpg' }], null);
		expect(blocks.map((block) => block.width)).toEqual([50, 50]);
		expect(blocks.map((block) => block.left)).toEqual([0, 50]);
	});

	it('uses the plan once every picture has a length', () => {
		const blocks = scoreBlocks([{ src: 'a.jpg' }, { src: 'b.jpg' }], {
			duration: 20,
			slots: [
				{ src: 'a.jpg', clipStart: 0, clipDur: 8 },
				{ src: 'b.jpg', clipStart: 6.5, clipDur: 13.5 }
			]
		});
		expect(blocks[0]).toMatchObject({ left: 0, width: 40 });
		expect(blocks[1].left).toBeCloseTo(32.5);
		expect(blocks[1].width).toBeCloseTo(67.5);
		const [join] = scoreJoins(blocks);
		expect(join.left).toBeCloseTo(32.5);
		expect(join.width).toBeCloseTo(7.5);
	});

	it('draws no join when the pictures only meet', () => {
		expect(scoreJoins(scoreBlocks([{ src: 'a.jpg' }, { src: 'b.jpg' }], null))).toEqual([]);
	});

	it('writes the song length as minutes:seconds and total seconds', () => {
		expect(songClock(222)).toBe('3:42 · 222s');
		expect(songClock(4)).toBe('0:04 · 4s');
	});
});

describe('project edits', () => {
	it('starts from Classic with no pictures', () => {
		const file = newProject();
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(file.defaults?.join?.duration).toBe(1);
		expect(file.canvas?.backdrop).toEqual({ kind: 'color', color: '#000000' });
		expect(file.slots).toEqual([]);
		expect(file.moves).toEqual(['in-center', 'out-center']);
	});

	it('cycles Photo film through zoom in, pan right, and zoom out', () => {
		const file = addPictures(applyLook(newProject(), 'photo-film'), ['a.jpg', 'b.jpg', 'c.jpg', 'd.jpg']);
		expect(file.slots?.map((slot) => slot.mode)).toEqual(['in-center', 'in-left-right', 'out-center', 'in-center']);
		expect(file.moves).toEqual(['in-center', 'in-left-right', 'out-center']);
	});

	it('appends pictures in pick order and alternates Classic moves', () => {
		const file = addPictures(newProject(), ['b.jpg', 'a.jpg']);
		expect(file.slots?.map((slot) => slot.src)).toEqual(['b.jpg', 'a.jpg']);
		expect(file.slots?.map((slot) => slot.mode)).toEqual(['in-center', 'out-center']);
	});

	it('moves a row without dropping its pin', () => {
		let file = addPictures(newProject(), ['a.jpg', 'b.jpg']);
		file = setDur(file, 1, 40);
		file = moveSlot(file, 1, -1);
		expect(file.slots?.[0]).toMatchObject({ src: 'b.jpg', dur: 40 });
		expect(file.slots?.[1].dur).toBeUndefined();
	});

	it('sorts by file name once', () => {
		const file = sortByName(addPictures(newProject(), ['b.jpg', 'a10.jpg', 'a2.jpg']));
		expect(file.slots?.map((slot) => slot.src)).toEqual(['a2.jpg', 'a10.jpg', 'b.jpg']);
	});

	it('stores only the typed length', () => {
		let file = setDur(addPictures(newProject(), ['a.jpg', 'b.jpg']), 0, 40);
		expect(file.slots?.[0].dur).toBe(40);
		expect(file.slots?.[1].dur).toBeUndefined();
		file = setDur(file, 0, null);
		expect(file.slots?.[0].dur).toBeUndefined();
	});

	it('clears pins when a Look gives every picture the same length', () => {
		let file = setDur(addPictures(newProject(), ['a.jpg', 'b.jpg']), 0, 40);
		file = applyLook(file, 'quick-slides');
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(file.slots?.every((slot) => slot.dur == null && slot.mode === 'in-center')).toBe(true);
	});

	it('shuffles order and moves with the numbers it is given', () => {
		const file = addPictures(newProject(), ['a.jpg', 'b.jpg', 'c.jpg']);
		const seq = [0.9, 0];
		let n = 0;
		const random = () => seq[n++] ?? 0;
		expect(shuffleSlots(file, random).slots?.map((slot) => slot.src)).toEqual(['b.jpg', 'a.jpg', 'c.jpg']);
		const moves = shuffleMoves(file, () => 0.99).slots?.map((slot) => slot.mode);
		expect(moves).toEqual(['out-center', 'out-center', 'out-center']);
	});

	it('shuffles inside the Look’s move list', () => {
		const file = addPictures(applyLook(newProject(), 'photo-film'), ['a.jpg', 'b.jpg']);
		expect(shuffleMoves(file, () => 0).slots?.map((slot) => slot.mode)).toEqual(['in-center', 'in-center']);
		expect(shuffleMoves(file, () => 0.99).slots?.map((slot) => slot.mode)).toEqual(['out-center', 'out-center']);
	});

	it('shuffles every built-in move when the file has no list', () => {
		const file: MontageFile = { montage: 1, slots: [{ src: 'a.jpg', mode: 'in-center' }] };
		expect(shuffleMoves(file, () => 0.99).slots?.[0].mode).toBe('hold');
		expect(shuffleMoves(file, () => 0).slots?.[0].mode).toBe('in-center');
	});

	it('keeps a short picture clickable between its grips', () => {
		const wide = { left: 0, width: 50 };
		const next = { left: 45, width: 50 };
		const centered = gripHit(wide, next, 47.5, 400);
		expect(centered).toEqual({ left: 176, width: 28 });

		const track = 720;
		const film = 200;
		const long = { left: 0, width: (25 / film) * 100 };
		const short = { left: ((25 - 1) / film) * 100, width: (7 / film) * 100 };
		const join = short.left + 0.5 * (1 / film) * 100;
		const hit = gripHit(long, short, join, track);
		expect(hit).not.toBeNull();
		const shortStart = (short.left / 100) * track;
		const shortEnd = shortStart + (short.width / 100) * track;
		const reservedStart = shortStart + (shortEnd - shortStart - 24) / 2;
		expect(hit!.left + hit!.width).toBeLessThanOrEqual(reservedStart + 0.01);
		expect(gripHit({ left: 0, width: 2 }, { left: 1, width: 2 }, 1.5, 80)).toBeNull();
	});

	it('drops a picture by the centers on the timeline', () => {
		const centers = [10, 50, 90];
		expect(dropIndex(centers, 1, 10)).toBe(0);
		expect(dropIndex(centers, 1, 0)).toBe(0);
		expect(dropIndex(centers, 2, 10)).toBe(0);
		expect(dropIndex(centers, 0, 100)).toBe(2);
		expect(dropIndex(centers, 1, 100)).toBe(2);
		expect(dropIndex(centers, 0, 90)).toBe(1);
		expect(dropIndex(centers, 2, 30)).toBe(1);
		expect(dropIndex(centers, 0, 70)).toBe(1);
		expect(dropIndex(centers, 1, 50)).toBe(1);
		expect(dropIndex(centers, 1, 70)).toBe(1);
	});

	it('opens a gap the width of the held picture', () => {
		const even = [
			{ src: 'a', left: 0, width: 50 },
			{ src: 'b', left: 50, width: 50 }
		];
		expect(dragLayout(even, 1, 0)).toEqual([{ src: 'a', left: 50, width: 50, slot: 0 }]);
		expect(dragLayout(even, 0, 1)).toEqual([{ src: 'b', left: 0, width: 50, slot: 1 }]);
		expect(dragLayout(even, 1, 1)).toEqual([{ src: 'a', left: 0, width: 50, slot: 0 }]);

		const third = 100 / 3;
		const three = ['a', 'b', 'c'].map((src, index) => ({ src, left: index * third, width: third }));
		const laid = dragLayout(three, 2, 0);
		expect(laid.map((block) => block.slot)).toEqual([0, 1]);
		expect(laid[0].left).toBeCloseTo(third);
		expect(laid[1].left).toBeCloseTo(third * 2);

		const overlapped = [
			{ src: 'a', left: 0, width: 56.25 },
			{ src: 'b', left: 43.75, width: 56.25 }
		];
		expect(dragLayout(overlapped, 1, 0)).toEqual([{ src: 'a', left: 43.75, width: 56.25, slot: 0 }]);
		expect(settledLayout(even, 1, 0)).toEqual([
			{ src: 'b', left: 0, width: 50 },
			{ src: 'a', left: 50, width: 50 }
		]);
		expect(settledLayout(overlapped, 1, 0)).toEqual([
			{ src: 'b', left: 0, width: 56.25 },
			{ src: 'a', left: 43.75, width: 56.25 }
		]);
	});

	it('reorders a picture and keeps its pin', () => {
		let file = setDur(addPictures(newProject(), ['a.jpg', 'b.jpg', 'c.jpg']), 2, 8);
		file = reorderSlot(file, 2, 0);
		expect(file.slots?.map((slot) => slot.src)).toEqual(['c.jpg', 'a.jpg', 'b.jpg']);
		expect(file.slots?.[0].dur).toBe(8);
		expect(reorderSlot(file, 0, 0)).toBe(file);
	});

	it('pins the two pictures around a divider and leaves the rest sharing', () => {
		const file = addPictures(songFilm(), ['a.jpg', 'b.jpg', 'c.jpg']);
		const next = resizePictures(file, [8, 8, 8], 0, 2, 1);
		expect(next.timing).toEqual({ policy: 'fit-audio' });
		expect(next.slots?.map((slot) => slot.dur)).toEqual([10, 6, undefined]);
	});

	it('pins one of two pictures so the other keeps absorbing the song', () => {
		const file = addPictures(songFilm(), ['a.jpg', 'b.jpg']);
		const next = resizePictures(file, [10.5, 10.5], 0, 1, 1);
		expect(next.slots?.map((slot) => slot.dur)).toEqual([11.5, undefined]);
	});

	it('keeps already pinned neighbors and the picture that still shares', () => {
		let file = addPictures(songFilm(), ['a.jpg', 'b.jpg', 'c.jpg']);
		file = setDur(setDur(file, 0, 10), 1, 8);
		const next = resizePictures(file, [10, 8, 6], 0, 1, 1);
		expect(next.slots?.map((slot) => slot.dur)).toEqual([11, 7, undefined]);
	});

	it('lets the unpinned picture on the left absorb the divider drag', () => {
		let file = addPictures(songFilm(), ['a.jpg', 'b.jpg', 'c.jpg']);
		file = setDur(setDur(file, 1, 10), 2, 6);
		const next = resizePictures(file, [6, 10, 6], 0, 1, 1);
		expect(next.slots?.map((slot) => slot.dur)).toEqual([undefined, 9, 6]);
	});

	it('lets the only unpinned picture absorb the divider drag', () => {
		let file = addPictures(songFilm(), ['a.jpg', 'b.jpg', 'c.jpg']);
		file = setDur(setDur(file, 0, 10), 1, 6);
		const next = resizePictures(file, [10, 6, 6], 1, 1, 1);
		expect(next.slots?.map((slot) => slot.dur)).toEqual([10, 7, undefined]);
	});

	it('stops the divider when a picture would be as short as the dissolve', () => {
		const file = addPictures(songFilm(), ['a.jpg', 'b.jpg']);
		const next = resizePictures(file, [10, 10], 0, 100, 1);
		expect(next.slots?.[0].dur).toBeCloseTo(18.999, 3);
		expect(next.slots?.[1].dur).toBeUndefined();
	});

	it('leaves fixed-clip when a divider gives each picture its own length', () => {
		let file = applyLook(addPictures(newProject(), ['a.jpg', 'b.jpg', 'c.jpg']), 'quick-slides');
		file = resizePictures(file, [5, 5, 5], 0, 1, 0.5);
		expect(file.timing).toEqual({ policy: 'explicit' });
		expect(file.slots?.map((slot) => slot.dur)).toEqual([6, 4, 5]);
	});

	it('unpins the last picture when a fit-audio look would pin every one', () => {
		let file = applyLook(addPictures(newProject(), ['a.jpg', 'b.jpg']), 'quick-slides');
		file = resizePictures(file, [5, 5], 0, 1, 0.5);
		file = applyLook({ ...file, audio: 'song.mp3' }, 'classic');
		expect(file.timing).toEqual({ policy: 'fit-audio' });
		expect(file.slots?.[0].dur).toBe(6);
		expect(file.slots?.[1].dur).toBeUndefined();
	});

	it('keeps paths inside the project directory relative', () => {
		const file = relativize(
			{
				...addPictures(setAudio(newProject(), '/films/album/song.mp3'), ['/films/album/a.jpg']),
				output: { file: '/films/album/out.mp4' }
			},
			'/films/album'
		);
		expect(file.audio).toBe('song.mp3');
		expect(file.slots?.[0].src).toBe('a.jpg');
		expect(file.output?.file).toBe('out.mp4');
	});

	it('turns a typed film length and a time for each into the same clock', () => {
		const join = 1;
		const count = 10;
		expect(clipFromFilm(count, 41, join)).toBe(5);
		expect(filmFromClips(count, Array(count).fill(5), join)).toBe(41);
		let file = setTimeEach(addPictures(newProject(), ['a.jpg', 'b.jpg']), 8);
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 8 });
		file = setFilmLength(file, 17);
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 9 });
		file = setDur(file, 0, 12);
		expect(setFilmLength(file, 30)).toBe(file);
	});

	it('locks the finished length and keeps one picture sharing it', () => {
		let file = setDur(setDur(addPictures(newProject(), ['a.jpg', 'b.jpg']), 0, 6), 1, 6);
		file = lockFilm(file, 11);
		expect(file.timing).toEqual({ policy: 'fit-audio', duration: 11 });
		expect(file.slots?.[0].dur).toBe(6);
		expect(file.slots?.[1].dur).toBeUndefined();
		file = unlockFilm(file, 5);
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(file.slots?.[0].dur).toBe(6);
	});

	it('lets a Classic soundtrack replace a typed film length and keeps Quick slides', () => {
		let file = lockFilm(addPictures(newProject(), ['a.jpg']), 41);
		file = setDur(file, 0, 41);
		file = setAudio(file, 'song.mp3', 'classic');
		expect(file.audio).toBe('song.mp3');
		expect(file.timing).toEqual({ policy: 'fit-audio' });
		expect(file.slots?.[0].dur).toBe(41);
		file = clearAudio(file, 41);
		expect(file.audio).toBeUndefined();
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 41 });

		const slides = setAudio(applyLook(newProject(), 'quick-slides'), 'song.mp3', 'quick-slides');
		expect(slides.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(clearAudio(slides, 9).timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(clearAudio(slides, 9).audio).toBeUndefined();
	});

	it('keeps an opened picture clock when a song is added', () => {
		const file = setAudio(setTimeEach(newProject(), 8), 'song.mp3');
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 8 });
		expect(canPlan({ ...file, slots: [{ src: 'a.jpg' }] })).toBe(true);
		expect(canPlan(newProject())).toBe(false);
		expect(canPlan(lockFilm(addPictures(newProject(), ['a.jpg']), 12))).toBe(true);
	});

	it('starts Classic without a song at five seconds and keeps a pin', () => {
		let file = setDur(addPictures(newProject(), ['a.jpg', 'b.jpg']), 0, 9);
		file = applyLook(file, 'classic');
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(file.slots?.[0].dur).toBe(9);
		expect(file.slots?.[1].dur).toBeUndefined();
		file = applyLook({ ...file, audio: 'song.mp3' }, 'photo-film');
		expect(file.timing).toEqual({ policy: 'fit-audio' });
		expect(file.slots?.[0].dur).toBe(9);
	});
});

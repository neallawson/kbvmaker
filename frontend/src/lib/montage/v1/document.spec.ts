import { describe, expect, it } from 'vitest';
import {
	addPictures,
	applyLook,
	moveSlot,
	newProject,
	relativize,
	scoreBlocks,
	setAudio,
	setDur,
	shuffleMoves,
	shuffleSlots,
	songClock,
	sortByName
} from './document';

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
	});

	it('writes the song length as minutes:seconds and total seconds', () => {
		expect(songClock(222)).toBe('3:42 · 222s');
		expect(songClock(4)).toBe('0:04 · 4s');
	});
});

describe('project edits', () => {
	it('starts from Classic with no pictures', () => {
		const file = newProject();
		expect(file.timing).toEqual({ policy: 'fit-audio' });
		expect(file.defaults?.join?.duration).toBe(1);
		expect(file.canvas?.backdrop).toEqual({ kind: 'color', color: '#000000' });
		expect(file.slots).toEqual([]);
	});

	it('appends pictures in pick order and alternates Classic moves', () => {
		const file = addPictures(newProject(), ['b.jpg', 'a.jpg'], 'classic');
		expect(file.slots?.map((slot) => slot.src)).toEqual(['b.jpg', 'a.jpg']);
		expect(file.slots?.map((slot) => slot.mode)).toEqual(['in-center', 'out-center']);
	});

	it('moves a row without dropping its pin', () => {
		let file = addPictures(newProject(), ['a.jpg', 'b.jpg'], 'classic');
		file = setDur(file, 1, 40);
		file = moveSlot(file, 1, -1);
		expect(file.slots?.[0]).toMatchObject({ src: 'b.jpg', dur: 40 });
		expect(file.slots?.[1].dur).toBeUndefined();
	});

	it('sorts by file name once', () => {
		const file = sortByName(addPictures(newProject(), ['b.jpg', 'a10.jpg', 'a2.jpg'], 'classic'));
		expect(file.slots?.map((slot) => slot.src)).toEqual(['a2.jpg', 'a10.jpg', 'b.jpg']);
	});

	it('stores only the typed length', () => {
		let file = setDur(addPictures(newProject(), ['a.jpg', 'b.jpg'], 'classic'), 0, 40);
		expect(file.slots?.[0].dur).toBe(40);
		expect(file.slots?.[1].dur).toBeUndefined();
		file = setDur(file, 0, null);
		expect(file.slots?.[0].dur).toBeUndefined();
	});

	it('clears pins when a Look gives every picture the same length', () => {
		let file = setDur(addPictures(newProject(), ['a.jpg', 'b.jpg'], 'classic'), 0, 40);
		file = applyLook(file, 'quick-slides');
		expect(file.timing).toEqual({ policy: 'fixed-clip', clip: 5 });
		expect(file.slots?.every((slot) => slot.dur == null && slot.mode === 'in-center')).toBe(true);
	});

	it('shuffles order and moves with the numbers it is given', () => {
		const file = addPictures(newProject(), ['a.jpg', 'b.jpg', 'c.jpg'], 'classic');
		const seq = [0.9, 0];
		let n = 0;
		const random = () => seq[n++] ?? 0;
		expect(shuffleSlots(file, random).slots?.map((slot) => slot.src)).toEqual(['b.jpg', 'a.jpg', 'c.jpg']);
		const moves = shuffleMoves(file, () => 0.99).slots?.map((slot) => slot.mode);
		expect(moves).toEqual(['hold', 'hold', 'hold']);
	});

	it('keeps paths inside the project directory relative', () => {
		const file = relativize(
			{
				...addPictures(setAudio(newProject(), '/films/album/song.mp3'), ['/films/album/a.jpg'], 'classic'),
				output: { file: '/films/album/out.mp4' }
			},
			'/films/album'
		);
		expect(file.audio).toBe('song.mp3');
		expect(file.slots?.[0].src).toBe('a.jpg');
		expect(file.output?.file).toBe('out.mp4');
	});
});

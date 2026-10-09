import { describe, expect, it } from 'vitest';
import type { Plan } from './client';
import { describePlan } from './summary';

function plan(partial: Partial<Plan> & Pick<Plan, 'slots' | 'duration'>): Plan {
	return { montage: 1, ...partial };
}

describe('describePlan', () => {
	it('describes equal clips and the dissolve between them', () => {
		const text = describePlan(
			plan({
				duration: 20,
				slots: [
					{ src: 'a.jpg', clipDur: 10.75, join: { type: 'fade', duration: 1.5 } },
					{ src: 'b.jpg', clipDur: 10.75 }
				]
			})
		);
		expect(text).toBe('2 pictures, about 10.8s each, with a 1.5s dissolve between them');
	});

	it('does not claim equal length when the clips differ', () => {
		const text = describePlan(
			plan({
				duration: 12,
				slots: [
					{ src: 'a.jpg', clipDur: 4, join: { type: 'fade', duration: 1 } },
					{ src: 'b.jpg', clipDur: 9 }
				]
			})
		);
		expect(text).toBe('2 pictures, 12s altogether, with a 1s dissolve between them');
	});
});
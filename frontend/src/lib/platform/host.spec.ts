import { describe, expect, it } from 'vitest';
import { currentHost } from './host';

describe('currentHost', () => {
	it('is the browser when the window runtime is absent', () => {
		expect(currentHost().name).toBe('browser');
		expect(currentHost().pickMontage).toBeUndefined();
	});
});
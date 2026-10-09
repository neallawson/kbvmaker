import type { Plan } from './client';

// describePlan is the line under a Look: how many pictures, how long each
// one stays, and that a dissolve sits between them.
export function describePlan(plan: Plan): string {
	const n = plan.slots.length;
	const pictures = n === 1 ? '1 picture' : `${n} pictures`;
	if (n <= 1) {
		return `${pictures}, ${seconds(plan.duration)}`;
	}
	const first = plan.slots[0]?.clipDur ?? 0;
	const equal = plan.slots.every((slot) => Math.abs(slot.clipDur - first) < 0.05);
	const fade = plan.slots
		.slice(0, -1)
		.map((slot) => slot.join)
		.find((join) => join?.type === 'fade');
	const dissolve = fade ? `, with a ${seconds(fade.duration)} dissolve between them` : '';
	if (equal) {
		return `${pictures}, about ${seconds(first)} each${dissolve}`;
	}
	return `${pictures}, ${seconds(plan.duration)} altogether${dissolve}`;
}

function seconds(n: number): string {
	const rounded = Math.round(n * 10) / 10;
	const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
	return `${text}s`;
}

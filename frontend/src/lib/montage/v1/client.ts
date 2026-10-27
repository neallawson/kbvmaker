export class RuleError extends Error {
	slot: number;
	code: string;

	constructor(slot: number, code: string, message: string) {
		super(message);
		this.name = 'RuleError';
		this.slot = slot;
		this.code = code;
	}
}

export interface Card {
	title?: string;
	subtitle?: string;
	line?: string;
	credits?: string[];
	align?: 'left' | 'center' | 'right';
	vAlign?: 'upper' | 'middle' | 'lower';
	size?: 'small' | 'medium' | 'large';
	weight?: 'regular' | 'bold';
	face?: 'sans' | 'serif';
	color?: string;
	fill?: string;
	box?: boolean;
}

export interface PlanTrack {
	src: string;
	start: number;
	dur: number;
	gap?: number;
}

export interface Plan {
	montage: number;
	song?: number;
	duration: number;
	tracks?: PlanTrack[];
	slots: {
		src?: string;
		card?: Card;
		clipStart?: number;
		clipDur: number;
		join?: { type: string; duration: number };
	}[];
}

export interface Backdrop {
	kind: 'blur' | 'color' | 'pattern' | 'image';
	blur?: number;
	color?: string;
	name?: string;
	src?: string;
}

/** Slot is one timeline entry. A photograph sets src. A card sets card. Place and time live here. */
export interface Slot {
	src?: string;
	card?: Card;
	mode?: string;
	dur?: number;
	/** fixed keeps this index when the list is shuffled or sorted by name. */
	fixed?: boolean;
}

/** Track is one soundtrack file. gap is seconds of silence before it. */
export interface Track {
	src: string;
	gap?: number;
}

/** MontageFile is the authored project. Derived clip lengths are not fields. */
export interface MontageFile {
	montage: 1;
	tracks?: Track[];
	canvas?: { width?: number; height?: number; backdrop?: Backdrop };
	output?: { width?: number; height?: number; file?: string };
	defaults?: {
		join?: { type: string; duration: number };
		hold?: number;
		zoomSpan?: number;
		ease?: 'linear';
	};
	timing?: { policy: 'fit-audio' | 'fixed-clip' | 'explicit'; clip?: number; duration?: number };
	/** moves is the Look's list, in cycle order. Shuffle moves draws from it. */
	moves?: string[];
	slots?: Slot[];
}

export interface Client {
	version(): Promise<{ montage: number }>;
	audio(path: string): Promise<{ seconds: number }>;
	plan(document: string): Promise<Plan>;
	planBody(body: MontageFile, dir: string): Promise<Plan>;
	read(path: string): Promise<MontageFile>;
	save(path: string, body: MontageFile): Promise<void>;
	exportScript(path: string, file: string): Promise<void>;
	render(path: string): Promise<void>;
	/** thumb is a data URL for a JPEG. contain fits the whole picture inside the box. */
	thumb(path: string, width: number, height: number, fit?: 'contain'): Promise<string>;
	/** thumbCard paints a title or credits card to that same JPEG size. */
	thumbCard(card: Card, width: number, height: number): Promise<string>;
}

export function createClient(fetchImpl: typeof fetch = fetch, base = ''): Client {
	return {
		async version() {
			return readJSON(fetchImpl, `${base}/v1/version`);
		},
		async audio(path) {
			return readJSON(fetchImpl, `${base}/v1/audio`, jsonBody({ path }));
		},
		async plan(document) {
			return readJSON(fetchImpl, `${base}/v1/plan`, jsonBody({ document }));
		},
		async planBody(body, dir) {
			return readJSON(fetchImpl, `${base}/v1/plan`, jsonBody({ body, dir }));
		},
		async read(path) {
			return readJSON(fetchImpl, `${base}/v1/document`, jsonBody({ document: path }));
		},
		async save(path, body) {
			await readJSON(fetchImpl, `${base}/v1/document`, jsonBody({ document: path, body }));
		},
		async exportScript(path, file) {
			await readJSON(fetchImpl, `${base}/v1/export-script`, jsonBody({ document: path, file }));
		},
		async render(path) {
			await readJSON(fetchImpl, `${base}/v1/render`, jsonBody({ document: path }));
		},
		async thumb(path, width, height, fit) {
			const body = await readJSON<{ type: string; data: string }>(
				fetchImpl,
				`${base}/v1/thumbnail`,
				jsonBody({ path, width, height, ...(fit ? { fit } : {}) })
			);
			return `data:${body.type};base64,${body.data}`;
		},
		async thumbCard(card, width, height) {
			const body = await readJSON<{ type: string; data: string }>(
				fetchImpl,
				`${base}/v1/thumbnail`,
				jsonBody({ card, width, height })
			);
			return `data:${body.type};base64,${body.data}`;
		}
	};
}

function jsonBody(body: unknown): RequestInit {
	return {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	};
}

async function readJSON<T>(fetchImpl: typeof fetch, url: string, init?: RequestInit): Promise<T> {
	const res = await fetchImpl(url, init);
	const text = await res.text();
	const body = text ? (JSON.parse(text) as T & { slot?: number; code?: string; message?: string }) : ({} as T);
	if (!res.ok) {
		const err = body as { slot?: number; code?: string; message?: string };
		throw new RuleError(err.slot ?? 0, err.code ?? 'bad-number', err.message ?? res.statusText);
	}
	return body;
}

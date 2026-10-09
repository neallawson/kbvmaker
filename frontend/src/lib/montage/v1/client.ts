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

export interface Plan {
	montage: number;
	duration: number;
	slots: {
		src: string;
		clipDur: number;
		join?: { type: string; duration: number };
	}[];
}

export interface Client {
	version(): Promise<{ montage: number }>;
	plan(document: string): Promise<Plan>;
}

export function createClient(fetchImpl: typeof fetch = fetch, base = ''): Client {
	return {
		async version() {
			return readJSON(fetchImpl, `${base}/v1/version`);
		},
		async plan(document) {
			return readJSON(fetchImpl, `${base}/v1/plan`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ document })
			});
		}
	};
}

async function readJSON<T>(fetchImpl: typeof fetch, url: string, init?: RequestInit): Promise<T> {
	const res = await fetchImpl(url, init);
	const body = (await res.json()) as T & { slot?: number; code?: string; message?: string };
	if (!res.ok) {
		throw new RuleError(body.slot ?? 0, body.code ?? 'bad-number', body.message ?? res.statusText);
	}
	return body;
}

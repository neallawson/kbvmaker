import { host as browser } from './browser';

export interface Host {
	name: 'browser' | 'window';
	/** fetch carries /v1 requests. The window cannot use the WebKit fetch for POST bodies. */
	fetch?: typeof fetch;
	pickMontage?: () => Promise<string>;
	pickSaveMontage?: () => Promise<string>;
	pickImages?: () => Promise<string[]>;
	pickAudio?: () => Promise<string>;
	pickSaveScript?: () => Promise<string>;
	pickSaveVideo?: () => Promise<string>;
	/** setUnsaved tells the window that closing should ask before discarding the film. */
	setUnsaved?: (unsaved: boolean) => void;
}

interface WailsDialogs {
	OpenMontage(): Promise<string>;
	SaveMontage(): Promise<string>;
	OpenImages(): Promise<string[]>;
	OpenAudio(): Promise<string>;
	SaveScript(): Promise<string>;
	SaveVideo(): Promise<string>;
	SetUnsaved?(unsaved: boolean): Promise<void>;
}

interface WailsAPI {
	Call(method: string, path: string, body: string): Promise<{ status: number; body: string }>;
}

interface WailsMain {
	Dialogs?: WailsDialogs;
	API?: WailsAPI;
}

// currentHost is the browser, unless the Wails window injected its dialogs.
export function currentHost(): Host {
	const main = wailsMain();
	const dialogs = main?.Dialogs;
	if (!dialogs) {
		return browser;
	}
	return {
		name: 'window',
		fetch: main.API ? apiFetch(main.API) : undefined,
		pickMontage: () => dialogs.OpenMontage(),
		pickSaveMontage: () => dialogs.SaveMontage(),
		pickImages: () => dialogs.OpenImages(),
		pickAudio: () => dialogs.OpenAudio(),
		pickSaveScript: () => dialogs.SaveScript(),
		pickSaveVideo: () => dialogs.SaveVideo(),
		setUnsaved: dialogs.SetUnsaved ? (unsaved) => void dialogs.SetUnsaved?.(unsaved) : undefined
	};
}

/** apiFetch sends a request through the window's API binding to the same /v1 handler. */
export function apiFetch(api: WailsAPI): typeof fetch {
	return (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
		const path = url.replace(/^[a-z]+:\/\/[^/]*/i, '');
		const body = typeof init?.body === 'string' ? init.body : '';
		const reply = await api.Call(init?.method ?? 'GET', path, body);
		return new Response(reply.body, { status: reply.status });
	}) as typeof fetch;
}

function wailsMain(): WailsMain | undefined {
	const runtime = globalThis as { go?: { main?: WailsMain } };
	return runtime.go?.main;
}

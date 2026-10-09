import { host as browser } from './browser';

export interface Host {
	name: 'browser' | 'window';
	pickMontage?: () => Promise<string>;
}

interface WailsDialogs {
	OpenMontage(): Promise<string>;
}

// currentHost is the browser, unless the Wails window injected its dialogs.
export function currentHost(): Host {
	const dialogs = wailsDialogs();
	if (!dialogs) {
		return browser;
	}
	return {
		name: 'window',
		pickMontage: () => dialogs.OpenMontage()
	};
}

function wailsDialogs(): WailsDialogs | undefined {
	const runtime = globalThis as { go?: { main?: { Dialogs?: WailsDialogs } } };
	return runtime.go?.main?.Dialogs;
}

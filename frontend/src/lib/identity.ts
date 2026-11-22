import info from '../../../identity/identity.json';

declare const __KBVM_VERSION__: string;
declare const __KBVM_BUILT__: string;

export const identity = { ...info, version: __KBVM_VERSION__, built: __KBVM_BUILT__ };

export function identityLine(): string {
	return `${info.name} version ${__KBVM_VERSION__}, ${info.copyright}, built ${__KBVM_BUILT__}`;
}

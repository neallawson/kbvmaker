<script lang="ts">
	import { Button } from '#lib/components/ui/button';
	import { RuleError, type Client } from './client';
	import { describePlan } from './summary';

	let {
		client,
		host,
		pickMontage
	}: {
		client: Client;
		host: string;
		pickMontage?: () => Promise<string>;
	} = $props();

	let documentPath = $state('');
	let montage = $state<number | null>(null);
	let summary = $state('');
	let error = $state('');

	$effect(() => {
		client
			.version()
			.then((v) => {
				montage = v.montage;
			})
			.catch((err: unknown) => {
				error = err instanceof Error ? err.message : 'The API did not answer';
			});
	});

	async function openMontage() {
		if (!pickMontage) {
			return;
		}
		const path = await pickMontage();
		if (path) {
			documentPath = path;
			error = '';
		}
	}

	async function showPlan() {
		error = '';
		summary = '';
		try {
			summary = describePlan(await client.plan(documentPath));
		} catch (err) {
			error = err instanceof RuleError ? err.message : 'Could not read the plan';
		}
	}
</script>

<main class="mx-auto flex max-w-lg flex-col gap-4 p-8">
	<h1 class="text-2xl font-semibold">kbvmaker</h1>
	<p>Montage {montage ?? '…'}</p>
	<p>Host: {host}</p>
	<label class="flex flex-col gap-1 text-sm">
		Montage file
		<input
			class="border-input bg-background h-9 rounded-md border px-3"
			bind:value={documentPath}
		/>
	</label>
	{#if pickMontage}
		<Button variant="outline" onclick={openMontage}>Open</Button>
	{/if}
	<Button onclick={showPlan}>Show plan</Button>
	{#if summary}
		<p>{summary}</p>
	{/if}
	{#if error}
		<p role="alert">{error}</p>
	{/if}
</main>

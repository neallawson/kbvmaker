<script lang="ts">
	import Editor from '#lib/montage/v1/Editor.svelte';
	import { createClient } from '#lib/montage/v1/client';
	import { currentHost } from '#lib/platform/host';

	const host = currentHost();
</script>

<Editor client={createClient(host.fetch)} {host} />

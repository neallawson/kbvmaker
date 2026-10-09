<script lang="ts">
	import PlanPanel from '#lib/montage/v1/PlanPanel.svelte';
	import { createClient } from '#lib/montage/v1/client';
	import { currentHost } from '#lib/platform/host';

	const host = currentHost();
</script>

<PlanPanel client={createClient()} host={host.name} pickMontage={host.pickMontage} />

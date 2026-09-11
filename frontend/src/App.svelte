<script lang="ts">
  import { onDestroy, onMount } from "svelte";

  interface GateSummary {
    name: string;
    namespace: string;
    targetDeployment: string;
    verdict: string;
    message: string;
    readyReplicas: number;
    lastEvaluated?: string;
  }

  let gates: GateSummary[] = [];
  let error: string | null = null;
  let loading = true;
  let pollHandle: ReturnType<typeof setInterval> | undefined;

  async function fetchGates() {
    try {
      const res = await fetch("/api/gates");
      if (!res.ok) {
        throw new Error(`status API returned ${res.status}`);
      }
      gates = await res.json();
      error = null;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    fetchGates();
    pollHandle = setInterval(fetchGates, 5000);
  });

  onDestroy(() => {
    if (pollHandle) clearInterval(pollHandle);
  });

  function verdictClass(verdict: string): string {
    switch (verdict) {
      case "Pass":
        return "verdict-pass";
      case "Warn":
        return "verdict-warn";
      case "Breach":
        return "verdict-breach";
      default:
        return "verdict-unknown";
    }
  }
</script>

<main>
  <h1>Checkout Gates</h1>
  <p class="subtitle">Live status from the checkout-gate-operator's controller cache, polled every 5s.</p>

  {#if loading}
    <p>Loading...</p>
  {:else if error}
    <p class="error">Could not reach the status API: {error}</p>
  {:else if gates.length === 0}
    <p>No CheckoutGate resources found in the cluster.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Target Deployment</th>
          <th>Verdict</th>
          <th>Ready Replicas</th>
          <th>Message</th>
          <th>Last Evaluated</th>
        </tr>
      </thead>
      <tbody>
        {#each gates as gate (gate.namespace + "/" + gate.name)}
          <tr>
            <td>{gate.namespace}/{gate.name}</td>
            <td>{gate.targetDeployment}</td>
            <td><span class="verdict {verdictClass(gate.verdict)}">{gate.verdict}</span></td>
            <td>{gate.readyReplicas}</td>
            <td class="message">{gate.message}</td>
            <td>{gate.lastEvaluated ?? "–"}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</main>

<style>
  main {
    max-width: 960px;
    margin: 2rem auto;
    padding: 0 1rem;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    color: #1a1a1a;
  }
  h1 {
    font-size: 1.5rem;
    margin-bottom: 0.25rem;
  }
  .subtitle {
    color: #666;
    font-size: 0.9rem;
    margin-top: 0;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 1rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid #e0e0e0;
    font-size: 0.9rem;
  }
  th {
    color: #666;
    font-weight: 600;
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 0.03em;
  }
  .message {
    color: #444;
    max-width: 320px;
  }
  .verdict {
    display: inline-block;
    padding: 0.15rem 0.55rem;
    border-radius: 999px;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .verdict-pass {
    background: #e6f4ea;
    color: #1e7e34;
  }
  .verdict-warn {
    background: #fff3cd;
    color: #8a6d00;
  }
  .verdict-breach {
    background: #fdecea;
    color: #b3261e;
  }
  .verdict-unknown {
    background: #eee;
    color: #555;
  }
  .error {
    color: #b3261e;
  }
</style>

# Svelte: a UI framework with no runtime framework

**TL;DR: React and Vue ship a runtime library to the browser that does
the work of tracking state and updating the DOM. Svelte does that work
at build time instead: the compiler turns your component into plain,
imperative JavaScript that updates exactly the DOM nodes that changed,
nothing more. Less shipped code, no virtual DOM diffing at runtime.**

## Compiler, not library

```text
React / Vue:
  your component  -->  ships as-is, PLUS a runtime library  -->  browser
                        the library diffs a virtual DOM on every
                        state change to figure out what to update

Svelte:
  your component  -->  compiled AHEAD OF TIME into plain JS  -->  browser
                        that JS directly mutates the specific DOM
                        nodes tied to whatever variable changed,
                        no diffing step exists at runtime at all
```

This repo's build output makes the difference concrete:
`frontend/dist/assets/index-*.js` for the whole dashboard, fetch
polling, the gates table, the verdict color-coding, all of it, comes
out to about 8.65KB (3.77KB gzipped). There's no framework runtime in
that number, because there isn't one shipped.

## Reading `App.svelte`

A `.svelte` file is three sections in one file, and each one compiles
to a different part of the output:

```svelte
<script lang="ts">
  // becomes the component's JS logic
  let gates: GateSummary[] = [];
  async function fetchGates() { ... }
</script>

<main>
  <!-- becomes the DOM structure, with {gates} as data bindings -->
  {#each gates as gate}
    <tr><td>{gate.name}</td></tr>
  {/each}
</main>

<style>
  /* scoped to this component only, compiled to a
     uniquely-hashed class the compiler adds automatically */
  main { max-width: 960px; }
</style>
```

The `<style>` block is worth calling out specifically: it's scoped by
default, with zero configuration. The compiler generates a unique class
name and rewrites the selectors to include it, so `main { ... }` in
this file can never leak out and affect a `<main>` element somewhere
else on the page, and nothing from anywhere else can leak in.

## Reactivity: `let gates = []` is enough

```svelte
<script lang="ts">
  let gates: GateSummary[] = [];
  async function fetchGates() {
    gates = await (await fetch("/api/gates")).json();
  }
</script>

{#each gates as gate}...{/each}
```

There's no `useState`, no `ref()`, no explicit subscription. A plain
`let` declaration at the top level of a component's `<script>` is
reactive by default, the Svelte compiler statically analyzes the
component and rewrites every place `gates` gets reassigned into a call
that also triggers the DOM update. This is the actual "compiler, not
library" claim made concrete: React needs `useState` as a runtime API
because it has no build step that understands your code's structure
deeply enough to do this automatically. Svelte's compiler does.

## `onMount`/`onDestroy` and the polling loop

```typescript
let pollHandle: ReturnType<typeof setInterval> | undefined;

onMount(() => {
  fetchGates();
  pollHandle = setInterval(fetchGates, 5000);
});

onDestroy(() => {
  if (pollHandle) clearInterval(pollHandle);
});
```

This is the dashboard's actual live-update mechanism, poll `/api/gates`
every 5 seconds, and it's a direct, honest trade-off worth naming: a
5-second poll is simpler to reason about and debug than a WebSocket or
Server-Sent Events connection, at the cost of up to 5 seconds of
staleness and a request that happens whether or not anything actually
changed. For a dashboard showing gate verdicts, that trade is fine,
nobody needs sub-second latency on a status page. `onDestroy` clearing
the interval is the detail that's easy to skip and causes a real bug
later: without it, navigating away from this component would leave the
polling loop running forever in the background, a small, real memory
and network leak.

## The one real bug this repo's build caught

The first version of this frontend failed to build with `The keyword
'interface' is reserved`, TypeScript syntax inside a `.svelte` file
needs an explicit preprocessor wired up (`svelte.config.js` with
`vitePreprocess()`), Vite's Svelte plugin doesn't infer TypeScript from
`lang="ts"` alone. Worth knowing before hitting it fresh: `lang="ts"`
on the `<script>` tag is necessary but not sufficient.

## Go deeper

- [svelte.dev/tutorial](https://svelte.dev/tutorial) is the official,
  interactive tutorial, genuinely good for actually building the muscle
  memory rather than just reading about it.
- [svelte.dev/docs/svelte/overview](https://svelte.dev/docs/svelte/overview)
  is the reference documentation for when a specific API's exact
  behavior needs checking.

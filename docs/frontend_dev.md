# Frontend Developer Guide

This guide documents the frontend as it exists today, not as an early demo UI.

## Stack

- React 18
- Vite 5
- React Router
- Redux Toolkit + React Redux
- MUI v5 + Emotion
- Axios for REST
- EventSource for SSE
- Plain browser WebSocket for realtime channels
- Vitest + Testing Library
- Playwright for E2E smoke tests

## Core runtime model

The frontend is a single SPA served by the Go backend.

- Axios base URL: `/api`
- Vite dev proxy forwards `/api` and `/ws` to `http://localhost:8080`
- Long-running jobs use `GET /api/progress/:taskID`

Relevant files:

- `frontend/src/utils/api.js`
- `frontend/vite.config.js`
- `frontend/src/main.jsx`
- `frontend/src/App.jsx`

## Route map

Current app routes:

- `/login`
- `/`
- `/settings`
- `/scenes/:id`
- `/scenes/:id/story`
- `/scenes/:id/characters`
- `/scenes/:id/export`
- `/scenes/:id/comic`
- `/scenes/:id/comic/video`
- `/scripts/new`
- `/scripts`
- `/scripts/:id`
- `/scripts/:id/story`

All major workspace pages sit under `frontend/src/pages`.

## Key pages and responsibilities

### Home / scenes

- `src/pages/Home.jsx`
- scene listing and creation
- standalone comic shell entry

### Settings

- `src/pages/Settings.jsx`
- manages LLM, Vision, and Video settings in one page
- loads models from `GET /api/llm/models?provider=<provider>`
- saves the combined settings payload through `POST /api/settings`

Important current behavior:

- `test-connection` validates LLM only
- base URL placeholder changes for some providers, including NVIDIA
- do not assume every UI dropdown option is officially backend-supported unless it is also registered server-side

### Comics Studio

- `src/pages/ComicGenerator.jsx`
- main 5-step workflow
- consumes overview, analysis, prompts, key elements, references, generation state, and exports

Supporting modules include:

- `src/components/comic/*`
- `src/store/comicSlice.js`
- `src/hooks/useSSEProgress.js`

### Video Studio

- `src/pages/ComicVideoStudio.jsx`
- manages timeline build, generation, clip regeneration, recovery state, and exports

Supporting modules include:

- `src/api/comicVideo.js`
- `src/store/comicVideoSlice.js`
- `src/hooks/useVideoSSEProgress.js`

### Scripts

- `src/pages/Scripts.jsx`
- `src/pages/ScriptsNew.jsx`
- `src/pages/ScriptDetail.jsx`
- `src/pages/ScriptMode.jsx`

Scripts currently mix page-local state and API-layer orchestration more than the comics/video flows.

## State management

Current store modules:

- `authSlice.js`
- `sceneSlice.js`
- `storySlice.js`
- `characterSlice.js`
- `itemSlice.js`
- `skillSlice.js`
- `comicSlice.js`
- `comicVideoSlice.js`

General rule:

- reusable cross-page or long-task state belongs in Redux
- short-lived form state can stay local to the page/component

## Networking conventions

### REST

Keep endpoint calls in `src/api/*.js` thin wrappers around Axios.

Examples:

- `src/api/settings.js`
- `src/api/scenes.js`
- `src/api/comic.js`
- `src/api/comicVideo.js`
- `src/api/scripts.js`

### SSE

Use EventSource-based hooks for long-running jobs.

Patterns already used:

- subscribe with `task_id`
- update page/slice state on `progress`
- close when status becomes `completed` or `failed`

### WebSocket

Backend uses plain Gorilla WebSocket, not Socket.IO.

The frontend should treat `/ws/scene/:id` and `/ws/user/status` as raw WebSocket channels.

## Local development workflow

### Install

```bash
cd frontend
npm install
```

### Start frontend dev server

```bash
npm run dev
```

### Start backend in another terminal

```bash
go run ./cmd/server
```

### Main validation loop

```bash
npm test
npm run lint
npm run build
```

## Test strategy

### Unit / component tests

Run:

```bash
npm test
```

Important coverage areas:

- `src/pages/__tests__/Settings.test.jsx`
- `src/pages/__tests__/ComicGenerator.test.jsx`
- `src/components/comic/__tests__/*`
- `src/store/__tests__/*`

### E2E smoke tests

Run:

```bash
npm run test:e2e
```

Current E2E emphasis is on mocked route-level verification of key workspace behavior rather than full backend integration.

### Build verification

Run:

```bash
npm run build
```

This is mandatory before merging route, layout, or bundle-affecting changes.

## Current frontend architecture priorities

1. **Settings as the configuration hub**
   - one screen manages LLM, Vision, Video
   - `GET /api/settings` is the main source of truth for available models and provider defaults

2. **Comics and Video rely on backend contracts**
   - do not recreate business rules in the client if the backend already returns overview/recovery state
   - prefer reading `recovery.status` over guessing from partial fields

3. **Keep provider support documentation aligned with backend registration**
   - document only what is actually registered server-side
   - if UI exposes experimental options, label or treat them carefully

4. **SSE-driven UX must converge cleanly**
   - terminal states must always close subscriptions
   - avoid overview refresh storms on every progress event

## File organization guidance

- `src/pages` — workspace-level pages
- `src/components` — reusable feature UI
- `src/api` — HTTP client wrappers
- `src/store` — shared state and async thunks
- `src/hooks` — reusable UI/runtime hooks
- `src/contexts` — theme/language providers
- `src/utils` — generic utilities
- `src/i18n` — translation dictionary

## Contributor checklist

Before submitting frontend changes:

1. update or add tests where behavior changed
2. run `npm test`
3. run `npm run lint`
4. run `npm run build`
5. verify that any provider-related UI change still matches backend-supported behavior
6. if contracts changed, update `README*`, `docs/api*`, or `docs/deployment*` in the same pass

## Practical reminder

This frontend is now part of a larger platform with scenes, comics, video, and scripts. Changes should be evaluated against that full product shape, not against the older “simple scene demo” assumption.

<!--
## Troubleshooting Cheatsheet

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| 404s on API calls | Backend not running or wrong `VITE_API_BASE_URL` | Start Go server, confirm env file |
| WebSocket errors | Backend not reachable or proxy mismatch | Confirm Go backend port and Vite proxy for `/ws` |
| Styles missing | Forgot to import `src/index.css` or theme assets in component | Ensure root entry imports theme | 
| ESLint fails on hooks | Hook dependency array incomplete | Let ESLint auto-fix or refactor hook |

## Conventions & Tips

- **Component folders**: Each major domain (characters, story, scenes, items) has its own directory—add new UI there for easy discovery.
- **State slices**: When adding new data flows, extend `src/store/*Slice.js` and wire them through `store/index.js`.
- **Translations**: Use `useTranslation` and update `src/i18n/translations.js` for new strings so bilingual UI stays in sync.
## WebSockets (important)

The backend exposes **plain WebSocket** endpoints (Gorilla WebSocket), e.g.:

- `ws://localhost:8080/ws/scene/<sceneId>?user_id=<userId>`

This is **not** Socket.IO. A Socket.IO client will generally not be able to talk to a plain WebSocket server.

If you want live updates in the frontend, prefer using the native WebSocket API:

```js
const ws = new WebSocket(`ws://${location.host}/ws/scene/${sceneId}?user_id=${userId}`);

ws.onmessage = (evt) => {
	const msg = JSON.parse(evt.data);
	switch (msg.type) {
		case 'conversation:new':
			// handle
			break;
		case 'heartbeat':
		case 'pong':
			break;
		default:
			break;
	}
};

ws.send(JSON.stringify({ type: 'ping' }));
```

Note: some existing frontend code currently uses `socket.io-client` (see `frontend/src/hooks/useWebSocket.js`).
If you keep the backend as-is, that hook should be migrated to native WebSocket.
-->

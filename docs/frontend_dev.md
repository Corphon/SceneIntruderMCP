# Frontend Developer Guide

This document walks through the practical steps for working on the SceneIntruderMCP frontend (Vite + React). As of v2.0.0, the frontend spans Home/scenes, comics v2, Settings (LLM + Vision), and the New Script workspace. This guide covers prerequisites, project setup, live development, quality gates, and production build expectations so every contributor follows the same workflow.

## Tech Stack Snapshot

- **Framework**: React 18 with functional components and hooks
- **Bundler/Dev Server**: Vite 5
- **State Management**: Redux Toolkit + React Redux slices under `src/store`
- **Styling/UI**: MUI v5, Emotion, and custom CSS modules
- **Realtime/Networking**: Axios for REST calls (`src/api`), **SSE (EventSource)** for long-task progress (`GET /api/progress/:task_id`), and plain WebSocket for optional live updates (backend uses Gorilla WebSocket)

## Prerequisites

| Tool | Recommended Version | Notes |
|------|---------------------|-------|
| Node.js | 18 LTS or 20 LTS | Aligns with Vite 5 support matrix |
| npm | 9+ (ships with Node 18) | Yarn/pnpm work too, but npm scripts are canonical |
| Git | Optional but recommended | Keeps lockfile and dependencies in sync |

Verify local versions:

```bash
node -v
npm -v
```

## Project Setup

1. Navigate into the frontend workspace:
	```bash
	cd frontend
	```
2. Install dependencies (first run or whenever `package.json` changes):
	```bash
	npm install
	```
	This installs both runtime deps (React, MUI, Axios) and dev tooling (Vite, ESLint, Vitest, Playwright).

> 💡 Tip: Keep `package-lock.json` under version control so everyone resolves packages identically.

## Environment Configuration

By default, the frontend uses a relative REST base URL:

- Axios base URL: `/api` (see `frontend/src/utils/api.js`)

During development, Vite proxies `/api` and `/ws` to the Go backend (see `frontend/vite.config.js`).

If you need to point to a different backend host/port, update the Vite proxy target.

## Daily Development Workflow

1. **Launch the dev server** with hot-module replacement:
	```bash
	npm run dev
	```
	Vite prints a local URL (usually `http://localhost:5173`). API requests proxy directly to whatever base URL the env var points to.

2. **Start the Go backend** (in another terminal) so REST and WebSocket calls succeed:
	```bash
	go run ./cmd/server
	```

3. **Live editing**:
	- Components live under `src/components/*` grouped by feature (characters, scenes, items, story widgets).
	- Pages route through React Router (see `src/pages` and `src/main.jsx`).
	- Shared logic sits in `src/hooks`, `src/store`, and `src/utils`.

4. **API integrations**: Use the thin Axios clients under `src/api/*.js`. Keep endpoints centralized there to avoid scattering URLs.


### New Script integration (v2.0.0) 🔧

- Endpoints you will use:
  - `POST /api/scripts` — create a new script project (returns project metadata)
  - `POST /api/scripts/:id/generate` — start asynchronous generation (returns `{ "task_id": "..." }`)
  - `POST /api/scripts/:id/command` — execute assist commands (e.g., `assist_mode`: `inspiration`, `completion`, `polish`)
  - `GET /api/progress/:task_id` — subscribe to SSE progress updates (see below)

- Frontend files to edit / reference:
  - API client: `frontend/src/api/scripts.js`
  - Pages: `frontend/src/pages/Scripts.jsx`, `frontend/src/pages/ScriptDetail.jsx`
  - Components: `frontend/src/components/scripts/ScriptCreator.jsx`, `frontend/src/components/scripts/ScriptCard.jsx`

- Local dev & testing notes:
  - Workflow: start backend → `POST /api/scripts` to create a project → `POST /api/scripts/:id/generate` to begin generation → subscribe to `/api/progress/:task_id` (SSE) to receive progress events.
  - Simple SSE subscription example (browser):

```js
const es = new EventSource(`/api/progress/${taskId}`);
es.onmessage = (e) => { console.log('progress:', e.data); };
es.addEventListener('progress', (e) => { console.log('progress event:', e.data); });
es.addEventListener('connected', () => { console.log('connected'); });
```

  - Tests: backend endpoint tests live in `internal/api/scripts_endpoints_test.go` (covers create/list/get/generate/export/rewind).

- Important note: **`POST /api/scripts` only creates the project and initializes files; it does not automatically trigger generation.** You must call `POST /api/scripts/:id/generate` to start generation.


5. **API integrations**: Use the thin Axios clients under `src/api/*.js`. Keep endpoints centralized there to avoid scattering URLs.

## Quality Gates

### Linting

```bash
npm run lint
```

- Enforces React hook rules, unused disable directives, and zero warnings.
- Run before every PR to catch regressions early.

### Optional Type Safety

The project currently relies on plain JS. If you need stronger contracts for a feature, consider adding JSDoc typings or incrementally introducing TypeScript via Vite’s TS template.

## Testing

### Unit / component tests (Vitest)

```bash
npm test
```

- Uses Vitest + Testing Library.
- Common locations:
	- `frontend/src/pages/__tests__/*`
	- `frontend/src/components/**/__tests__/*`

### E2E smoke tests (Playwright)

```bash
npm run test:e2e
```

- The comics wizard has a minimal E2E smoke suite with route-level mocking (does not require the backend).

### Demo recordings (Playwright video/trace)

To generate reproducible demo artifacts (video + trace + screenshots) from the comics wizard E2E (still mocked, no backend required):

```bash
npm --prefix frontend run test:e2e:demo
```

Outputs are written under:

- `frontend/test-results/` (videos, traces, screenshots)

Notes:

- This uses `frontend/playwright.demo.config.js` to force `video: 'on'` and `trace: 'on'` even when tests pass.
- If you want a GIF, you can convert the recorded video using your preferred tool (e.g. `ffmpeg`) outside the repo; we do not commit large binary media by default.

## Comics (v2) developer notes

### Entry

- Route: `/scenes/:sceneId/comic`
- Page: `frontend/src/pages/ComicGenerator.jsx`
- Components: `frontend/src/components/comic/*`
- Standalone entry: Home page → `frontend/src/components/comic/ComicCreatorDialog.jsx` → `POST /api/scenes/shell` → `/scenes/:id/comic?entry=comic_standalone`

### Standalone New Comic flow

- API client: `frontend/src/api/scenes.js` → `createSceneShell()`
- Home entry: `frontend/src/pages/Home.jsx`
- Standalone query marker: `entry=comic_standalone`
- Redux support: `frontend/src/store/comicSlice.js` stores `standaloneSourceText`
- Analysis payload: Step1 can send `source_text` instead of `node_id`

This is the main v2.0.0 change for comics UX. It allows users to create a comic workspace first and only then paste/write story text directly into Step1.

### Core endpoints (backend)

- `POST /api/scenes/:sceneId/comic/analysis` (202 + `{ task_id }`)
- `POST /api/scenes/:sceneId/comic/prompts` (202 + `{ task_id }`)
- `POST /api/scenes/:sceneId/comic/key_elements` (202 + `{ task_id }`)
- `POST /api/scenes/:sceneId/comic/generate` (202 + `{ task_id }`)
- `GET  /api/progress/:task_id` (SSE)
- `POST /api/cancel/:task_id` (cancel)
- `GET  /api/scenes/:sceneId/comic` (overview)
- `GET  /api/scenes/:sceneId/comic/images/:frameId` (PNG direct output)
- `GET  /api/scenes/:sceneId/comic/export?format=zip|html` (download)

### Vision model list (frontend)

- The comics Step4 model dropdown is driven by `GET /api/settings`.
- The backend returns `vision_models` as an array of:
	- `key` (stable model key written into prompts as `model`)
	- `label` (human readable label)
	- `provider` (routing hint; backend still decides the actual provider mapping)
	- `supports_reference_image` (whether the model supports img2img/reference images)
- Frontend behavior:
	- `frontend/src/pages/ComicGenerator.jsx` best-effort fetches settings and passes `vision_models` into Step4 UI.
	- If `vision_models` is missing/empty or settings fetch fails, the UI falls back to a small built-in default list for compatibility.

Notes:

- The Settings page (`frontend/src/pages/Settings.jsx`) now manages both **LLM** and **Vision** settings.
- Vision settings currently expose the common fields needed for operational setup:
	- `vision_provider`
	- `vision_default_model`
	- `vision_config.endpoint`
	- `vision_config.api_key`
- The Vision provider dropdown includes `placeholder`, `sdwebui`, `dashscope`, `gemini`, `ark`, `openai`, and `glm`.
- For `glm`, the recommended endpoint is `https://open.bigmodel.cn/api/paas/v4` and the recommended default model is `glm-image`.
- Switching the Vision provider now auto-fills the recommended `vision_default_model` and `vision_config.endpoint`.
- The Vision section also exposes an explicit “apply recommended defaults” action for the current provider.
- `POST /api/settings/test-connection` still validates **LLM** connectivity only; Vision configuration should be verified through actual image generation/regeneration.

### Recommended frontend regression coverage for v2.0.0

- Settings page: `frontend/src/pages/__tests__/Settings.test.jsx`
- Comics page: `frontend/src/pages/__tests__/ComicGenerator.test.jsx`
- Comics UI cards: `frontend/src/components/comic/__tests__/*`
- Redux slice: `frontend/src/store/__tests__/comicSlice.test.js`
- E2E: `frontend/tests/e2e/comic-wizard.spec.ts`, `frontend/tests/e2e/comic-home-entry.spec.ts`

### SSE progress subscription

Frontend uses a small hook wrapper around EventSource:

- Hook: `frontend/src/hooks/useSSEProgress.js`
- Expected event: `event: progress` with JSON payload containing at least `{ status, message, progress }`.

### Backend-only E2E script (PowerShell)

When you want to validate the backend chain (analysis -> prompts -> key_elements -> generate -> export) without the frontend UI, run:

```powershell
pwsh -File .\docs\comics_e2e.ps1 -SceneId scene_123
```

This script polls GET endpoints until results are ready and downloads export artifacts into the `docs/` folder.

## Building for Production

```bash
npm run build
```

- Produces a production bundle under `frontend/dist` (Vite default).
- The Go backend serves static assets from `frontend/dist` when running in production mode.

To preview the production build locally:

```bash
npm run preview
```

This spins up a static server mimicking how assets will load once deployed.

## Recommended Workflow Checklist

1. `git pull` / sync main branch
2. `npm install` (only if lockfile changed)
3. Create/update `.env.local`
4. `npm run dev` + `go run ./cmd/server`
5. Implement feature/bugfix
6. `npm run lint`
7. `npm run build` (optional sanity check)
8. Commit with updated docs/tests as needed

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

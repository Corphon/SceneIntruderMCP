# Frontend Developer Guide

This document walks through the practical steps for working on the SceneIntruderMCP frontend (Vite + React). It covers prerequisites, project setup, live development, quality gates, and production build expectations so every contributor follows the same workflow.

## Tech Stack Snapshot

- **Framework**: React 18 with functional components and hooks
- **Bundler/Dev Server**: Vite 5
- **State Management**: Redux Toolkit + React Redux slices under `src/store`
- **Styling/UI**: MUI v5, Emotion, and custom CSS modules
- **Realtime/Networking**: Axios for REST calls (`src/api`), Socket.IO client for live updates (`hooks/useWebSocket`)

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
	This installs both runtime deps (React, MUI, Axios, Socket.IO) and dev tooling (Vite, ESLint, React plugin).

> 💡 Tip: Keep `package-lock.json` under version control so everyone resolves packages identically.

## Environment Configuration

The frontend reads `VITE_*` variables from `.env` files at the project root (e.g., `.env.local`). Common knobs:

```bash
VITE_API_BASE_URL=http://localhost:8080/api
VITE_WS_BASE_URL=ws://localhost:8080/ws
VITE_DEFAULT_LOCALE=en
```

- Anything prefixed with `VITE_` becomes available via `import.meta.env.VITE_*`.
- Commit default-safe values to `.env.example` and keep `.env.local` ignored for personal secrets.

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

## Quality Gates

### Linting

```bash
npm run lint
```

- Enforces React hook rules, unused disable directives, and zero warnings.
- Run before every PR to catch regressions early.

### Optional Type Safety

The project currently relies on plain JS. If you need stronger contracts for a feature, consider adding JSDoc typings or incrementally introducing TypeScript via Vite’s TS template.

## Testing (Planned)

No automated UI tests are wired in yet. If you add them, prefer Vitest + Testing Library and expose them through `npm run test` for consistency.

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
| WebSocket errors | Wrong WS URL or backend port mismatch | Update `VITE_WS_BASE_URL` to actual host:port |
| Styles missing | Forgot to import `src/index.css` or theme assets in component | Ensure root entry imports theme | 
| ESLint fails on hooks | Hook dependency array incomplete | Let ESLint auto-fix or refactor hook |

## Conventions & Tips

- **Component folders**: Each major domain (characters, story, scenes, items) has its own directory—add new UI there for easy discovery.
- **State slices**: When adding new data flows, extend `src/store/*Slice.js` and wire them through `store/index.js`.
- **Translations**: Use `useTranslation` and update `src/i18n/translations.js` for new strings so bilingual UI stays in sync.
- **WebSockets**: Reuse `hooks/useWebSocket.js` rather than instantiating clients manually; it already handles reconnection.

# cloud-dashboard

React + Vite + TypeScript dashboard for the synced transactions. In production CloudFront serves the
build from a private S3 bucket and it reads `cloud-api` through API Gateway.

## Run locally

```powershell
npm install
npm run dev        # http://localhost:5173, hot reload
npm run build      # type-check + production build into dist/
npm run preview    # serve dist/ exactly as it would be deployed
npm run lint
```

`?month=YYYY-MM` in the URL opens a given month.

## Data

| `VITE_API_URL` | Data source |
|---|---|
| unset (default) | `src/api/mock.ts`: generated, deterministic demo data. The header shows **Demo data**. |
| e.g. `http://localhost:8080` | `cloud-api`: `GET /v1/summary?month=`, `GET`/`POST /v1/transactions`, `PUT`/`DELETE /v1/transactions/{id}`, `GET`/`POST /v1/recurring`, `PUT`/`DELETE /v1/recurring/{id}`, `GET /v1/net-worth`, `GET`/`PUT /v1/categories` |

Put it in `.env.local` (git-ignored) to point at a real API. In demo mode, manual entries, recurring rules
and edited categories are kept in this browser's localStorage only, and due recurring entries are
created whenever data is read (the real backend does it in a daily job). The response shapes are in
`src/api/types.ts`; `Transaction` mirrors `backend/internal/transaction`.

## Layout

| Path | Contents |
|---|---|
| `src/App.tsx` | Month state (synced to the URL), data loading, page layout |
| `src/api/` | Types, HTTP client, mock |
| `src/recurrence.ts` | Recurring-rule calendar math (month-end clamping, next date, descriptions) |
| `src/entry.ts` | Shared state and checks for the entry and recurring forms |
| `src/categories.ts` | Default categories, the built-in Other/Income ids, palette slots |
| `src/components/` | Net worth, stat tiles, stacked monthly chart, donut, entry form, recurring panel, category editor, transactions table |
| `src/format.ts` | Month helpers and money formatting (amounts are integer minor units) |
| `src/index.css` | Design tokens, light and dark |

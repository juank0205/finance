# cloud-dashboard

React + Vite + TypeScript dashboard for the synced transactions. In production CloudFront serves the
build from a private S3 bucket. It signs in with Cognito and reads `cloud-api` through its Lambda
Function URL. Until CloudFront is enabled, it runs locally against the API in AWS (see below).

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
| `/` | The page's own origin. `npm run dev` forwards `/v1` to a local `cloud-api` on port 8080 (see below). |
| e.g. `http://localhost:8080` | `cloud-api` at that URL (needs CORS, which `cloud-api` does not send): `GET /v1/summary?month=`, `GET`/`POST /v1/transactions`, `PUT`/`DELETE /v1/transactions/{id}`, `GET`/`POST /v1/recurring`, `PUT`/`DELETE /v1/recurring/{id}`, `GET /v1/net-worth`, `GET`/`PUT /v1/categories` |

Put it in `.env.local` (git-ignored) to point at a real API. In demo mode, manual entries, recurring rules
and edited categories are kept in this browser's localStorage only, and due recurring entries are
created whenever data is read (the real backend does it on the first request of each day). The
response shapes are in `src/api/types.ts`; `Transaction` mirrors `backend/internal/transaction`.

### Against a local cloud-api

```powershell
# terminal 1: the API, data kept in memory until it stops
cd backend
go run ./cmd/cloud-api

# or, with the real storage code: DynamoDB Local (data kept while the container runs)
docker run --rm -d --name finance-ddb -p 8000:8000 amazon/dynamodb-local
$env:FINANCE_TABLE = "finance-ledger"; $env:FINANCE_DYNAMODB_ENDPOINT = "http://localhost:8000"
go run ./cmd/cloud-api      # creates the table on first run

# terminal 2: the dashboard
cd frontend/cloud-dashboard
Set-Content .env.local "VITE_API_URL=/"
npm run dev        # http://localhost:5173, header no longer says Demo data
```

Delete `.env.local` to go back to demo data.

### Against the API in AWS

Until AWS enables CloudFront for the account, the dashboard runs only on this machine while the data,
sign-in and API live in AWS ([docs/deploy-api.md](../../docs/deploy-api.md) sets them up). Put the
values of `terraform output dashboard_env` (in `infra/terraform/envs/prod`) in `.env.local`:

```
VITE_API_URL=https://<id>.lambda-url.eu-west-3.on.aws/
VITE_COGNITO_DOMAIN=https://finance-<hash>.auth.eu-west-3.amazoncognito.com
VITE_COGNITO_CLIENT_ID=<client id>
```

Then `npm run dev` (port 5173) or `npm run build && npm run preview` (port 4173): only these two
origins may sign in and call the API. The page sends you to the Cognito sign-in page first and shows
**Sign out** in the header. Tokens are kept in memory only, so a reload passes through the sign-in page
again, which returns straight away while its own session lasts (one hour).

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

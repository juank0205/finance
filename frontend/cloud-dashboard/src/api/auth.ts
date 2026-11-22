// Cognito sign-in: authorization code flow with PKCE against the hosted sign-in page.
//
// Tokens stay in memory only, never in localStorage, so a script injected into the page later
// cannot find a stored one. A reload therefore bounces through the hosted page, which still has its
// own session and comes straight back without asking again.

const domain = import.meta.env.VITE_COGNITO_DOMAIN?.replace(/\/$/, '')
const clientId = import.meta.env.VITE_COGNITO_CLIENT_ID

/** Sign-in is on when both Cognito settings are present; local runs without them skip it. */
export const authEnabled = Boolean(domain && clientId)

/** sessionStorage key for the one sign-in in flight: PKCE verifier, state and where to return. */
const PENDING = 'finance.signin'

interface Pending {
  state: string
  verifier: string
  returnTo: string
}

interface Tokens {
  access: string
  refresh?: string
  /** ms since epoch */
  expiresAt: number
}

let tokens: Tokens | null = null
let refreshing: Promise<void> | null = null

const redirectUri = () => `${window.location.origin}/`

function randomString(bytes: number): string {
  return base64url(crypto.getRandomValues(new Uint8Array(bytes)))
}

function base64url(data: Uint8Array): string {
  return btoa(String.fromCharCode(...data))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

/** Sends the browser to the hosted sign-in page. The returned promise never settles. */
export async function signIn(): Promise<never> {
  const pending: Pending = {
    state: randomString(16),
    verifier: randomString(32),
    returnTo: window.location.pathname + window.location.search,
  }
  sessionStorage.setItem(PENDING, JSON.stringify(pending))
  const challenge = base64url(
    new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(pending.verifier))),
  )
  const url = new URL(`${domain}/oauth2/authorize`)
  url.search = new URLSearchParams({
    response_type: 'code',
    client_id: clientId!,
    redirect_uri: redirectUri(),
    scope: 'openid email',
    state: pending.state,
    code_challenge: challenge,
    code_challenge_method: 'S256',
  }).toString()
  window.location.assign(url)
  return new Promise<never>(() => {})
}

async function tokenRequest(params: Record<string, string>): Promise<void> {
  const res = await fetch(`${domain}/oauth2/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ client_id: clientId!, ...params }),
  })
  if (!res.ok) throw new Error(`sign-in failed: HTTP ${res.status}`)
  const body = (await res.json()) as { access_token: string; refresh_token?: string; expires_in: number }
  tokens = {
    access: body.access_token,
    // A refresh grant returns no new refresh token; keep the one we have.
    refresh: body.refresh_token ?? tokens?.refresh,
    expiresAt: Date.now() + body.expires_in * 1000,
  }
}

/**
 * Runs once at startup. Finishes a sign-in when the page is the hosted page's redirect back,
 * otherwise starts one. Resolves when there is an access token.
 */
async function start(): Promise<void> {
  const params = new URLSearchParams(window.location.search)
  const code = params.get('code')
  const raw = sessionStorage.getItem(PENDING)
  sessionStorage.removeItem(PENDING)
  if (params.has('error')) {
    throw new Error(`sign-in failed: ${params.get('error_description') ?? params.get('error')}`)
  }
  if (!code) return signIn()

  const pending = raw ? (JSON.parse(raw) as Pending) : null
  // The state must be the one this tab sent, or the code may have been planted by another site.
  if (!pending || pending.state !== params.get('state')) return signIn()
  await tokenRequest({
    grant_type: 'authorization_code',
    code,
    redirect_uri: redirectUri(),
    code_verifier: pending.verifier,
  })
  // Drop ?code=&state= from the address bar and history, back to where the user was (e.g. ?month=).
  window.history.replaceState(null, '', pending.returnTo)
}

/** Resolves once signed in (immediately when sign-in is off). Render nothing before it. */
export const ready: Promise<void> = authEnabled ? start() : Promise.resolve()

/** A current access token, refreshed when it is about to expire. */
export async function accessToken(): Promise<string> {
  await ready
  if (tokens && tokens.expiresAt - Date.now() < 2 * 60_000 && tokens.refresh) {
    refreshing ??= tokenRequest({ grant_type: 'refresh_token', refresh_token: tokens.refresh })
      .catch(() => signIn())
      .finally(() => (refreshing = null))
    await refreshing
  }
  if (!tokens) return signIn()
  return tokens.access
}

/** Revokes the refresh token and ends the hosted page's session too. */
export async function signOut(): Promise<void> {
  const refresh = tokens?.refresh
  tokens = null
  if (refresh) {
    await fetch(`${domain}/oauth2/revoke`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({ client_id: clientId!, token: refresh }),
    }).catch(() => undefined)
  }
  const url = new URL(`${domain}/logout`)
  url.search = new URLSearchParams({ client_id: clientId!, logout_uri: redirectUri() }).toString()
  window.location.assign(url)
}

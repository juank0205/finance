import { useEffect, useRef, useState } from 'react'
import type { Account, AccountKind } from '../api/types.ts'
import { amountInput, parseAmount } from '../format.ts'
import { toDay } from '../recurrence.ts'

interface Props {
  initial: Account[]
  /** The summary currency: the default for a new account. */
  currency: string
  today: Date
  onSave: (accounts: Account[]) => Promise<void>
  onClose: () => void
}

/** An account as edited: the opening balance as typed. */
interface Draft extends Omit<Account, 'opening_minor'> {
  opening: string
}

/** cloud-api keeps the net worth card readable. */
const MAX_ACCOUNTS = 12

const KIND_LABEL: Record<AccountKind, string> = {
  cash: 'Cash',
  credit: 'Credit card',
}

function newId(): string {
  return `a-${crypto.randomUUID().slice(0, 8)}`
}

function toDraft(a: Account): Draft {
  const { opening_minor, ...rest } = a
  return { ...rest, opening: opening_minor ? `${opening_minor < 0 ? '-' : ''}${amountInput(opening_minor, a.currency)}` : '' }
}

/** A signed amount: "1250.50", "-300", or blank for 0. Returns minor units, or null if it isn't one. */
function parseOpening(text: string, currency: string): number | null {
  const t = text.trim()
  const negative = /^[-−]/.test(t) // hyphen or minus sign
  const rest = negative ? t.slice(1).trim() : t
  if (!rest) return negative ? null : 0
  if (/^0+([.,]0*)?$/.test(rest)) return 0
  const minor = parseAmount(rest, currency)
  return minor === null ? null : negative ? -minor : minor
}

/** Mirrors the server-side checks so most mistakes are caught before saving. */
function problem(draft: Draft[]): string | null {
  const seen = new Set<string>()
  for (const a of draft) {
    const name = a.name.trim()
    if (!name) return 'Every account needs a name.'
    if (seen.has(name.toLowerCase())) return `"${name}" appears twice.`
    seen.add(name.toLowerCase())
    if (!/^[A-Z]{3}$/.test(a.currency)) return `${name}: use a 3-letter currency code, like EUR.`
    if (parseOpening(a.opening, a.currency) === null) return `${name}: enter the opening balance like 1250.00 or -300.`
    if (!a.opening_date) return `${name}: pick the opening date.`
  }
  return null
}

export function AccountSettings({ initial, currency, today, onSave, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [draft, setDraft] = useState<Draft[]>(() => initial.map(toDraft))
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  // A saved account's currency is fixed: its entries keep amounts in it.
  const saved = new Set(initial.map((a) => a.id))

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  // Any edit may fix the last problem, so clear its message.
  const edit = (fn: (list: Draft[]) => Draft[]) => {
    setError(null)
    setDraft(fn)
  }

  const update = (id: string, patch: Partial<Draft>) =>
    edit((list) => list.map((a) => (a.id === id ? { ...a, ...patch } : a)))

  const add = () =>
    edit((list) => [
      ...list,
      { id: newId(), name: '', kind: 'cash', currency, opening: '', opening_date: toDay(today) },
    ])

  const save = async () => {
    const p = problem(draft)
    if (p) {
      setError(p)
      return
    }
    setSaving(true)
    setError(null)
    try {
      await onSave(
        draft.map(({ opening, ...a }) => ({
          ...a,
          name: a.name.trim(),
          opening_minor: parseOpening(opening, a.currency) ?? 0,
        })),
      )
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setSaving(false)
    }
  }

  return (
    <dialog ref={dialog} className="settings wide" onClose={onClose} aria-labelledby="accounts-title">
      <h2 id="accounts-title">Accounts</h2>
      <p className="card-sub">
        A balance is its opening balance plus every entry tied to the account from the opening date on. Up to{' '}
        {MAX_ACCOUNTS}. Entries of a deleted account stay, but no longer count toward any balance.
      </p>

      {draft.length === 0 && <p className="empty acct-none">No accounts yet.</p>}

      <ol className="cat-edit">
        {draft.map((a, i) => {
          const label = a.name || 'account'
          const fixed = saved.has(a.id)
          const hintId = `acct-hint-${a.id}`
          return (
            <li key={a.id}>
              <div className="acct-edit-row">
                <input
                  value={a.name}
                  maxLength={30}
                  placeholder="Name"
                  aria-label={`Account ${i + 1} name`}
                  onChange={(e) => update(a.id, { name: e.target.value })}
                />
                <select
                  value={a.kind}
                  aria-label={`Type of ${label}`}
                  onChange={(e) => update(a.id, { kind: e.target.value as AccountKind })}
                >
                  {(Object.keys(KIND_LABEL) as AccountKind[]).map((k) => (
                    <option key={k} value={k}>
                      {KIND_LABEL[k]}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  className="ghost icon"
                  onClick={() => edit((l) => l.filter((o) => o.id !== a.id))}
                  aria-label={`Delete ${label}`}
                >
                  ✕
                </button>
              </div>
              <div className="acct-fields">
                <label>
                  <span>Currency</span>
                  <input
                    value={a.currency}
                    maxLength={3}
                    autoComplete="off"
                    spellCheck={false}
                    disabled={fixed}
                    title={fixed ? 'Fixed once saved; add a new account for another currency' : undefined}
                    onChange={(e) => update(a.id, { currency: e.target.value.toUpperCase().trim() })}
                  />
                </label>
                <label>
                  <span>Opening balance</span>
                  <input
                    value={a.opening}
                    autoComplete="off"
                    placeholder={a.kind === 'credit' ? '-0.00' : '0.00'}
                    aria-describedby={a.kind === 'credit' ? hintId : undefined}
                    onChange={(e) => update(a.id, { opening: e.target.value })}
                  />
                </label>
                <label className="acct-date">
                  <span>At the start of</span>
                  <input
                    type="date"
                    value={a.opening_date}
                    onChange={(e) => update(a.id, { opening_date: e.target.value })}
                    required
                  />
                </label>
              </div>
              {a.kind === 'credit' && (
                <p className="acct-hint" id={hintId}>
                  Negative while you owe: if you owe 850,000, enter -850000.
                </p>
              )}
            </li>
          )
        })}
      </ol>

      <button type="button" className="ghost" onClick={add} disabled={draft.length >= MAX_ACCOUNTS}>
        + Add account
      </button>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      <div className="settings-actions">
        <button type="button" className="ghost" onClick={onClose}>
          Cancel
        </button>
        <button type="button" className="primary" onClick={save} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
      </div>
    </dialog>
  )
}

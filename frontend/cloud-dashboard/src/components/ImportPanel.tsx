import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api } from '../api/client.ts'
import type { Account, ImportEntry, ImportFormat, ImportLine, ImportPreview, TransactionKind } from '../api/types.ts'
import { KINDS, KIND_LABEL, type CategoryIndex } from '../categories.ts'
import { dateLabel, formatMoney } from '../format.ts'
import { dayToInstant } from '../recurrence.ts'

interface Props {
  accounts: Account[]
  categories: CategoryIndex
  /** Lines were booked: the month data and balances must be re-fetched. Gets the text to show. */
  onImported: (summary: string) => void
  onClose: () => void
}

/** cloud-api's limit, before base64. */
const MAX_FILE = 2 << 20
/** Lines per request (cloud-api takes up to 200): smaller batches show progress and lose less on an error. */
const BATCH = 50

const FORMAT_LABEL: Record<ImportFormat, string> = {
  sg: 'Société Générale',
  revolut: 'Revolut',
  bancolombia: 'Bancolombia',
}

const ALL_KINDS: TransactionKind[] = [...KINDS, 'transfer']

/** A line as edited: what will be booked if it stays checked. */
interface Draft {
  checked: boolean
  kind: TransactionKind
  merchant: string
  category: string // "" = not picked; ignored for transfers
  other: string // Account.id; transfers only
}

/** The file as base64, without the "data:…;base64," prefix. */
function readBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result).replace(/^data:[^,]*,/, ''))
    reader.onerror = () => reject(reader.error ?? new Error('Could not read the file'))
    reader.readAsDataURL(file)
  })
}

// Days are calendar days; noon keeps every time zone on the same one.
const shortDay = (day: string) => dateLabel(dayToInstant(day))

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

function summaryText(added: number, already: number): string {
  const there = already ? `${already} ${already === 1 ? 'was' : 'were'} already there` : ''
  if (!added) return `Nothing new imported: ${there}.`
  return `Imported ${plural(added, 'entry', 'entries')}${there ? `, ${there}` : ''}.`
}

function toDraft(l: ImportLine, categories: CategoryIndex, others: Account[]): Draft {
  return {
    checked: l.status === 'new',
    kind: l.kind,
    merchant: l.merchant,
    // A suggested category deleted since can't be booked; the person picks another.
    category: l.kind !== 'transfer' && l.category && categories.ids[l.kind].has(l.category) ? l.category : '',
    other: others.some((a) => a.id === l.other_account) ? (l.other_account ?? '') : '',
  }
}

/** Mirrors the server's checks for the checked lines. Returns the first problem and its line. */
function problem(lines: ImportLine[], drafts: Record<string, Draft>): { key: string; text: string } | null {
  for (const l of lines) {
    const d = drafts[l.key]
    if (!d?.checked) continue
    const where = `${shortDay(l.day)}, ${l.description}`
    const fail = (text: string) => ({ key: l.key, text: `${where}: ${text}` })
    if (!d.merchant.trim()) return fail('fill in the description.')
    if (d.kind === 'transfer') {
      if (!d.other) return fail('pick the other account.')
    } else if (!d.category) {
      return fail('pick a category.')
    }
    if (d.kind === 'income' && l.amount_minor < 0) return fail('income must be money coming in. Use Expense or Transfer.')
    if (d.kind === 'investment' && l.amount_minor > 0) {
      return fail('an investment must be money going out. Use Money back, Income or Transfer.')
    }
  }
  return null
}

function toEntry(l: ImportLine, d: Draft): ImportEntry {
  // Only the fields the server knows: it rejects anything else.
  return {
    key: l.key,
    day: l.day,
    amount_minor: l.amount_minor,
    currency: l.currency,
    kind: d.kind,
    merchant: d.merchant.trim(),
    ...(d.kind === 'transfer' ? { other_account: d.other } : { category: d.category }),
  }
}

export function ImportPanel({ accounts, categories, onImported, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [account, setAccount] = useState(accounts[0]?.id ?? '')
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [drafts, setDrafts] = useState<Record<string, Draft>>({})
  // Lines booked by this dialog, across attempts: an error halfway keeps them marked.
  const [sent, setSent] = useState<ReadonlySet<string>>(() => new Set())
  const [error, setError] = useState<string | null>(null)
  const [bad, setBad] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null)
  // What was booked so far; reported once, when the dialog closes.
  const booked = useRef({ added: 0, already: 0, reported: false })

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  const acct = accounts.find((a) => a.id === (preview?.account ?? account))
  const others = accounts.filter((a) => a.id !== acct?.id)
  const lines = preview?.lines ?? []
  const importable = (l: ImportLine) => (l.status === 'new' || l.status === 'duplicate') && !sent.has(l.key)
  const chosen = lines.filter((l) => importable(l) && drafts[l.key]?.checked)

  const close = () => {
    const b = booked.current
    if (!b.reported && (b.added || b.already)) {
      b.reported = true
      onImported(summaryText(b.added, b.already))
    }
    onClose()
  }

  const update = (key: string, patch: Partial<Draft>) => {
    setError(null)
    setBad(null)
    setDrafts((all) => {
      const next = { ...all[key], ...patch }
      // Categories belong to one kind; keep the choice only if it still fits.
      if (patch.kind && next.kind !== 'transfer' && !categories.ids[next.kind].has(next.category)) next.category = ''
      return { ...all, [key]: next }
    })
  }

  const selectAll = (checked: boolean) => {
    setError(null)
    setBad(null)
    setDrafts((all) => {
      const next = { ...all }
      for (const l of lines) if (importable(l)) next[l.key] = { ...next[l.key], checked }
      return next
    })
  }

  const runPreview = async (e: FormEvent) => {
    e.preventDefault()
    if (!acct) return setError('Pick the account this statement is for.')
    if (!file) return setError('Choose the statement file.')
    if (file.size === 0) return setError('The file is empty.')
    if (file.size > MAX_FILE) return setError('The file is larger than 2 MB. Export a shorter period.')
    setBusy(true)
    setError(null)
    try {
      const p = await (await api).previewImport(acct.id, await readBase64(file))
      const otherAccounts = accounts.filter((a) => a.id !== p.account)
      setDrafts(Object.fromEntries(p.lines.map((l) => [l.key, toDraft(l, categories, otherAccounts)])))
      setPreview(p)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const runImport = async () => {
    if (!preview) return
    const p = problem(chosen, drafts)
    if (p) {
      setError(p.text)
      setBad(p.key)
      document.getElementById(`imp-${lines.findIndex((l) => l.key === p.key)}`)?.scrollIntoView({ block: 'nearest' })
      return
    }
    const todo = chosen
    setBusy(true)
    setError(null)
    setProgress({ done: 0, total: todo.length })
    const b = booked.current
    const before = b.added + b.already
    try {
      const a = await api
      // One batch at a time, so a failure says exactly what got in.
      for (let i = 0; i < todo.length; i += BATCH) {
        const batch = todo.slice(i, i + BATCH)
        const res = await a.importLines(preview.account, batch.map((l) => toEntry(l, drafts[l.key])))
        b.added += res.added
        b.already += res.already_there
        setSent((s) => new Set([...s, ...batch.map((l) => l.key)]))
        setProgress({ done: i + batch.length, total: todo.length })
      }
      close()
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      const got = b.added + b.already - before
      setError(
        got
          ? `${message} ${plural(got, 'line was', 'lines were')} imported before this and are marked below; fix the problem and import the rest.`
          : message,
      )
      setProgress(null)
      setBusy(false)
    }
  }

  const back = () => {
    setPreview(null)
    setDrafts({})
    setError(null)
    setBad(null)
  }

  const count = (s: ImportLine['status']) => lines.filter((l) => l.status === s).length
  const anyImportable = lines.some(importable)

  return (
    <dialog
      ref={dialog}
      className="settings import"
      // Closing halfway would leave the last batches unreported.
      onCancel={(e) => busy && e.preventDefault()}
      onClose={close}
      aria-labelledby="imp-title"
    >
      <h2 id="imp-title">Import statement</h2>

      {!preview ? (
        <form onSubmit={runPreview} noValidate>
          <p className="card-sub">
            A Société Générale or Revolut CSV, or a Bancolombia XLSX, up to 2 MB. You review every line before
            anything is added; the file itself is not kept.
          </p>
          <div className="form-grid imp-pick">
            <label className="span-2">
              <span>Account</span>
              <select
                value={account}
                onChange={(e) => {
                  setError(null)
                  setAccount(e.target.value)
                }}
              >
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} ({a.currency})
                  </option>
                ))}
              </select>
            </label>
            <label className="span-2">
              <span>Statement file</span>
              <input
                type="file"
                accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={(e) => {
                  setError(null)
                  setFile(e.target.files?.[0] ?? null)
                }}
                required
              />
            </label>
          </div>

          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}

          <div className="settings-actions">
            <button type="button" className="ghost" onClick={close}>
              Cancel
            </button>
            <button type="submit" className="primary" disabled={busy || !file}>
              {busy ? 'Reading…' : 'Preview'}
            </button>
          </div>
        </form>
      ) : (
        <>
          <p className="card-sub">
            {FORMAT_LABEL[preview.format]} statement for {acct?.name ?? 'a removed account'}:{' '}
            {[
              plural(count('new'), 'new line', 'new lines'),
              count('imported') && `${count('imported')} already imported`,
              count('duplicate') && plural(count('duplicate'), 'possible duplicate', 'possible duplicates'),
              count('before_opening') && `${count('before_opening')} before the opening date`,
              preview.skipped && plural(preview.skipped, 'row skipped', 'rows skipped'),
            ]
              .filter(Boolean)
              .join(' · ')}
          </p>

          {lines.length === 0 ? (
            <p className="empty imp-none">This statement has no lines to import.</p>
          ) : (
            <>
              <div className="imp-tools">
                <span className="card-sub">{chosen.length} selected</span>
                <button
                  type="button"
                  className="ghost small"
                  onClick={() => selectAll(true)}
                  disabled={busy || !anyImportable}
                >
                  Select all
                </button>
                <button
                  type="button"
                  className="ghost small"
                  onClick={() => selectAll(false)}
                  disabled={busy || !anyImportable}
                >
                  Select none
                </button>
              </div>

              <div className="imp-scroll">
                <table className="data-table imp-table">
                  <caption className="sr-only">Statement lines</caption>
                  <thead>
                    <tr>
                      <th scope="col">
                        <span className="sr-only">Import</span>
                      </th>
                      <th scope="col">Date</th>
                      <th scope="col">Description</th>
                      <th scope="col" className="num">
                        Amount
                      </th>
                      <th scope="col">Type</th>
                      <th scope="col">Category or account</th>
                      <th scope="col">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {lines.map((l, i) => {
                      const d = drafts[l.key]
                      if (!d) return null
                      const done = sent.has(l.key)
                      const open = importable(l)
                      const locked = busy || !open
                      const name = `${l.description}, ${shortDay(l.day)}`
                      const incoming = l.amount_minor > 0
                      const transfer = d.kind === 'transfer'
                      const rowClass = [!open && 'is-off', bad === l.key && 'is-bad'].filter(Boolean).join(' ')
                      return (
                        <tr key={l.key} id={`imp-${i}`} className={rowClass || undefined}>
                          <td className="imp-check">
                            <input
                              type="checkbox"
                              checked={open && d.checked}
                              disabled={locked}
                              onChange={(e) => update(l.key, { checked: e.target.checked })}
                              aria-label={`Import ${name}`}
                            />
                          </td>
                          <td className="imp-date muted nowrap">{shortDay(l.day)}</td>
                          <td className="imp-entry">
                            <input
                              value={d.merchant}
                              maxLength={80}
                              disabled={locked}
                              onChange={(e) => update(l.key, { merchant: e.target.value })}
                              aria-label={`Description for ${name}`}
                            />
                            <span className="imp-bank">{l.description}</span>
                          </td>
                          <td
                            className={`num imp-amount ${transfer ? 'money-moved' : incoming ? 'money-in' : ''}`}
                          >
                            {formatMoney(l.amount_minor, l.currency, { signed: true })}
                          </td>
                          <td className="imp-kind">
                            <select
                              value={d.kind}
                              disabled={locked}
                              onChange={(e) => update(l.key, { kind: e.target.value as TransactionKind })}
                              aria-label={`Type of ${name}`}
                            >
                              {ALL_KINDS.map((k) => (
                                <option key={k} value={k}>
                                  {k === 'expense' && incoming ? 'Money back' : KIND_LABEL[k]}
                                </option>
                              ))}
                            </select>
                          </td>
                          <td className="imp-cat">
                            {d.kind === 'transfer' ? (
                              <select
                                value={d.other}
                                disabled={locked}
                                onChange={(e) => update(l.key, { other: e.target.value })}
                                aria-label={`${incoming ? 'Account it came from' : 'Account it went to'}, ${name}`}
                              >
                                <option value="" disabled>
                                  Pick the account
                                </option>
                                {others.map((a) => (
                                  <option key={a.id} value={a.id}>
                                    {incoming ? 'from' : 'to'} {a.name}
                                  </option>
                                ))}
                              </select>
                            ) : (
                              <select
                                value={d.category}
                                disabled={locked}
                                onChange={(e) => update(l.key, { category: e.target.value })}
                                aria-label={`Category of ${name}`}
                              >
                                <option value="" disabled>
                                  Pick a category
                                </option>
                                {categories.byKind[d.kind].map((c) => (
                                  <option key={c.id} value={c.id}>
                                    {c.name}
                                  </option>
                                ))}
                              </select>
                            )}
                          </td>
                          <td className="imp-status">
                            <span className="imp-badges">
                              {done ? (
                                <span className="imp-badge is-done">Imported</span>
                              ) : l.status === 'imported' ? (
                                <span className="imp-badge">
                                  Already imported{l.match && ` as ${l.match.merchant}`}
                                </span>
                              ) : l.status === 'duplicate' ? (
                                <span className="imp-badge is-dup">
                                  Possible duplicate
                                  {l.match && ` of ${l.match.merchant} on ${dateLabel(l.match.occurred_at)}`}
                                </span>
                              ) : l.status === 'before_opening' ? (
                                <span className="imp-badge">Before opening date</span>
                              ) : null}
                              {l.learned && (
                                <span
                                  className="imp-badge is-learned"
                                  title="Suggested from an earlier import of the same counterparty"
                                >
                                  learned
                                </span>
                              )}
                            </span>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <p className="sr-only" role="status">
            {progress ? `Importing ${progress.done} of ${progress.total}` : ''}
          </p>

          <div className="settings-actions">
            <button type="button" className="ghost push-left" onClick={back} disabled={busy}>
              Change file
            </button>
            <button type="button" className="ghost" onClick={close} disabled={busy}>
              {sent.size ? 'Close' : 'Cancel'}
            </button>
            <button type="button" className="primary" onClick={runImport} disabled={busy || chosen.length === 0}>
              {progress ? `Importing ${progress.done}/${progress.total}…` : `Import ${chosen.length}`}
            </button>
          </div>
        </>
      )}
    </dialog>
  )
}

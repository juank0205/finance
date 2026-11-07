import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api } from '../api/client.ts'
import type { Recurring, RecurringInput, RepeatUnit } from '../api/types.ts'
import { type CategoryIndex } from '../categories.ts'
import { amountInput, formatMoney } from '../format.ts'
import { dayLabel, describe, nextOccurrence, occurrences, toDay } from '../recurrence.ts'
import { checkEntry, useEntry } from '../entry.ts'
import { EntryFields } from './EntryFields.tsx'
import { Swatch } from './Legend.tsx'

interface Props {
  categories: CategoryIndex
  currency: string
  today: Date
  /** Something was created, changed or deleted: the month data must be re-fetched. */
  onChanged: (message: string) => void
  onClose: () => void
}

const UNITS: { value: RepeatUnit; one: string; many: string }[] = [
  { value: 'week', one: 'week', many: 'weeks' },
  { value: 'month', one: 'month', many: 'months' },
  { value: 'year', one: 'year', many: 'years' },
]

/** What a rule costs per month on average, to show fixed costs at a glance. */
function perMonth(r: Recurring): number {
  const factor = r.unit === 'week' ? 52 / 12 : r.unit === 'month' ? 1 : 1 / 12
  return (r.amount_minor * factor) / r.every
}

export function RecurringPanel({ categories, currency, today, onChanged, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [rules, setRules] = useState<Recurring[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  // null: the list. {}: adding. {rule}: editing that rule.
  const [form, setForm] = useState<{ rule?: Recurring } | null>(null)
  const [reload, setReload] = useState(0)
  const todayDay = toDay(today)

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  useEffect(() => {
    const ctrl = new AbortController()
    api
      .then((a) => a.recurring(ctrl.signal))
      .then(setRules)
      .catch((err: unknown) => !ctrl.signal.aborted && setLoadError(String(err)))
    return () => ctrl.abort()
  }, [reload])

  const done = (message: string) => {
    setForm(null)
    setRules(null) // show "Loading…" rather than the stale list until the fresh one arrives
    setReload((n) => n + 1)
    onChanged(message)
  }

  const active = (r: Recurring) => !r.end_date || r.end_date >= todayDay
  const fixedCosts = (rules ?? []).filter((r) => r.kind === 'expense' && active(r)).reduce((s, r) => s + perMonth(r), 0)

  return (
    <dialog ref={dialog} className="settings wide" onClose={onClose} aria-labelledby="rec-title">
      {form ? (
        <RuleForm
          key={form.rule?.id ?? 'new'}
          rule={form.rule}
          categories={categories}
          currency={currency}
          todayDay={todayDay}
          onDone={done}
          onCancel={() => setForm(null)}
        />
      ) : (
        <>
          <h2 id="rec-title">Recurring</h2>
          <p className="card-sub">
            Entries that repeat, like the mortgage or the phone bill, are added automatically on their day. Use
            this for direct debits and transfers: card payments made with Google Wallet already arrive from your
            phone and would be counted twice.
          </p>

          {loadError && (
            <p className="error" role="alert">
              Could not load: {loadError}
            </p>
          )}
          {!rules && !loadError && <p className="loading">Loading…</p>}
          {rules && rules.length === 0 && <p className="empty">Nothing recurring yet.</p>}

          {rules && rules.length > 0 && (
            <ul className="rule-list">
              {rules.map((r) => {
                const c = categories.get(r.category, r.kind)
                const next = nextOccurrence(r, todayDay)
                return (
                  <li key={r.id} className={active(r) ? undefined : 'is-ended'}>
                    <div className="rule-main">
                      <span className="rule-name">
                        {c.color > 0 && <Swatch color={c.color} />}
                        {r.merchant}
                      </span>
                      <span className="rule-meta">
                        {describe(r)} · {c.name}
                        {r.kind !== 'expense' && ` · ${r.kind === 'income' ? 'Income' : 'Investment'}`}
                      </span>
                      <span className="rule-meta">
                        {next
                          ? `Next: ${dayLabel(next)}`
                          : r.end_date
                            ? `Ended ${dayLabel(r.end_date)}`
                            : 'No more dates'}
                      </span>
                    </div>
                    <span className={`rule-amount ${r.kind === 'income' ? 'money-in' : ''}`}>
                      {formatMoney(r.kind === 'income' ? r.amount_minor : -r.amount_minor, r.currency, {
                        signed: true,
                      })}
                    </span>
                    <button
                      type="button"
                      className="ghost small"
                      onClick={() => setForm({ rule: r })}
                      aria-label={`Edit ${r.merchant}`}
                    >
                      Edit
                    </button>
                  </li>
                )
              })}
            </ul>
          )}

          {rules && rules.length > 0 && (
            <p className="rule-total">
              Fixed costs: <strong>{formatMoney(Math.round(fixedCosts), currency)}</strong> per month on average
            </p>
          )}

          <div className="settings-actions">
            <button type="button" className="ghost push-left" onClick={() => setForm({})}>
              + Add recurring
            </button>
            <button type="button" className="primary" onClick={onClose}>
              Done
            </button>
          </div>
        </>
      )}
    </dialog>
  )
}

interface RuleFormProps {
  rule?: Recurring
  categories: CategoryIndex
  currency: string
  todayDay: string
  onDone: (message: string) => void
  onCancel: () => void
}

function RuleForm({ rule, categories, currency, todayDay, onDone, onCancel }: RuleFormProps) {
  const cur = rule?.currency ?? currency
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [start, setStart] = useState(rule?.start_date ?? todayDay)
  const [end, setEnd] = useState(rule?.end_date ?? '')
  const [every, setEvery] = useState(String(rule?.every ?? 1))
  const [unit, setUnit] = useState<RepeatUnit>(rule?.unit ?? 'month')
  const clear = () => setError(null)
  const entry = useEntry(
    rule
      ? {
          kind: rule.kind,
          amount: amountInput(rule.amount_minor, rule.currency),
          merchant: rule.merchant,
          category: categories.get(rule.category, rule.kind).id,
          note: rule.note ?? '',
        }
      : { category: categories.ids.expense.has('bills') ? 'bills' : undefined },
    categories,
    clear,
  )

  const n = Number(every)
  const schedule = { start_date: start, end_date: end || undefined, every: n, unit }
  const scheduleOk = !!start && Number.isInteger(n) && n >= 1 && n <= 12 && (!end || end >= start)
  // Only a new rule backfills; an edited one keeps the entries it already made.
  const past = scheduleOk && !rule ? occurrences(schedule, start, todayDay).length : 0
  const next = scheduleOk ? nextOccurrence(schedule, todayDay) : undefined

  const build = (endOverride?: string): RecurringInput | string => {
    const checked = checkEntry(entry.values, cur)
    if ('error' in checked) return checked.error
    if (!start) return 'Pick a start date.'
    if (!Number.isInteger(n) || n < 1 || n > 12) return 'Repeat every 1 to 12.'
    const endDate = endOverride ?? (end || undefined)
    if (endDate && endDate < start) return 'The end date must be on or after the start date.'
    return { ...checked, currency: cur, start_date: start, end_date: endDate, every: n, unit }
  }

  const run = async (fn: () => Promise<string>) => {
    setBusy(true)
    setError(null)
    try {
      onDone(await fn())
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setBusy(false)
    }
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const input = build()
    if (typeof input === 'string') return setError(input)
    void run(async () => {
      const a = await api
      if (rule) {
        await a.updateRecurring(rule.id, input)
        return `Saved “${input.merchant}”. Entries already added are unchanged.`
      }
      await a.createRecurring(input)
      return past > 0
        ? `Added “${input.merchant}” and ${past} entr${past === 1 ? 'y' : 'ies'} up to today.`
        : `Added “${input.merchant}”.`
    })
  }

  const stop = () => {
    // Ends after today's occurrence, if there is one.
    const input = build(todayDay < start ? start : todayDay)
    if (typeof input === 'string') return setError(input)
    void run(async () => {
      await (await api).updateRecurring(rule!.id, input)
      return `Stopped “${input.merchant}”. No new entries will be added.`
    })
  }

  const remove = () =>
    run(async () => {
      await (await api).deleteRecurring(rule!.id)
      return `Deleted “${rule!.merchant}”. Entries already added are kept.`
    })

  return (
    <form onSubmit={submit} noValidate>
      <h2 id="rec-title">{rule ? 'Edit recurring' : 'Add recurring'}</h2>

      <EntryFields
        entry={entry}
        categories={categories}
        currency={cur}
        dateField={
          <label>
            <span>Starts</span>
            <input
              type="date"
              value={start}
              onChange={(e) => {
                clear()
                setStart(e.target.value)
              }}
              required
            />
          </label>
        }
      >
        <div className="span-2 repeat-row">
          <label>
            <span>Repeat every</span>
            <span className="repeat-every">
              <input
                type="number"
                min={1}
                max={12}
                value={every}
                onChange={(e) => {
                  clear()
                  setEvery(e.target.value)
                }}
                aria-label="Repeat interval"
              />
              <select
                value={unit}
                onChange={(e) => {
                  clear()
                  setUnit(e.target.value as RepeatUnit)
                }}
                aria-label="Repeat unit"
              >
                {UNITS.map((u) => (
                  <option key={u.value} value={u.value}>
                    {n === 1 ? u.one : u.many}
                  </option>
                ))}
              </select>
            </span>
          </label>
          <label>
            <span>
              Ends <span className="optional">(optional)</span>
            </span>
            <input
              type="date"
              value={end}
              min={start}
              onChange={(e) => {
                clear()
                setEnd(e.target.value)
              }}
            />
          </label>
        </div>
      </EntryFields>

      {scheduleOk && (
        <p className="card-sub form-hint" aria-live="polite">
          {describe(schedule)}
          {next ? `, next on ${dayLabel(next)}` : ', no more dates'}.
          {past > 0 && ` ${past} past entr${past === 1 ? 'y' : 'ies'} since ${dayLabel(start)} will be added now.`}
          {rule && ' Changes apply to entries not added yet.'}
        </p>
      )}
      {!scheduleOk && start && end && end < start && (
        <p className="card-sub form-hint">The end date is before the start date.</p>
      )}

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      <div className="settings-actions">
        {rule &&
          (confirmDelete ? (
            <button type="button" className="danger push-left" onClick={remove} disabled={busy}>
              Really delete?
            </button>
          ) : (
            <>
              <button type="button" className="ghost push-left" onClick={() => setConfirmDelete(true)} disabled={busy}>
                Delete
              </button>
              {(!rule.end_date || rule.end_date > todayDay) && (
                <button type="button" className="ghost" onClick={stop} disabled={busy}>
                  Stop
                </button>
              )}
            </>
          ))}
        <button type="button" className="ghost" onClick={onCancel}>
          Back
        </button>
        <button type="submit" className="primary" disabled={busy}>
          {busy ? 'Saving…' : rule ? 'Save' : 'Add'}
        </button>
      </div>
    </form>
  )
}

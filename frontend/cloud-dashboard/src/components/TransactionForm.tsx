import { useEffect, useRef, useState, type FormEvent } from 'react'
import type { Transaction, TransactionInput } from '../api/types.ts'
import { kindOf, type CategoryIndex } from '../categories.ts'
import { amountInput } from '../format.ts'
import { dayToInstant, toDay } from '../recurrence.ts'
import { checkEntry, useEntry } from '../entry.ts'
import { EntryFields } from './EntryFields.tsx'

interface Props {
  categories: CategoryIndex
  currency: string
  today: Date
  /** Set to edit a manual or recurring entry; unset to add a new one. */
  editing?: Transaction
  onSubmit: (input: TransactionInput) => Promise<void>
  onDelete?: () => Promise<void>
  onClose: () => void
}

export function TransactionForm({ categories, currency, today, editing, onSubmit, onDelete, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const cur = editing?.currency ?? currency
  const kind = editing ? kindOf(editing) : 'expense'
  const initialDate = editing ? toDay(new Date(editing.occurred_at)) : toDay(today)
  const [date, setDate] = useState(initialDate)
  const entry = useEntry(
    editing
      ? {
          kind,
          amount: amountInput(editing.amount_minor, editing.currency),
          merchant: editing.merchant,
          category: categories.get(editing.category, kind).id,
          note: editing.note ?? '',
        }
      : {},
    categories,
    () => setError(null),
  )
  const fromRule = editing?.source === 'recurring'

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const checked = checkEntry(entry.values, cur)
    if ('error' in checked) return setError(checked.error)
    if (!date || date > toDay(today)) return setError('Pick a date that is not in the future.')

    setBusy(true)
    setError(null)
    try {
      await onSubmit({
        ...checked,
        currency: cur,
        // Unchanged date: keep the original time so the entry doesn't move within its day.
        occurred_at: editing && date === initialDate ? editing.occurred_at : dayToInstant(date),
      })
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setBusy(false)
    }
  }

  const remove = async () => {
    if (!onDelete) return
    setBusy(true)
    try {
      await onDelete()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setBusy(false)
    }
  }

  return (
    <dialog ref={dialog} className="settings" onClose={onClose} aria-labelledby="tx-title">
      <form onSubmit={submit} noValidate>
        <h2 id="tx-title">{editing ? 'Edit entry' : 'Add entry'}</h2>
        {fromRule && (
          <p className="card-sub">
            Created by a recurring entry. Changes apply to this one only; deleting it skips this date for good.
          </p>
        )}

        <EntryFields
          entry={entry}
          categories={categories}
          currency={cur}
          dateField={
            <label>
              <span>Date</span>
              <input
                type="date"
                value={date}
                max={toDay(today)}
                onChange={(e) => {
                  setError(null)
                  setDate(e.target.value)
                }}
                required
              />
            </label>
          }
        />

        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}

        <div className="settings-actions">
          {onDelete &&
            (confirmDelete ? (
              <button type="button" className="danger push-left" onClick={remove} disabled={busy}>
                {fromRule ? 'Really skip?' : 'Really delete?'}
              </button>
            ) : (
              <button type="button" className="ghost push-left" onClick={() => setConfirmDelete(true)} disabled={busy}>
                {fromRule ? 'Skip this one' : 'Delete'}
              </button>
            ))}
          <button type="button" className="ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={busy}>
            {busy ? 'Saving…' : editing ? 'Save' : 'Add'}
          </button>
        </div>
      </form>
    </dialog>
  )
}

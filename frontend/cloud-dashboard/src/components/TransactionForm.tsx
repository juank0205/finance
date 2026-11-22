import { useEffect, useRef, useState, type FormEvent } from 'react'
import type { Account, Transaction, TransactionInput } from '../api/types.ts'
import { kindOf, type CategoryIndex } from '../categories.ts'
import { amountInput } from '../format.ts'
import { dayToInstant, toDay } from '../recurrence.ts'
import { checkTransaction, useEntry } from '../entry.ts'
import { EntryFields } from './EntryFields.tsx'

interface Props {
  categories: CategoryIndex
  accounts: Account[]
  /** The summary currency: used when no account is picked. */
  currency: string
  today: Date
  /** Set to edit a manual or recurring entry; unset to add a new one. */
  editing?: Transaction
  onSubmit: (input: TransactionInput) => Promise<void>
  onDelete?: () => Promise<void>
  onClose: () => void
}

export function TransactionForm({ categories, accounts, currency, today, editing, onSubmit, onDelete, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const kind = editing ? kindOf(editing) : 'expense'
  const initialDate = editing ? toDay(new Date(editing.occurred_at)) : toDay(today)
  const [date, setDate] = useState(initialDate)
  // An account deleted since can't be saved again; the entry starts without one.
  const known = (id?: string) => accounts.find((a) => a.id === id)
  const toAccount = known(editing?.to_account)
  const entry = useEntry(
    editing
      ? {
          kind,
          amount: amountInput(editing.amount_minor, editing.currency),
          currency: editing.currency,
          merchant: editing.merchant,
          category: kind === 'transfer' ? '' : categories.get(editing.category, kind).id,
          note: editing.note ?? '',
          account: known(editing.account)?.id ?? '',
          refund: !!editing.refund,
          toAccount: toAccount?.id ?? '',
          toAmount: toAccount && editing.to_minor ? amountInput(editing.to_minor, toAccount.currency) : '',
        }
      : {},
    categories,
    () => setError(null),
    { accounts, currency },
  )
  const fromRule = editing?.source === 'recurring'
  const v = entry.values
  // The summary currency, every account's, and whatever this entry already uses.
  const currencies = [...new Set([currency, ...accounts.map((a) => a.currency), v.currency])]

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const checked = checkTransaction(v, accounts)
    if ('error' in checked) return setError(checked.error)
    if (!date || date > toDay(today)) return setError('Pick a date that is not in the future.')

    setBusy(true)
    setError(null)
    try {
      await onSubmit({
        ...checked,
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
          accounts={{ list: accounts, currencies }}
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

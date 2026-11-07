import { useState } from 'react'
import type { TransactionKind } from './api/types.ts'
import { CATCH_ALL, type CategoryIndex } from './categories.ts'
import { parseAmount } from './format.ts'

// The fields a single entry and a recurring rule share: type, amount, description, category, note.

export const MERCHANT_LABEL: Record<TransactionKind, string> = {
  expense: 'Where or what',
  income: 'From',
  investment: 'Into',
}

export const MERCHANT_HINT: Record<TransactionKind, string> = {
  expense: 'e.g. Fresh Market',
  income: 'e.g. Employer payroll',
  investment: 'e.g. Broker monthly plan',
}

export interface EntryValues {
  kind: TransactionKind
  amount: string // as typed
  merchant: string
  category: string
  note: string
}

export function useEntry(initial: Partial<EntryValues>, categories: CategoryIndex, onChange: () => void) {
  const kind = initial.kind ?? 'expense'
  const [values, setValues] = useState<EntryValues>({
    kind,
    amount: initial.amount ?? '',
    merchant: initial.merchant ?? '',
    category: initial.category ?? CATCH_ALL[kind],
    note: initial.note ?? '',
  })

  const set = <K extends keyof EntryValues>(key: K, value: EntryValues[K]) => {
    onChange()
    setValues((v) => {
      const next = { ...v, [key]: value }
      // Categories belong to one kind; keep the choice only if it still fits.
      if (key === 'kind' && !categories.ids[next.kind].has(next.category)) next.category = CATCH_ALL[next.kind]
      return next
    })
  }

  return { values, set }
}

/** Checks the shared fields. Returns the clean values, or a message for the person. */
export function checkEntry(
  v: EntryValues,
  currency: string,
): { error: string } | { kind: TransactionKind; amount_minor: number; merchant: string; category: string; note?: string } {
  const minor = parseAmount(v.amount, currency)
  if (minor === null) {
    return {
      error: /[.,]\d{3,}$/.test(v.amount.trim()) ? 'Use at most 2 decimals.' : 'Enter an amount above 0, like 12.50.',
    }
  }
  if (!v.merchant.trim()) return { error: `Fill in "${MERCHANT_LABEL[v.kind]}".` }
  return {
    kind: v.kind,
    amount_minor: minor,
    merchant: v.merchant.trim(),
    category: v.category,
    note: v.note.trim() || undefined,
  }
}

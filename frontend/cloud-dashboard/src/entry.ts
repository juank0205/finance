import { useState } from 'react'
import type { Account, TransactionInput, TransactionKind } from './api/types.ts'
import { CATCH_ALL, type CategoryIndex } from './categories.ts'
import { parseAmount } from './format.ts'

// The fields a single entry and a recurring rule share: type, amount, description, category, note.
// Single entries also have an account, a currency of their own, money back, and transfers.

export const MERCHANT_LABEL: Record<TransactionKind, string> = {
  expense: 'Where or what',
  income: 'From',
  investment: 'Into',
  transfer: 'Description',
}

export const MERCHANT_HINT: Record<TransactionKind, string> = {
  expense: 'e.g. Fresh Market',
  income: 'e.g. Employer payroll',
  investment: 'e.g. Broker monthly plan',
  transfer: 'e.g. Savings top-up',
}

export interface EntryValues {
  kind: TransactionKind
  amount: string // as typed
  currency: string
  merchant: string
  category: string // ignored for transfers
  note: string
  account: string // Account.id; "" = none. A transfer's From.
  refund: boolean // expenses only: money back
  toAccount: string // transfers only
  toAmount: string // transfers only, as typed; "" = converted at the day's rate
}

/** Single entries only: the accounts to pick from, and the currency to use without one. */
export interface AccountChoice {
  accounts: Account[]
  currency: string
}

export function useEntry(
  initial: Partial<EntryValues>,
  categories: CategoryIndex,
  onChange: () => void,
  choice?: AccountChoice,
) {
  const kind = initial.kind ?? 'expense'
  const [values, setValues] = useState<EntryValues>({
    kind,
    amount: initial.amount ?? '',
    currency: initial.currency ?? choice?.currency ?? 'EUR',
    merchant: initial.merchant ?? '',
    category: initial.category ?? (kind === 'transfer' ? '' : CATCH_ALL[kind]),
    note: initial.note ?? '',
    account: initial.account ?? '',
    refund: initial.refund ?? false,
    toAccount: initial.toAccount ?? '',
    toAmount: initial.toAmount ?? '',
  })

  const set = <K extends keyof EntryValues>(key: K, value: EntryValues[K]) => {
    onChange()
    setValues((v) => {
      const next = { ...v, [key]: value }
      // Categories belong to one kind; keep the choice only if it still fits. Transfers ignore it.
      if (key === 'kind' && next.kind !== 'transfer' && !categories.ids[next.kind].has(next.category)) {
        next.category = CATCH_ALL[next.kind]
      }
      // The currency follows the account: a transfer is always in its From account's, anything else
      // starts in it and can be changed.
      if (choice && (key === 'account' || key === 'kind')) {
        const own = choice.accounts.find((a) => a.id === next.account)?.currency
        if (own && (key === 'account' || next.kind === 'transfer')) next.currency = own
        else if (key === 'account') next.currency = choice.currency
      }
      // The To list leaves out the From account.
      if (key === 'account' && next.toAccount === next.account) next.toAccount = ''
      return next
    })
  }

  return { values, set }
}

/** Checks the shared fields. Returns the clean values, or a message for the person. */
export function checkEntry(
  v: EntryValues,
): { error: string } | { kind: TransactionKind; amount_minor: number; currency: string; merchant: string; category: string; note?: string } {
  const minor = parseAmount(v.amount, v.currency)
  if (minor === null) return { error: amountProblem(v.amount) }
  if (!v.merchant.trim()) return { error: `Fill in "${MERCHANT_LABEL[v.kind]}".` }
  return {
    kind: v.kind,
    amount_minor: minor,
    currency: v.currency,
    merchant: v.merchant.trim(),
    category: v.kind === 'transfer' ? '' : v.category,
    note: v.note.trim() || undefined,
  }
}

function amountProblem(typed: string): string {
  return /[.,]\d{3,}$/.test(typed.trim()) ? 'Use at most 2 decimals.' : 'Enter an amount above 0, like 12.50.'
}

/** Checks a single entry, accounts included. Mirrors the server's checks, so most mistakes show here. */
export function checkTransaction(
  v: EntryValues,
  accounts: Account[],
): { error: string } | Omit<TransactionInput, 'occurred_at'> {
  const checked = checkEntry(v)
  if ('error' in checked) return checked
  if (v.kind !== 'transfer') {
    return {
      ...checked,
      account: v.account || undefined,
      refund: (v.kind === 'expense' && v.refund) || undefined,
    }
  }

  const from = accounts.find((a) => a.id === v.account)
  const to = accounts.find((a) => a.id === v.toAccount)
  if (!from) return { error: 'Pick the account the money left.' }
  if (!to) return { error: 'Pick the account the money went to.' }
  if (from.id === to.id) return { error: 'Pick two different accounts.' }
  // Same currency: what arrives is what left, so there is nothing to type.
  let received: number | undefined
  if (from.currency !== to.currency && v.toAmount.trim()) {
    const minor = parseAmount(v.toAmount, to.currency)
    if (minor === null) return { error: `Amount received: ${amountProblem(v.toAmount)} Or leave it blank.` }
    received = minor
  }
  return {
    ...checked,
    currency: from.currency,
    account: from.id,
    to_account: to.id,
    to_amount_minor: received,
  }
}

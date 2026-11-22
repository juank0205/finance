import { useMemo, useState } from 'react'
import type { Account, Category, CategoryKind, Source, Transaction, TransactionKind } from '../api/types.ts'
import { KINDS, KIND_LABEL, kindOf, type CategoryIndex } from '../categories.ts'
import { dateLabel, formatMoney } from '../format.ts'
import { Swatch } from './Legend.tsx'

const SOURCE_LABEL: Record<Source, string> = {
  google_wallet: 'Wallet',
  manual: 'Manual',
  statement: 'Statement',
  recurring: 'Recurring',
}

const GROUP_LABEL: Record<CategoryKind, string> = {
  expense: 'Expenses',
  income: 'Income',
  investment: 'Investments',
}

type SortKey = 'date' | 'amount'

/** Filter values: a Category.id, or "kind:<kind>" for every entry of one type. */
const kindFilter = (k: TransactionKind) => `kind:${k}`

function CategoryCell({ category, kind }: { category: Category; kind: CategoryKind }) {
  return (
    <span className="cat-cell">
      {category.color > 0 && <Swatch color={category.color} />}
      {category.name}
      {kind !== 'expense' && <span className="kind-tag">{KIND_LABEL[kind]}</span>}
    </span>
  )
}

/** Sorting by amount compares the summary-currency value: 50,000 COP is less than 50 EUR. */
const sortAmount = (t: Transaction) => Math.abs(t.base_minor ?? t.amount_minor)

interface Props {
  transactions: Transaction[]
  categories: CategoryIndex
  accounts: Account[]
  filter: string | null
  onFilter: (filter: string | null) => void
  onEdit: (t: Transaction) => void
}

export function TransactionsTable({ transactions, categories, accounts, filter, onFilter, onEdit }: Props) {
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean }>({ key: 'date', desc: true })
  const accountById = useMemo(() => new Map(accounts.map((a) => [a.id, a])), [accounts])
  // Entries keep the id of an account deleted since; it no longer counts toward any balance.
  const accountName = (id?: string) => (id ? (accountById.get(id)?.name ?? 'Removed account') : '')

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase()
    const filtered = transactions.filter((t) => {
      const kind = kindOf(t)
      if (filter) {
        const ok = filter.startsWith('kind:')
          ? filter === kindFilter(kind)
          : kind !== 'transfer' && categories.get(t.category, kind).id === filter
        if (!ok) return false
      }
      return !q || t.merchant.toLowerCase().includes(q) || !!t.note?.toLowerCase().includes(q)
    })
    const dir = sort.desc ? -1 : 1
    return filtered.sort((a, b) =>
      sort.key === 'date'
        ? dir * a.occurred_at.localeCompare(b.occurred_at)
        : dir * (sortAmount(a) - sortAmount(b)),
    )
  }, [transactions, categories, filter, query, sort])

  const sortButton = (key: SortKey, label: string) => {
    const on = sort.key === key
    return (
      <button type="button" className="sort" onClick={() => setSort({ key, desc: on ? !sort.desc : true })}>
        {label}
        <span aria-hidden="true">{on ? (sort.desc ? ' ↓' : ' ↑') : ''}</span>
      </button>
    )
  }

  const ariaSort = (key: SortKey) => (sort.key === key ? (sort.desc ? 'descending' : 'ascending') : 'none')

  return (
    <section className="card">
      <header className="card-head">
        <div>
          <h2>Transactions</h2>
          <p className="card-sub">
            {rows.length} of {transactions.length}
          </p>
        </div>
        <div className="filters">
          <input
            type="search"
            placeholder="Search"
            aria-label="Search description or note"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <select aria-label="Filter" value={filter ?? ''} onChange={(e) => onFilter(e.target.value || null)}>
            <option value="">Everything</option>
            {KINDS.map((k) => (
              <optgroup key={k} label={GROUP_LABEL[k]}>
                <option value={kindFilter(k)}>All {GROUP_LABEL[k].toLowerCase()}</option>
                {categories.byKind[k].map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </optgroup>
            ))}
            <option value={kindFilter('transfer')}>Transfers</option>
          </select>
        </div>
      </header>

      <div className="table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              <th scope="col" aria-sort={ariaSort('date')}>
                {sortButton('date', 'Date')}
              </th>
              <th scope="col">Description</th>
              <th scope="col" className="hide-sm">
                Category
              </th>
              <th scope="col" className="hide-sm">
                Account
              </th>
              <th scope="col" className="hide-sm">
                Source
              </th>
              <th scope="col" className="num" aria-sort={ariaSort('amount')}>
                {sortButton('amount', 'Amount')}
              </th>
              <th scope="col">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((t) => {
              const kind = kindOf(t)
              const received = kind === 'transfer' ? accountById.get(t.to_account ?? '') : undefined
              return (
                <tr key={t.id}>
                  <td className="muted nowrap">{dateLabel(t.occurred_at)}</td>
                  <td>
                    {t.merchant}
                    {t.note && <span className="tx-note">{t.note}</span>}
                  </td>
                  <td className="muted hide-sm">
                    {kind === 'transfer' ? (
                      <span className="kind-tag">{KIND_LABEL.transfer}</span>
                    ) : (
                      <CategoryCell category={categories.get(t.category, kind)} kind={kind} />
                    )}
                  </td>
                  <td className="muted hide-sm">
                    {kind === 'transfer' ? (
                      <>
                        {accountName(t.account)} <span aria-hidden="true">→</span>
                        <span className="sr-only">to</span> {accountName(t.to_account)}
                      </>
                    ) : (
                      accountName(t.account)
                    )}
                  </td>
                  <td className="muted hide-sm">
                    {SOURCE_LABEL[t.source]}
                    {t.card_last4 && ` ···${t.card_last4}`}
                  </td>
                  {kind === 'transfer' ? (
                    // Your money only moved, so neither sign nor color.
                    <td className="num money-moved">
                      {formatMoney(Math.abs(t.amount_minor), t.currency)}
                      {received && t.to_minor && received.currency !== t.currency ? (
                        <span className="tx-note">
                          <span aria-hidden="true">→</span>
                          <span className="sr-only">received</span> {formatMoney(t.to_minor, received.currency)}
                        </span>
                      ) : null}
                    </td>
                  ) : (
                    <td className={`num ${kind === 'income' || t.refund ? 'money-in' : ''}`}>
                      {formatMoney(t.amount_minor, t.currency, { signed: true })}
                      {t.refund && <span className="tx-note">Money back</span>}
                    </td>
                  )}
                  <td className="row-actions">
                    {/* Imported entries are re-created by the next sync, so they aren't editable. */}
                    {(t.source === 'manual' || t.source === 'recurring') && (
                      <button
                        type="button"
                        className="ghost small"
                        onClick={() => onEdit(t)}
                        aria-label={`Edit ${t.merchant}`}
                      >
                        Edit
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
            {rows.length === 0 && (
              <tr>
                <td colSpan={7} className="empty">
                  No transactions match.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  )
}

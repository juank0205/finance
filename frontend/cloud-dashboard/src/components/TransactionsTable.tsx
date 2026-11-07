import { useMemo, useState } from 'react'
import type { Category, Source, Transaction, TransactionKind } from '../api/types.ts'
import { KINDS, KIND_LABEL, kindOf, type CategoryIndex } from '../categories.ts'
import { dateLabel, formatMoney } from '../format.ts'
import { Swatch } from './Legend.tsx'

const SOURCE_LABEL: Record<Source, string> = {
  google_wallet: 'Wallet',
  manual: 'Manual',
  statement: 'Statement',
  recurring: 'Recurring',
}

const GROUP_LABEL: Record<TransactionKind, string> = {
  expense: 'Expenses',
  income: 'Income',
  investment: 'Investments',
}

type SortKey = 'date' | 'amount'

/** Filter values: a Category.id, or "kind:<kind>" for every entry of one type. */
const kindFilter = (k: TransactionKind) => `kind:${k}`

function CategoryCell({ category, kind }: { category: Category; kind: TransactionKind }) {
  return (
    <span className="cat-cell">
      {category.color > 0 && <Swatch color={category.color} />}
      {category.name}
      {kind !== 'expense' && <span className="kind-tag">{KIND_LABEL[kind]}</span>}
    </span>
  )
}

interface Props {
  transactions: Transaction[]
  categories: CategoryIndex
  filter: string | null
  onFilter: (filter: string | null) => void
  onEdit: (t: Transaction) => void
}

export function TransactionsTable({ transactions, categories, filter, onFilter, onEdit }: Props) {
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean }>({ key: 'date', desc: true })

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase()
    const filtered = transactions.filter((t) => {
      const kind = kindOf(t)
      if (filter) {
        const ok = filter.startsWith('kind:')
          ? filter === kindFilter(kind)
          : categories.get(t.category, kind).id === filter
        if (!ok) return false
      }
      return !q || t.merchant.toLowerCase().includes(q) || !!t.note?.toLowerCase().includes(q)
    })
    const dir = sort.desc ? -1 : 1
    return filtered.sort((a, b) =>
      sort.key === 'date'
        ? dir * a.occurred_at.localeCompare(b.occurred_at)
        : dir * (Math.abs(a.amount_minor) - Math.abs(b.amount_minor)),
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
              return (
                <tr key={t.id}>
                  <td className="muted nowrap">{dateLabel(t.occurred_at)}</td>
                  <td>
                    {t.merchant}
                    {t.note && <span className="tx-note">{t.note}</span>}
                  </td>
                  <td className="muted hide-sm">
                    <CategoryCell category={categories.get(t.category, kind)} kind={kind} />
                  </td>
                  <td className="muted hide-sm">
                    {SOURCE_LABEL[t.source]}
                    {t.card_last4 && ` ···${t.card_last4}`}
                  </td>
                  <td className={`num ${kind === 'income' ? 'money-in' : ''}`}>
                    {formatMoney(t.amount_minor, t.currency, { signed: true })}
                  </td>
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
                <td colSpan={6} className="empty">
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

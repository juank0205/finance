import type { HoldingKind, NetWorth } from '../api/types.ts'
import { formatMoney } from '../format.ts'

const KIND_LABEL: Record<HoldingKind, string> = {
  cash: 'Cash',
  investment: 'Investments',
  property: 'Property',
  liability: 'Debt',
}

interface Props {
  data?: NetWorth
  error?: string
  /** Opens the account settings; offered when there are no accounts yet. */
  onAddAccounts?: () => void
}

export function NetWorthCard({ data, error, onAddAccounts }: Props) {
  if (!data) {
    return (
      <section className="card networth" aria-label="Net worth">
        <p className={error ? 'error' : 'loading'}>{error ? `Could not load net worth: ${error}` : 'Loading…'}</p>
      </section>
    )
  }

  if (data.holdings.length === 0) {
    return (
      <section className="card nw-empty" aria-label="Net worth">
        <div>
          <span className="tile-label">Net worth</span>
          <p className="empty">Add your accounts to see your net worth.</p>
        </div>
        {onAddAccounts && (
          <button type="button" className="ghost" onClick={onAddAccounts}>
            Add accounts
          </button>
        )}
      </section>
    )
  }

  // Totals are in the summary currency: each balance converted at today's rate.
  const cur = data.currency
  let assets = 0
  let debts = 0
  for (const h of data.holdings) {
    if (h.base_minor >= 0) assets += h.base_minor
    else debts += h.base_minor
  }
  const converted = data.holdings.some((h) => h.currency !== cur)
  const asOf = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(data.as_of),
  )

  return (
    <section className="card networth" aria-label="Net worth">
      <div className="nw-main">
        <span className="tile-label">Net worth</span>
        <span className="hero-value">{formatMoney(assets + debts, cur)}</span>
        <span className="delta">
          As of {asOf}
          {converted && `, other currencies at today's rate`}
        </span>
        <dl className="nw-split">
          <div>
            <dt>Assets</dt>
            <dd>{formatMoney(assets, cur)}</dd>
          </div>
          <div>
            <dt>Debts</dt>
            <dd>{formatMoney(debts, cur)}</dd>
          </div>
        </dl>
        {error && (
          <p className="error" role="alert">
            Could not refresh: {error}
          </p>
        )}
      </div>
      <table className="data-table compact nw-list">
        <caption className="sr-only">Holdings</caption>
        <tbody>
          {data.holdings.map((h) => (
            <tr key={h.id}>
              <td>{h.name}</td>
              <td className="muted">{KIND_LABEL[h.kind]}</td>
              <td className="num">
                {formatMoney(h.balance_minor, h.currency)}
                {h.currency !== cur && <span className="nw-base">≈ {formatMoney(h.base_minor, cur)}</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}

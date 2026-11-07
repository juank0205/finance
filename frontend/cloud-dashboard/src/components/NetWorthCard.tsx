import type { HoldingKind, NetWorth } from '../api/types.ts'
import { formatMoney } from '../format.ts'

const KIND_LABEL: Record<HoldingKind, string> = {
  cash: 'Cash',
  investment: 'Investments',
  property: 'Property',
  liability: 'Debt',
}

export function NetWorthCard({ data, error }: { data?: NetWorth; error?: string }) {
  if (!data) {
    return (
      <section className="card networth" aria-label="Net worth">
        <p className={error ? 'error' : 'loading'}>{error ? `Could not load net worth: ${error}` : 'Loading…'}</p>
      </section>
    )
  }

  const cur = data.currency
  let assets = 0
  let debts = 0
  for (const h of data.holdings) {
    if (h.balance_minor >= 0) assets += h.balance_minor
    else debts += h.balance_minor
  }
  const asOf = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(data.as_of),
  )

  return (
    <section className="card networth" aria-label="Net worth">
      <div className="nw-main">
        <span className="tile-label">Net worth</span>
        <span className="hero-value">{formatMoney(assets + debts, cur)}</span>
        <span className="delta">As of {asOf}</span>
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
      </div>
      <table className="data-table compact nw-list">
        <caption className="sr-only">Holdings</caption>
        <tbody>
          {data.holdings.map((h) => (
            <tr key={h.id}>
              <td>{h.name}</td>
              <td className="muted">{KIND_LABEL[h.kind]}</td>
              <td className="num">{formatMoney(h.balance_minor, cur)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}

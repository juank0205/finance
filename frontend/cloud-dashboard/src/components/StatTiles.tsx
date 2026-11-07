import type { Summary } from '../api/types.ts'
import { currentMonth, formatMoney, formatPercent, monthLabel } from '../format.ts'

interface DeltaProps {
  now: number
  before: number | undefined
  upIsGood: boolean
  vs: string
  /** Absolute difference instead of percent: for values that can change sign, like net. */
  money?: string
}

function Delta({ now, before, upIsGood, vs, money }: DeltaProps) {
  if (before === undefined || (!money && before === 0)) return <span className="delta">No data for {vs}</span>
  const diff = now - before
  const change = money ? diff : diff / Math.abs(before)
  if (money ? diff === 0 : Math.abs(change) < 0.005) return <span className="delta">Same as {vs}</span>
  const up = diff > 0
  const good = up === upIsGood
  const amount = money ? formatMoney(Math.abs(diff), money) : formatPercent(Math.abs(change))
  return (
    <span className={`delta ${good ? 'delta-good' : 'delta-bad'}`}>
      <span aria-hidden="true">{up ? '▲' : '▼'}</span> {amount} {up ? 'more' : 'less'} than {vs}
    </span>
  )
}

export function StatTiles({ summary, today }: { summary: Summary; today: Date }) {
  const cur = summary.currency
  const net = summary.income_minor - summary.spent_minor
  const prev = summary.months.at(-2)
  const vs = prev ? monthLabel(prev.month, 'long') : 'last month'
  // A month in progress compared with a full month always looks like a big drop, so skip deltas.
  const inProgress = summary.month === currentMonth(today)
  const daysInMonth = new Date(today.getFullYear(), today.getMonth() + 1, 0).getDate()

  return (
    <section className="tiles" aria-label="Month totals">
      <div className="tile">
        <span className="tile-label">
          Spent{inProgress && ' so far'}
        </span>
        <span className="tile-value">{formatMoney(summary.spent_minor, cur)}</span>
        {inProgress ? (
          <span className="delta">
            Day {today.getDate()} of {daysInMonth}
          </span>
        ) : (
          <Delta now={summary.spent_minor} before={prev?.spent_minor} upIsGood={false} vs={vs} />
        )}
      </div>
      <div className="tile">
        <span className="tile-label">Income</span>
        <span className="tile-value">{formatMoney(summary.income_minor, cur)}</span>
        {!inProgress && <Delta now={summary.income_minor} before={prev?.income_minor} upIsGood vs={vs} />}
      </div>
      <div className="tile">
        <span className="tile-label">Saved (income − spending)</span>
        <span className="tile-value">{formatMoney(net, cur, { signed: true })}</span>
        {!inProgress && prev && (
          <Delta now={net} before={prev.income_minor - prev.spent_minor} upIsGood vs={vs} money={cur} />
        )}
      </div>
      <div className="tile">
        <span className="tile-label">Invested</span>
        <span className="tile-value">{formatMoney(summary.invested_minor, cur)}</span>
        {!inProgress && <Delta now={summary.invested_minor} before={prev?.invested_minor} upIsGood vs={vs} />}
      </div>
    </section>
  )
}

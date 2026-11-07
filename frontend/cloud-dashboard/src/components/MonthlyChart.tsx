import { useState } from 'react'
import type { MonthTotal } from '../api/types.ts'
import { colorVar, resolveId, type CategoryIndex } from '../categories.ts'
import { formatMoney, monthLabel } from '../format.ts'
import { useElementWidth } from '../useElementWidth.ts'
import { Legend, Swatch } from './Legend.tsx'

const HEIGHT = 300
const MARGIN = { top: 24, right: 8, bottom: 28, left: 56 }
const MAX_BAR = 24
const RADIUS = 4
const GAP = 2 // surface gap between stacked segments

interface Props {
  months: MonthTotal[]
  selected: string
  currency: string
  categories: CategoryIndex
  onSelect: (month: string) => void
}

/** Clean y-axis ticks: 0 up to the first multiple of a 1/2/2.5/5 step above max. */
function niceTicks(max: number, count = 4): number[] {
  if (max <= 0) return [0]
  const raw = max / count
  const mag = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? raw
  const top = Math.ceil(max / step) * step
  const ticks: number[] = []
  for (let v = 0; v <= top + step / 2; v += step) ticks.push(v)
  return ticks
}

/** Column with rounded data-end, square at the baseline. */
function columnPath(x: number, y: number, w: number, h: number): string {
  const r = Math.min(RADIUS, h, w / 2)
  return `M${x},${y + h}V${y + r}Q${x},${y} ${x + r},${y}H${x + w - r}Q${x + w},${y} ${x + w},${y + r}V${y + h}Z`
}

/** Per-month amounts in legend order, so colors and stacking follow the category, not its rank. */
function stacks(months: MonthTotal[], categories: CategoryIndex): number[][] {
  return months.map((m) => {
    const byId = new Map<string, number>()
    for (const c of m.by_category) {
      const id = resolveId(c.category, 'expense', categories.ids.expense)
      byId.set(id, (byId.get(id) ?? 0) + c.spent_minor)
    }
    return categories.byKind.expense.map((c) => byId.get(c.id) ?? 0)
  })
}

interface Segment {
  index: number // into the category list
  y: number
  h: number
  top: boolean // the topmost segment gets the rounded data-end
}

/** Stacks non-zero values upward from the baseline, leaving a surface gap between segments. */
function segments(values: number[], baseline: number, height: (v: number) => number): Segment[] {
  const out: Segment[] = []
  let cursor = baseline
  values.forEach((v, index) => {
    if (v <= 0) return
    const segTop = cursor - height(v)
    const bottom = out.length === 0 ? cursor : cursor - GAP
    cursor = segTop
    // Segments thinner than the gap vanish; the tooltip and table still carry their values.
    if (bottom - segTop > 0) out.push({ index, y: segTop, h: bottom - segTop, top: false })
  })
  if (out.length) out[out.length - 1].top = true
  return out
}

export function MonthlyChart({ months, selected, currency, categories, onSelect }: Props) {
  const [ref, width] = useElementWidth<HTMLDivElement>()
  const [hover, setHover] = useState<number | null>(null)
  const [asTable, setAsTable] = useState(false)

  const expense = categories.byKind.expense
  const values = stacks(months, categories)
  const ticks = niceTicks(Math.max(...months.map((m) => m.spent_minor), 0))
  const top = ticks.at(-1) || 1
  const plotW = Math.max(width - MARGIN.left - MARGIN.right, 0)
  const plotH = HEIGHT - MARGIN.top - MARGIN.bottom
  const band = plotW / months.length
  const barW = Math.min(MAX_BAR, band * 0.6)
  const y = (v: number) => MARGIN.top + plotH - (v / top) * plotH
  const selectedIdx = months.findIndex((m) => m.month === selected)
  // Short month names need ~32px; below that, label every other month (always the selected one).
  const labelEvery = band < 32 ? 2 : 1
  const showLabel = (i: number) => i === selectedIdx || (i - selectedIdx) % labelEvery === 0

  const fmt = (v: number) => formatMoney(v, currency)
  const tip = hover === null ? null : months[hover]

  return (
    <section className="card">
      <header className="card-head">
        <div>
          <h2>Spending by month</h2>
          <p className="card-sub">Last 12 months by category. Select a month to see its details.</p>
        </div>
        <button type="button" className="ghost" onClick={() => setAsTable((v) => !v)} aria-pressed={asTable}>
          {asTable ? 'Chart' : 'Table'}
        </button>
      </header>

      {asTable ? (
        <div className="table-scroll">
          <table className="data-table compact">
            <thead>
              <tr>
                <th scope="col">Month</th>
                {expense.map((c) => (
                  <th key={c.id} scope="col" className="num">
                    {c.name}
                  </th>
                ))}
                <th scope="col" className="num">
                  Total
                </th>
              </tr>
            </thead>
            <tbody>
              {months.map((m, i) => (
                <tr key={m.month} className={m.month === selected ? 'is-selected' : undefined}>
                  <td className="nowrap">{monthLabel(m.month)}</td>
                  {values[i].map((v, j) => (
                    <td key={expense[j].id} className="num">
                      {v ? fmt(v) : '–'}
                    </td>
                  ))}
                  <td className="num">{fmt(m.spent_minor)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <>
          <Legend categories={expense} />
          <div className="chart" ref={ref}>
            {width > 0 && (
              <svg width={width} height={HEIGHT} role="group" aria-label="Spending per month by category">
                {ticks.map((t) => (
                  <g key={t}>
                    <line className="grid" x1={MARGIN.left} x2={width - MARGIN.right} y1={y(t)} y2={y(t)} />
                    <text className="axis" x={MARGIN.left - 8} y={y(t)} dy="0.32em" textAnchor="end">
                      {formatMoney(t, currency, { compact: true })}
                    </text>
                  </g>
                ))}

                {months.map((m, i) => {
                  const cx = MARGIN.left + band * i + band / 2
                  const isSel = i === selectedIdx
                  const x = cx - barW / 2
                  const segs = segments(values[i], y(0), (v) => (v / top) * plotH)
                  return (
                    <g
                      key={m.month}
                      className={`col ${isSel ? 'is-selected' : ''} ${hover === i ? 'is-hover' : ''}`}
                      role="button"
                      tabIndex={0}
                      aria-pressed={isSel}
                      aria-label={`${monthLabel(m.month)}: spent ${fmt(m.spent_minor)}`}
                      onClick={() => onSelect(m.month)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault()
                          onSelect(m.month)
                        }
                      }}
                      onMouseEnter={() => setHover(i)}
                      onMouseLeave={() => setHover(null)}
                      onFocus={() => setHover(i)}
                      onBlur={() => setHover(null)}
                    >
                      {/* Hit target and selection band: the whole column slot. */}
                      <rect className="hit" x={cx - band / 2} y={MARGIN.top} width={band} height={plotH} />
                      {segs.map((sg) => {
                        const c = expense[sg.index]
                        const fill = colorVar(c.color)
                        return sg.top ? (
                          <path key={c.id} d={columnPath(x, sg.y, barW, sg.h)} style={{ fill }} />
                        ) : (
                          <rect key={c.id} x={x} y={sg.y} width={barW} height={sg.h} style={{ fill }} />
                        )
                      })}
                      {isSel && (
                        <text className="cap-label" x={cx} y={y(m.spent_minor) - 6} textAnchor="middle">
                          {formatMoney(m.spent_minor, currency, { compact: true })}
                        </text>
                      )}
                      {showLabel(i) && (
                        <text className="axis month-label" x={cx} y={HEIGHT - 8} textAnchor="middle">
                          {monthLabel(m.month, 'short')}
                        </text>
                      )}
                    </g>
                  )
                })}
                <line className="baseline" x1={MARGIN.left} x2={width - MARGIN.right} y1={y(0)} y2={y(0)} />
              </svg>
            )}

            {tip && hover !== null && (
              <div
                className={`tooltip ${hover > months.length / 2 ? 'tooltip-left' : 'tooltip-right'}`}
                style={{ left: MARGIN.left + band * hover + band / 2, top: MARGIN.top }}
                aria-hidden="true"
              >
                <strong>{monthLabel(tip.month)}</strong>
                {/* Top of the stack first, matching what the eye sees. */}
                {values[hover]
                  .map((v, j) => ({ v, c: expense[j] }))
                  .filter((s) => s.v > 0)
                  .reverse()
                  .map((s) => (
                    <span key={s.c.id} className="tip-row">
                      <Swatch color={s.c.color} />
                      <span>{s.c.name}</span>
                      <span className="num">{fmt(s.v)}</span>
                    </span>
                  ))}
                <span className="tip-row tip-total">
                  <span />
                  <span>Total spent</span>
                  <span className="num">{fmt(tip.spent_minor)}</span>
                </span>
                <span className="tip-row">
                  <span />
                  <span>Income</span>
                  <span className="num">{fmt(tip.income_minor)}</span>
                </span>
              </div>
            )}
          </div>
        </>
      )}
    </section>
  )
}

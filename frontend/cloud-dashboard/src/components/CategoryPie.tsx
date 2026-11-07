import { useState } from 'react'
import type { CategoryTotal } from '../api/types.ts'
import { colorVar, resolveId, type CategoryIndex } from '../categories.ts'
import { formatMoney, formatPercent } from '../format.ts'
import { Swatch } from './Legend.tsx'

const SIZE = 200
const OUTER = 96
const INNER = 62
const GAP = 2 // surface gap between slices, in px at the outer edge

interface Props {
  totals: CategoryTotal[]
  total: number
  currency: string
  categories: CategoryIndex
  active: string | null
  onToggle: (category: string) => void
}

function arcPath(r0: number, r1: number, a0: number, a1: number): string {
  const c = SIZE / 2
  const p = (r: number, a: number) => `${c + r * Math.cos(a)},${c + r * Math.sin(a)}`
  const large = a1 - a0 > Math.PI ? 1 : 0
  return `M${p(r1, a0)}A${r1},${r1} 0 ${large} 1 ${p(r1, a1)}L${p(r0, a1)}A${r0},${r0} 0 ${large} 0 ${p(r0, a0)}Z`
}

/** One donut path per value, clockwise from 12 o'clock, with a surface gap between slices. */
function slicePaths(values: number[], total: number): string[] {
  const pad = values.length > 1 ? GAP / OUTER : 0
  let angle = -Math.PI / 2
  return values.map((v) => {
    const span = (v / total) * Math.PI * 2
    const a0 = angle + pad / 2
    // A single full-circle arc has identical endpoints and would draw nothing.
    const a1 = angle + Math.min(span, Math.PI * 2 - 0.0001) - pad / 2
    angle += span
    return arcPath(INNER, OUTER, a0, Math.max(a1, a0))
  })
}

export function CategoryPie({ totals, total, currency, categories, active, onToggle }: Props) {
  const [hover, setHover] = useState<string | null>(null)

  // Legend order, not size order: a category keeps its place and color from month to month.
  const byId = new Map<string, CategoryTotal>()
  for (const t of totals) {
    const id = resolveId(t.category, 'expense', categories.ids.expense)
    const prev = byId.get(id)
    byId.set(id, {
      category: id,
      spent_minor: (prev?.spent_minor ?? 0) + t.spent_minor,
      count: (prev?.count ?? 0) + t.count,
    })
  }
  const slices = categories.byKind.expense.flatMap((c) => {
    const t = byId.get(c.id)
    return t && t.spent_minor > 0 ? [{ c, t }] : []
  })

  const paths = slicePaths(
    slices.map((s) => s.t.spent_minor),
    total,
  )
  const arcs = slices.map((s, i) => ({ ...s, d: paths[i] }))

  const focus = hover ? slices.find((s) => s.c.id === hover) : undefined
  const fmt = (v: number) => formatMoney(v, currency)

  return (
    <section className="card">
      <header className="card-head">
        <div>
          <h2>Where it went</h2>
          <p className="card-sub">Share of this month's spending. Select a category to filter.</p>
        </div>
      </header>

      {slices.length === 0 ? (
        <p className="empty">No spending this month.</p>
      ) : (
        <div className="pie-wrap">
          <div className="pie">
            <svg
              width={SIZE}
              height={SIZE}
              role="img"
              aria-label={`Spending split: ${slices.map((s) => `${s.c.name} ${formatPercent(s.t.spent_minor / total)}`).join(', ')}`}
            >
              {arcs.map((a) => (
                <path
                  key={a.c.id}
                  d={a.d}
                  className={`slice ${active && active !== a.c.id ? 'is-dim' : ''}`}
                  style={{ fill: colorVar(a.c.color) }}
                  onMouseEnter={() => setHover(a.c.id)}
                  onMouseLeave={() => setHover(null)}
                  onClick={() => onToggle(a.c.id)}
                />
              ))}
            </svg>
            <div className="pie-center" aria-hidden="true">
              <span className="pie-center-label">{focus ? focus.c.name : 'Spent'}</span>
              <span className="pie-center-value">
                {formatMoney(focus ? focus.t.spent_minor : total, currency, { compact: true })}
              </span>
              {focus && <span className="pie-center-label">{formatPercent(focus.t.spent_minor / total)}</span>}
            </div>
          </div>

          <ul className="pie-legend">
            {slices.map(({ c, t }) => (
              <li key={c.id}>
                <button
                  type="button"
                  className={`pie-row ${active === c.id ? 'is-active' : ''} ${hover === c.id ? 'is-hover' : ''}`}
                  aria-pressed={active === c.id}
                  onClick={() => onToggle(c.id)}
                  onMouseEnter={() => setHover(c.id)}
                  onMouseLeave={() => setHover(null)}
                  onFocus={() => setHover(c.id)}
                  onBlur={() => setHover(null)}
                >
                  <Swatch color={c.color} />
                  <span className="pie-name">{c.name}</span>
                  <span className="num">{fmt(t.spent_minor)}</span>
                  <span className="num muted pie-share">{formatPercent(t.spent_minor / total)}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  )
}

// Calendar math for recurring rules. Dates are local calendar days ("YYYY-MM-DD"), not instants: a
// bill due on the 1st is due on the 1st wherever you are. cloud-api must implement the same rules.

import type { RecurringInput, RepeatUnit } from './api/types.ts'

type Rule = Pick<RecurringInput, 'start_date' | 'end_date' | 'every' | 'unit'>

const pad = (n: number) => String(n).padStart(2, '0')

export function toDay(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function parts(day: string): [number, number, number] {
  const [y, m, d] = day.split('-').map(Number)
  return [y, m, d]
}

/** n months after `day`, keeping its day of month but never past the end of a short month. */
function addMonths(day: string, n: number): string {
  const [y, m, d] = parts(day)
  const first = new Date(y, m - 1 + n, 1)
  const last = new Date(first.getFullYear(), first.getMonth() + 1, 0).getDate()
  return toDay(new Date(first.getFullYear(), first.getMonth(), Math.min(d, last)))
}

function addDays(day: string, n: number): string {
  const [y, m, d] = parts(day)
  return toDay(new Date(y, m - 1, d + n))
}

/** The i-th occurrence (0 = start date). Always computed from the start, so clamping never drifts. */
function nth(rule: Rule, i: number): string {
  const k = rule.every * i
  if (rule.unit === 'week') return addDays(rule.start_date, 7 * k)
  return addMonths(rule.start_date, rule.unit === 'year' ? 12 * k : k)
}

/** Occurrences within [from, to], inclusive. Upper bound keeps a bad rule from looping forever. */
export function occurrences(rule: Rule, from: string, to: string, max = 1000): string[] {
  const out: string[] = []
  const last = rule.end_date && rule.end_date < to ? rule.end_date : to
  for (let i = 0; i < max; i++) {
    const day = nth(rule, i)
    if (day > last) break
    if (day >= from) out.push(day)
  }
  return out
}

/** First occurrence strictly after `after`, or undefined if the rule has ended by then. */
export function nextOccurrence(rule: Rule, after: string): string | undefined {
  for (let i = 0; i < 5000; i++) {
    const day = nth(rule, i)
    if (rule.end_date && day > rule.end_date) return undefined
    if (day > after) return day
  }
  return undefined
}

const UNIT_WORD: Record<RepeatUnit, [string, string]> = {
  week: ['week', 'weeks'],
  month: ['month', 'months'],
  year: ['year', 'years'],
}

/** "Monthly on day 1", "Every 3 months on day 15", "Every 2 weeks on Monday", "Yearly on 3 Mar". */
export function describe(rule: Rule): string {
  const [y, m, d] = parts(rule.start_date)
  const date = new Date(y, m - 1, d)
  const when =
    rule.unit === 'week'
      ? `on ${new Intl.DateTimeFormat(undefined, { weekday: 'long' }).format(date)}`
      : rule.unit === 'month'
        ? `on day ${d}`
        : `on ${new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short' }).format(date)}`
  const every =
    rule.every === 1
      ? { week: 'Weekly', month: 'Monthly', year: 'Yearly' }[rule.unit]
      : `Every ${rule.every} ${UNIT_WORD[rule.unit][1]}`
  return `${every} ${when}`
}

export function dayLabel(day: string): string {
  const [y, m, d] = parts(day)
  return new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' }).format(
    new Date(y, m - 1, d),
  )
}

/** Noon local time on that day, as an instant: no time zone can move it to a neighbouring day. */
export function dayToInstant(day: string): string {
  const [y, m, d] = parts(day)
  return new Date(y, m - 1, d, 12).toISOString()
}

// Months are "YYYY-MM" strings: they sort correctly as text and match the API's query parameter.

export function currentMonth(now = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

function toDate(month: string): Date {
  const [y, m] = month.split('-').map(Number)
  return new Date(y, m - 1, 1)
}

export function addMonths(month: string, n: number): string {
  const d = toDate(month)
  d.setMonth(d.getMonth() + n)
  return currentMonth(d)
}

/** Inclusive list of months from `from` to `to`. */
export function monthRange(from: string, to: string): string[] {
  const out: string[] = []
  for (let m = from; m <= to; m = addMonths(m, 1)) out.push(m)
  return out
}

export function monthLabel(month: string, style: 'long' | 'short' = 'long'): string {
  const opts: Intl.DateTimeFormatOptions =
    style === 'long' ? { month: 'long', year: 'numeric' } : { month: 'short' }
  return new Intl.DateTimeFormat(undefined, opts).format(toDate(month))
}

export function dateLabel(iso: string): string {
  return new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short' }).format(new Date(iso))
}

const minorDigits = new Map<string, number>()

// ISO 4217 minor units (EUR 2, JPY 0) as Intl knows them.
function digits(currency: string): number {
  let d = minorDigits.get(currency)
  if (d === undefined) {
    d = new Intl.NumberFormat('en', { style: 'currency', currency }).resolvedOptions().maximumFractionDigits ?? 2
    minorDigits.set(currency, d)
  }
  return d
}

export function formatMoney(
  minor: number,
  currency: string,
  opts: { compact?: boolean; signed?: boolean } = {},
): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    notation: opts.compact ? 'compact' : 'standard',
    maximumFractionDigits: opts.compact ? 1 : undefined,
    signDisplay: opts.signed ? 'exceptZero' : 'auto',
  }).format(minor / 10 ** digits(currency))
}

export function formatPercent(ratio: number): string {
  return new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0 }).format(ratio)
}

/**
 * Parses what a person types into an amount field: "12", "12.5", "12,50", "1 234,50". Returns
 * positive minor units, or null if it isn't a valid amount for the currency.
 */
export function parseAmount(text: string, currency: string): number | null {
  const s = text.replace(/[\s']/g, '') // \s includes no-break spaces
  const m = /^(\d+)(?:[.,](\d+))?$/.exec(s)
  if (!m) return null
  const d = digits(currency)
  const frac = m[2] ?? ''
  if (frac.length > d) return null
  const minor = Number(m[1]) * 10 ** d + Number(frac.padEnd(d, '0') || 0)
  return Number.isSafeInteger(minor) && minor > 0 ? minor : null
}

/** Minor units back to an editable string, e.g. 1250 EUR -> "12.50". */
export function amountInput(minor: number, currency: string): string {
  return (Math.abs(minor) / 10 ** digits(currency)).toFixed(digits(currency))
}

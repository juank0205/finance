// Deterministic demo data: the same month always produces the same transactions, so reloads are
// stable. Manual entries and edited categories are kept in this browser's localStorage. Only loaded
// when VITE_API_URL is unset.

import { CATCH_ALL, DEFAULT_CATEGORIES, KINDS, MAX_CATEGORIES, kindOf, resolveId } from '../categories.ts'
import { addMonths, currentMonth, monthRange } from '../format.ts'
import { dayToInstant, occurrences, toDay } from '../recurrence.ts'
import type {
  Api,
  Category,
  CategoryTotal,
  NetWorth,
  Recurring,
  RecurringInput,
  Source,
  Summary,
  Transaction,
  TransactionInput,
  TransactionKind,
} from './types.ts'

const CURRENCY = 'EUR'
const CARD = '4821'
const LATENCY_MS = 250
// Stand in for cloud-api's tables. Per browser only.
const CATEGORIES_KEY = 'finance.demo.categories.v2'
// Manual entries and created recurring occurrences: everything a person (or a rule) wrote.
const MANUAL_KEY = 'finance.demo.manual'
const RECURRING_KEY = 'finance.demo.recurring'

interface Variable {
  category: string
  merchants: string[]
  perMonth: [number, number]
  amount: [number, number] // major units, money out
}

interface Fixed {
  kind: TransactionKind
  category: string
  merchant: string
  day: number
  amount: [number, number] // major units, always positive; the kind sets the sign
  source: Source
}

const VARIABLE: Variable[] = [
  { category: 'groceries', merchants: ['Supermarket Central', 'Fresh Market', 'Corner Grocer'], perMonth: [8, 12], amount: [12, 90] },
  { category: 'restaurants', merchants: ['Trattoria Nove', 'Noodle Bar', 'Café du Coin', 'Burger Lab'], perMonth: [4, 10], amount: [11, 65] },
  { category: 'clothes', merchants: ['Department Store', 'Denim Co', 'Shoe Corner'], perMonth: [0, 3], amount: [18, 140] },
  { category: 'other', merchants: ['City Metro', 'RideNow', 'Pharmacy Plus', 'Cinema Lumière', 'Bookshop'], perMonth: [5, 10], amount: [2, 50] },
]

// Imported every month by the (future) statement upload. The amount varies, so it isn't a rule.
const FIXED: Fixed[] = [
  { kind: 'expense', category: 'bills', merchant: 'Power & Water Co', day: 10, amount: [60, 110], source: 'statement' },
]

/** Demo rules, seeded once per browser; they start a year back so the charts have history. */
function seedRules(): StoredRule[] {
  const now = new Date()
  const yearAgo = (day: number) => toDay(new Date(now.getFullYear() - 1, now.getMonth(), day))
  const rule = (
    id: string,
    kind: TransactionKind,
    category: string,
    merchant: string,
    major: number,
    day: number,
    note?: string,
  ): StoredRule => ({
    id,
    kind,
    amount_minor: Math.round(major * 100),
    currency: CURRENCY,
    merchant,
    category,
    note,
    start_date: yearAgo(day),
    every: 1,
    unit: 'month',
    skip: [],
  })
  return [
    rule('r-mortgage', 'expense', 'mortgage', 'Home loan', 1150, 1, 'Direct debit, fixed rate until 2031'),
    rule('r-broker', 'investment', 'index-funds', 'Broker monthly plan', 300, 2),
    rule('r-gym', 'expense', 'subscriptions', 'Gym Club', 35, 3),
    rule('r-streambox', 'expense', 'subscriptions', 'StreamBox', 12.99, 5),
    rule('r-music', 'expense', 'subscriptions', 'Music+', 10.99, 12),
    rule('r-phone', 'expense', 'bills', 'Mobile Telecom', 25, 15),
    rule('r-internet', 'expense', 'bills', 'Fibre Internet', 35, 20),
    rule('r-salary', 'income', 'salary', 'Employer payroll', 3400, 28),
  ]
}

const signed = (kind: TransactionKind, positiveMinor: number) => (kind === 'income' ? positiveMinor : -positiveMinor)

// mulberry32: tiny seeded PRNG, good enough for fake data.
function rng(seed: number) {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function hash(s: string): number {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 16777619)
  return h >>> 0
}

const generated = new Map<string, Transaction[]>()

function generate(month: string): Transaction[] {
  const cached = generated.get(month)
  if (cached) return cached

  const now = new Date()
  const thisMonth = currentMonth(now)
  if (month > thisMonth) return []

  const [y, m] = month.split('-').map(Number)
  const daysInMonth = new Date(y, m, 0).getDate()
  const lastDay = month === thisMonth ? now.getDate() : daysInMonth
  const rand = rng(hash(month))
  const between = (lo: number, hi: number) => lo + rand() * (hi - lo)
  const pick = <T,>(xs: T[]) => xs[Math.floor(rand() * xs.length)]

  const out: Transaction[] = []
  const add = (kind: TransactionKind, day: number, major: number, merchant: string, category: string, source: Source) => {
    const hour = Math.floor(between(8, 22))
    const minute = Math.floor(between(0, 60))
    const id = `${month}-${String(out.length).padStart(3, '0')}`
    out.push({
      id,
      kind,
      occurred_at: new Date(y, m - 1, day, hour, minute).toISOString(),
      amount_minor: signed(kind, Math.round(major * 100)),
      currency: CURRENCY,
      merchant,
      category,
      card_last4: source === 'google_wallet' ? CARD : undefined,
      source,
      dedup_key: `demo-${id}`,
    })
  }

  for (const f of FIXED) {
    if (f.day <= lastDay) add(f.kind, f.day, between(...f.amount), f.merchant, f.category, f.source)
  }
  for (const v of VARIABLE) {
    const n = Math.round(between(...v.perMonth) * (lastDay / daysInMonth))
    for (let i = 0; i < n; i++) {
      // Never 'manual': in demo mode only entries added through the form are manual, and editable.
      const source: Source = rand() < 0.9 ? 'google_wallet' : 'statement'
      add('expense', 1 + Math.floor(rand() * lastDay), between(...v.amount), pick(v.merchants), v.category, source)
    }
  }

  generated.set(month, out)
  return out
}

// --- Browser storage ----------------------------------------------------------------------------

function read<T>(key: string): T | undefined {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : undefined
  } catch {
    return undefined // blocked or corrupted: behave as empty
  }
}

function write(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    throw new Error('Could not save: browser storage is blocked')
  }
}

const loadManual = () => read<Transaction[]>(MANUAL_KEY) ?? []

/** Server-side state of a rule: the dates whose occurrence was deleted, so they never come back. */
type StoredRule = Recurring & { skip: string[] }

function loadRules(): StoredRule[] {
  const stored = read<StoredRule[]>(RECURRING_KEY)
  if (stored) return stored
  const seeded = seedRules()
  write(RECURRING_KEY, seeded)
  return seeded
}

function publicRule(r: StoredRule): Recurring {
  const { skip, ...rest } = r
  void skip
  return rest
}

const dedupFor = (ruleId: string, day: string) => `recurring:${ruleId}:${day}`

/**
 * What cloud-api's daily job does: create every due occurrence that doesn't exist yet. The dedup key
 * makes it idempotent, so running it on every read here is harmless.
 */
function materialize() {
  const today = toDay(new Date())
  const entries = loadManual()
  const existing = new Set(entries.map((t) => t.dedup_key))
  let added = 0
  for (const r of loadRules()) {
    const skip = new Set(r.skip)
    for (const day of occurrences(r, r.start_date, today)) {
      const dedup = dedupFor(r.id, day)
      if (existing.has(dedup) || skip.has(day)) continue
      entries.push({
        id: `o-${r.id}-${day}`,
        kind: r.kind,
        occurred_at: dayToInstant(day),
        amount_minor: signed(r.kind, r.amount_minor),
        currency: r.currency,
        merchant: r.merchant,
        category: r.category,
        note: r.note,
        source: 'recurring',
        dedup_key: dedup,
      })
      existing.add(dedup)
      added++
    }
  }
  if (added) write(MANUAL_KEY, entries)
}

function loadCategories(): Category[] {
  const stored = read<Category[]>(CATEGORIES_KEY)
  if (stored) {
    try {
      return validateCategories(stored)
    } catch {
      // Stored list from an older version: fall back to the defaults.
    }
  }
  return DEFAULT_CATEGORIES
}

function transactionsFor(month: string): Transaction[] {
  materialize()
  const manual = loadManual().filter((t) => currentMonth(new Date(t.occurred_at)) === month)
  return [...generate(month), ...manual].sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))
}

// --- Validation: the checks cloud-api will have to do too ---------------------------------------

function validateCategories(list: Category[]): Category[] {
  if (!Array.isArray(list)) throw new Error('Expected a list of categories')
  for (const kind of KINDS) {
    const ofKind = list.filter((c) => c.kind === kind)
    if (ofKind.length > MAX_CATEGORIES) throw new Error(`At most ${MAX_CATEGORIES} categories per type`)
    if (!ofKind.some((c) => c.id === CATCH_ALL[kind])) throw new Error('Catch-all categories cannot be deleted')
    const names = new Set<string>()
    const colors = new Set<number>()
    for (const c of ofKind) {
      const name = c.name?.trim() ?? ''
      if (!c.id || !name || name.length > 30) throw new Error('Every category needs a name of up to 30 characters')
      if (names.has(name.toLowerCase())) throw new Error(`Duplicate name: ${name}`)
      if (!Number.isInteger(c.color) || c.color < 1 || c.color > MAX_CATEGORIES || colors.has(c.color)) {
        throw new Error('Each category needs its own color')
      }
      names.add(name.toLowerCase())
      colors.add(c.color)
    }
  }
  if (list.some((c) => !KINDS.includes(c.kind))) throw new Error('Unknown category type')
  return list.map((c) => ({ id: c.id, kind: c.kind, name: c.name.trim(), color: c.color }))
}

function validateInput(input: TransactionInput): TransactionInput {
  if (!KINDS.includes(input.kind)) throw new Error('Unknown type')
  if (!Number.isInteger(input.amount_minor) || input.amount_minor <= 0 || input.amount_minor > 100_000_000_00) {
    throw new Error('Amount must be more than 0')
  }
  if (!/^[A-Z]{3}$/.test(input.currency)) throw new Error('Unknown currency')
  const when = new Date(input.occurred_at)
  const endOfToday = new Date()
  endOfToday.setHours(23, 59, 59, 999)
  if (Number.isNaN(when.getTime())) throw new Error('Invalid date')
  if (when > endOfToday) throw new Error('The date cannot be in the future')
  const merchant = input.merchant.trim()
  if (!merchant || merchant.length > 80) throw new Error('Description must be 1 to 80 characters')
  const note = input.note?.trim() || undefined
  if (note && note.length > 500) throw new Error('Note must be at most 500 characters')
  const category = loadCategories().find((c) => c.id === input.category)
  if (!category || category.kind !== input.kind) throw new Error('Pick a category of the same type')
  return { ...input, merchant, note }
}

function validateRule(input: RecurringInput): RecurringInput {
  // Same field rules as a single entry; the date checks differ.
  const base = validateInput({ ...input, occurred_at: new Date().toISOString() })
  const day = /^\d{4}-\d{2}-\d{2}$/
  if (!day.test(input.start_date)) throw new Error('Invalid start date')
  if (input.end_date && (!day.test(input.end_date) || input.end_date < input.start_date)) {
    throw new Error('The end date must be on or after the start date')
  }
  if (!Number.isInteger(input.every) || input.every < 1 || input.every > 12) throw new Error('Repeat every 1 to 12')
  if (!['week', 'month', 'year'].includes(input.unit)) throw new Error('Unknown repeat unit')
  return {
    kind: base.kind,
    amount_minor: base.amount_minor,
    currency: base.currency,
    merchant: base.merchant,
    category: base.category,
    note: base.note,
    start_date: input.start_date,
    end_date: input.end_date || undefined,
    every: input.every,
    unit: input.unit,
  }
}

function toTransaction(id: string, input: TransactionInput): Transaction {
  return {
    id,
    kind: input.kind,
    occurred_at: input.occurred_at,
    amount_minor: signed(input.kind, input.amount_minor),
    currency: input.currency,
    merchant: input.merchant,
    category: input.category,
    note: input.note,
    source: 'manual',
    dedup_key: `manual-${id}`,
  }
}

// --- Aggregates ----------------------------------------------------------------------------------

function totals(txs: Transaction[], expenseIds: ReadonlySet<string>) {
  let spent = 0
  let income = 0
  let invested = 0
  const byCategory = new Map<string, CategoryTotal>()
  for (const t of txs) {
    const kind = kindOf(t)
    const abs = Math.abs(t.amount_minor)
    if (kind === 'income') income += abs
    else if (kind === 'investment') invested += abs
    else {
      spent += abs
      const id = resolveId(t.category, 'expense', expenseIds)
      const c = byCategory.get(id) ?? { category: id, spent_minor: 0, count: 0 }
      c.spent_minor += abs
      c.count++
      byCategory.set(id, c)
    }
  }
  return {
    spent,
    income,
    invested,
    byCategory: [...byCategory.values()].sort((a, b) => b.spent_minor - a.spent_minor),
  }
}

function summarize(month: string): Summary {
  const expenseIds = new Set(loadCategories().filter((c) => c.kind === 'expense').map((c) => c.id))
  const txs = transactionsFor(month)
  const t = totals(txs, expenseIds)

  return {
    month,
    currency: CURRENCY,
    spent_minor: t.spent,
    income_minor: t.income,
    invested_minor: t.invested,
    count: txs.length,
    by_category: t.byCategory,
    months: monthRange(addMonths(month, -11), month).map((mo) => {
      const mt = totals(transactionsFor(mo), expenseIds)
      return {
        month: mo,
        spent_minor: mt.spent,
        income_minor: mt.income,
        invested_minor: mt.invested,
        by_category: mt.byCategory,
      }
    }),
  }
}

function netWorth(): NetWorth {
  return {
    as_of: new Date().toISOString(),
    currency: CURRENCY,
    holdings: [
      { id: 'checking', name: 'Checking account', kind: 'cash', balance_minor: 324_015 },
      { id: 'savings', name: 'Savings account', kind: 'cash', balance_minor: 1_250_000 },
      { id: 'etf', name: 'Index funds', kind: 'investment', balance_minor: 1_834_270 },
      { id: 'home', name: 'Home (estimated value)', kind: 'property', balance_minor: 24_000_000 },
      { id: 'mortgage', name: 'Mortgage', kind: 'liability', balance_minor: -17_840_000 },
    ],
  }
}

function delay<T>(value: () => T, signal?: AbortSignal): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      try {
        resolve(value())
      } catch (err) {
        reject(err)
      }
    }, LATENCY_MS)
    signal?.addEventListener('abort', () => {
      clearTimeout(timer)
      reject(signal.reason)
    })
  })
}

function manualEntry(id: string): { list: Transaction[]; index: number } {
  const list = loadManual()
  const index = list.findIndex((t) => t.id === id)
  if (index < 0) throw new Error('Only manual and recurring entries can be changed')
  return { list, index }
}

function ruleIndex(rules: StoredRule[], id: string): number {
  const i = rules.findIndex((r) => r.id === id)
  if (i < 0) throw new Error('Recurring entry not found')
  return i
}

export const mockApi: Api = {
  summary: (month, signal) => delay(() => summarize(month), signal),
  transactions: (month, signal) => delay(() => transactionsFor(month), signal),
  createTransaction: (input) =>
    delay(() => {
      const t = toTransaction(`m-${crypto.randomUUID()}`, validateInput(input))
      write(MANUAL_KEY, [...loadManual(), t])
      return t
    }),
  updateTransaction: (id, input) =>
    delay(() => {
      const { list, index } = manualEntry(id)
      const old = list[index]
      // An edited occurrence stays tied to its rule, so the daily job won't create it again.
      const t = { ...toTransaction(id, validateInput(input)), source: old.source, dedup_key: old.dedup_key }
      list[index] = t
      write(MANUAL_KEY, list)
      return t
    }),
  deleteTransaction: (id) =>
    delay(() => {
      const { list, index } = manualEntry(id)
      const [removed] = list.splice(index, 1)
      // A deleted occurrence is a skipped one: remember it, or the next run would bring it back.
      const m = /^recurring:(.+):(\d{4}-\d{2}-\d{2})$/.exec(removed.dedup_key)
      if (m) {
        const rules = loadRules()
        const r = rules.find((x) => x.id === m[1])
        if (r) {
          r.skip = [...r.skip, m[2]]
          write(RECURRING_KEY, rules)
        }
      }
      write(MANUAL_KEY, list)
    }),
  recurring: (signal) =>
    delay(() => loadRules().map(publicRule).sort((a, b) => a.merchant.localeCompare(b.merchant)), signal),
  createRecurring: (input) =>
    delay(() => {
      const r: StoredRule = { id: `r-${crypto.randomUUID().slice(0, 8)}`, ...validateRule(input), skip: [] }
      write(RECURRING_KEY, [...loadRules(), r])
      materialize()
      return publicRule(r)
    }),
  updateRecurring: (id, input) =>
    delay(() => {
      const rules = loadRules()
      const i = ruleIndex(rules, id)
      rules[i] = { ...rules[i], ...validateRule(input) }
      write(RECURRING_KEY, rules)
      materialize()
      return publicRule(rules[i])
    }),
  deleteRecurring: (id) =>
    delay(() => {
      const rules = loadRules()
      rules.splice(ruleIndex(rules, id), 1)
      write(RECURRING_KEY, rules)
    }),
  netWorth: (signal) => delay(netWorth, signal),
  categories: (signal) => delay(loadCategories, signal),
  saveCategories: (list) =>
    delay(() => {
      const clean = validateCategories(list)
      write(CATEGORIES_KEY, clean)
      return clean
    }),
}

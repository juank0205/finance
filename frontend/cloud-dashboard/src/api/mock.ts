// Deterministic demo data: the same month always produces the same transactions, so reloads are
// stable. Manual entries, accounts and edited categories are kept in this browser's localStorage.
// Only loaded when VITE_API_URL is unset. Mirrors backend/internal/ledger, error texts included.

import {
  CATCH_ALL,
  DEFAULT_CATEGORIES,
  KINDS,
  MAX_CATEGORIES,
  isCategoryKind,
  kindOf,
  resolveId,
} from '../categories.ts'
import { addMonths, currentMonth, monthRange } from '../format.ts'
import { dayToInstant, occurrences, toDay } from '../recurrence.ts'
import type {
  Account,
  Api,
  Category,
  CategoryKind,
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
const MAX_ACCOUNTS = 12
const MAX_AMOUNT_MINOR = 100_000_000_00 // 100 million: anything above it is a typo
// Stand in for cloud-api's tables. Per browser only.
const CATEGORIES_KEY = 'finance.demo.categories.v2'
const ACCOUNTS_KEY = 'finance.demo.accounts'
// Manual entries and created recurring occurrences: everything a person (or a rule) wrote.
const MANUAL_KEY = 'finance.demo.manual'
const RECURRING_KEY = 'finance.demo.recurring'

// Fixed demo rates, units per 1 EUR. cloud-api uses each day's reference rates (internal/fx).
const RATES: Record<string, number> = { EUR: 1, USD: 1.1, COP: 3850 }

/** Seeded once per browser, opened a year back. The generated entries below use these ids. */
const DEMO_ACCOUNTS: Omit<Account, 'opening_date'>[] = [
  { id: 'sg', name: 'SG', kind: 'cash', currency: 'EUR', opening_minor: 324_015 },
  { id: 'arq', name: 'ARQ', kind: 'cash', currency: 'USD', opening_minor: 150_000 },
  { id: 'bancolombia', name: 'Bancolombia', kind: 'cash', currency: 'COP', opening_minor: 2_400_000_00 },
  { id: 'visa', name: 'Visa', kind: 'credit', currency: 'COP', opening_minor: -850_000_00 },
]

interface Variable {
  category: string
  merchants: string[]
  perMonth: [number, number]
  amount: [number, number] // major units, money out
}

interface Fixed {
  kind: CategoryKind
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
    kind: CategoryKind,
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

function seedAccounts(): Account[] {
  const now = new Date()
  const opening_date = toDay(new Date(now.getFullYear() - 1, now.getMonth(), 1))
  return DEMO_ACCOUNTS.map((a) => ({ ...a, opening_date }))
}

const signed = (kind: TransactionKind, positiveMinor: number) => (kind === 'income' ? positiveMinor : -positiveMinor)

/** minor units of `from` in `to`, like cloud-api's convert (Go's math.Round: halves away from zero). */
function convert(minor: number, from: string, to: string): number {
  if (from === to || minor === 0) return minor
  const a = RATES[from]
  const b = RATES[to]
  if (!a || !b) throw new Error(`No ${from} to ${to} exchange rate right now; try again later`)
  const x = (minor * b) / a
  return Math.sign(x) * Math.round(Math.abs(x))
}

// The server's JSON omits zero amounts.
const nz = (n: number) => n || undefined

type AccountRef = Pick<Account, 'id' | 'currency'>

/** What cloud-api's build adds: the amount in the summary currency and in each account's, at the day's rate. */
function withAmounts(t: Transaction, from?: AccountRef, to?: AccountRef, toAmount = 0): Transaction {
  const out: Transaction = { ...t, base_minor: nz(convert(t.amount_minor, t.currency, CURRENCY)) }
  if (from) {
    out.account = from.id
    out.account_minor = nz(convert(t.amount_minor, t.currency, from.currency))
  }
  if (to) {
    out.to_account = to.id
    out.to_minor = nz(toAmount || convert(Math.abs(t.amount_minor), t.currency, to.currency))
  }
  return out
}

/** t's amount in the summary currency. Entries from before currencies were converted are all in it. */
function baseAmount(t: Transaction): number {
  return !t.base_minor && t.currency === CURRENCY ? t.amount_minor : (t.base_minor ?? 0)
}

/** What t moved in account a, in a's currency. */
function accountAmount(t: Transaction, a: Account): number {
  return !t.account_minor && t.currency === a.currency ? t.amount_minor : (t.account_minor ?? 0)
}

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

interface Extra {
  currency?: string // default CURRENCY
  account?: string // a DEMO_ACCOUNTS id
  to?: string // transfers: the receiving DEMO_ACCOUNTS id
  refund?: boolean
}

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
  const demoAccount = (id?: string) => DEMO_ACCOUNTS.find((a) => a.id === id)

  const out: Transaction[] = []
  const add = (
    kind: TransactionKind,
    day: number,
    major: number,
    merchant: string,
    category: string,
    source: Source,
    extra: Extra = {},
  ) => {
    const hour = Math.floor(between(8, 22))
    const minute = Math.floor(between(0, 60))
    const id = `${month}-${String(out.length).padStart(3, '0')}`
    const amount = Math.round(major * 100)
    const t: Transaction = {
      id,
      kind,
      occurred_at: new Date(y, m - 1, day, hour, minute).toISOString(),
      amount_minor: extra.refund ? amount : signed(kind, amount),
      currency: extra.currency ?? CURRENCY,
      merchant,
      category: category || undefined,
      card_last4: source === 'google_wallet' ? CARD : undefined,
      refund: extra.refund,
      source,
      dedup_key: `demo-${id}`,
    }
    out.push(withAmounts(t, demoAccount(extra.account), demoAccount(extra.to)))
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

  // Tied to the demo accounts, so their balances move: savings sent to the USD account, the COP credit
  // card paid from Bancolombia, deliveries on that card, and a friend paying their share of a dinner.
  if (lastDay >= 5) add('transfer', 5, 200, 'Savings to ARQ', '', 'statement', { account: 'sg', to: 'arq' })
  if (lastDay >= 15) {
    add('transfer', 15, 150_000, 'Visa payment', '', 'statement', { currency: 'COP', account: 'bancolombia', to: 'visa' })
  }
  const deliveries = Math.round(between(1, 3) * (lastDay / daysInMonth))
  for (let i = 0; i < deliveries; i++) {
    const day = 1 + Math.floor(rand() * lastDay)
    add('expense', day, Math.round(between(35, 120)) * 1000, 'Rappi', 'restaurants', 'statement', {
      currency: 'COP',
      account: 'visa',
    })
  }
  const shareDay = 1 + Math.floor(rand() * lastDay)
  if (rand() < 0.6) {
    add('expense', shareDay, between(12, 40), 'Ana, her share of dinner', 'restaurants', 'statement', {
      account: 'sg',
      refund: true,
    })
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
 * What cloud-api does on the first request of each day: create every due occurrence that doesn't exist yet. The dedup key
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
      // Rules have no account, so their entries move no balance.
      entries.push(
        withAmounts({
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
        }),
      )
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

function loadAccounts(): Account[] {
  const stored = read<Account[]>(ACCOUNTS_KEY)
  if (stored) return stored
  const seeded = seedAccounts()
  write(ACCOUNTS_KEY, seeded)
  return seeded
}

function transactionsFor(month: string): Transaction[] {
  materialize()
  const manual = loadManual().filter((t) => currentMonth(new Date(t.occurred_at)) === month)
  return [...generate(month), ...manual].sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))
}

// --- Validation: the checks cloud-api does too (backend/internal/ledger/validate.go) ------------

// Go counts runes, not UTF-16 units.
const length = (s: string) => [...s].length

function validDay(day: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day)) return false
  const [y, m, d] = day.split('-').map(Number)
  const date = new Date(y, m - 1, d)
  return date.getFullYear() === y && date.getMonth() === m - 1 && date.getDate() === d
}

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
  if (list.some((c) => !isCategoryKind(c.kind))) throw new Error('Unknown category type')
  return list.map((c) => ({ id: c.id, kind: c.kind, name: c.name.trim(), color: c.color }))
}

/** accounts.go: `old` is the saved list, whose currencies are fixed. */
function validateAccounts(list: Account[], old: Account[]): Account[] {
  if (!Array.isArray(list)) throw new Error('Expected a list of accounts')
  if (list.length > MAX_ACCOUNTS) throw new Error(`At most ${MAX_ACCOUNTS} accounts`)
  const ids = new Set<string>()
  const names = new Set<string>()
  return list.map((a) => {
    if (!/^[a-z0-9][a-z0-9-]{0,39}$/.test(a.id ?? '') || ids.has(a.id)) throw new Error('Every account needs its own id')
    const name = a.name?.trim() ?? ''
    if (!name || length(name) > 30) throw new Error('Every account needs a name of up to 30 characters')
    if (names.has(name.toLowerCase())) throw new Error(`Duplicate name: ${name}`)
    if (a.kind !== 'cash' && a.kind !== 'credit') throw new Error('Unknown account type')
    if (!/^[A-Z]{3}$/.test(a.currency ?? '')) throw new Error('Unknown currency')
    // Entries keep their amount in the account's currency, so changing it would garble them.
    const before = old.find((o) => o.id === a.id)
    if (before && before.currency !== a.currency) {
      throw new Error("An account's currency cannot change; add a new account instead")
    }
    if (!Number.isInteger(a.opening_minor) || Math.abs(a.opening_minor) > MAX_AMOUNT_MINOR) {
      throw new Error('Opening balance is too large')
    }
    if (!validDay(a.opening_date ?? '')) throw new Error('Invalid opening date')
    ids.add(a.id)
    names.add(name.toLowerCase())
    const { id, kind, currency, opening_minor, opening_date } = a
    return { id, name, kind, currency, opening_minor, opening_date }
  })
}

/** The fields a single entry and a recurring rule share. */
type EntryFields = Pick<TransactionInput, 'kind' | 'amount_minor' | 'currency' | 'merchant' | 'category' | 'note'>

/** What every entry has, transfers included: amount, currency, text. */
function validateBasics<T extends EntryFields>(f: T): T {
  if (!Number.isInteger(f.amount_minor) || f.amount_minor <= 0 || f.amount_minor > MAX_AMOUNT_MINOR) {
    throw new Error('Amount must be more than 0')
  }
  if (!/^[A-Z]{3}$/.test(f.currency)) throw new Error('Unknown currency')
  const merchant = f.merchant.trim()
  if (!merchant || length(merchant) > 80) throw new Error('Description must be 1 to 80 characters')
  const note = f.note?.trim() || undefined
  if (note && length(note) > 500) throw new Error('Note must be at most 500 characters')
  return { ...f, merchant, note }
}

function validateEntry<T extends EntryFields>(f: T): T {
  if (!isCategoryKind(f.kind)) throw new Error('Unknown type')
  const clean = validateBasics(f)
  const category = loadCategories().find((c) => c.id === clean.category)
  if (!category || category.kind !== clean.kind) throw new Error('Pick a category of the same type')
  return clean
}

/** A checked TransactionInput. */
interface Checked {
  input: TransactionInput
  from?: Account // the account the money left or entered
  to?: Account // transfers only
  toAmount: number // transfers only; 0 = convert at the day's rate
}

function validateInput(input: TransactionInput): Checked {
  let clean: TransactionInput
  if (input.kind === 'transfer') {
    clean = validateBasics({ ...input, category: input.category ?? '' })
    if (clean.category !== '') throw new Error('Transfers have no category')
  } else {
    clean = validateEntry(input)
  }
  const accounts = loadAccounts()
  let from: Account | undefined
  let to: Account | undefined
  if (input.account) {
    from = accounts.find((a) => a.id === input.account)
    if (!from) throw new Error('Unknown account')
  }
  if (input.refund && input.kind !== 'expense') throw new Error('Only expenses can be refunds')
  const toAmount = input.to_amount_minor ?? 0
  if (input.kind === 'transfer') {
    to = accounts.find((a) => a.id === input.to_account)
    if (!from) throw new Error('Pick the account the money left')
    if (!to) throw new Error('Pick the account the money went to')
    if (to.id === from.id) throw new Error('Pick two different accounts')
    if (clean.currency !== from.currency) throw new Error('A transfer is in the currency of the account it leaves')
    if (!Number.isInteger(toAmount) || toAmount < 0 || toAmount > MAX_AMOUNT_MINOR) {
      throw new Error('Invalid amount received')
    }
  } else if (input.to_account || toAmount) {
    throw new Error('Only transfers have a receiving account')
  }
  const when = new Date(input.occurred_at)
  const endOfToday = new Date()
  endOfToday.setHours(23, 59, 59, 999)
  if (Number.isNaN(when.getTime())) throw new Error('Invalid date')
  if (when > endOfToday) throw new Error('The date cannot be in the future')
  return { input: { ...clean, refund: !!input.refund }, from, to, toAmount }
}

function validateRule(input: RecurringInput): RecurringInput {
  // Same field rules as a single entry, transfers excluded; the date checks differ.
  const base = validateEntry(input)
  const now = new Date()
  if (!validDay(input.start_date)) throw new Error('Invalid start date')
  // Keeps the backfill small.
  if (input.start_date < toDay(new Date(now.getFullYear() - 10, now.getMonth(), now.getDate()))) {
    throw new Error('The start date can be at most 10 years ago')
  }
  if (input.end_date && (!validDay(input.end_date) || input.end_date < input.start_date)) {
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

function toTransaction(id: string, { input, from, to, toAmount }: Checked): Transaction {
  const t: Transaction = {
    id,
    kind: input.kind,
    occurred_at: input.occurred_at,
    // A refund stays an expense but brings money in.
    amount_minor: input.refund ? input.amount_minor : signed(input.kind, input.amount_minor),
    currency: input.currency,
    merchant: input.merchant,
    category: input.category || undefined,
    note: input.note,
    refund: input.refund || undefined,
    source: 'manual',
    dedup_key: `manual-${id}`,
  }
  return withAmounts(t, from, to, toAmount)
}

// --- Aggregates ----------------------------------------------------------------------------------

/** In the summary currency. Refunds are positive, so they lower spending; transfers count nowhere. */
function totals(txs: Transaction[], expenseIds: ReadonlySet<string>) {
  let spent = 0
  let income = 0
  let invested = 0
  const byCategory = new Map<string, CategoryTotal>()
  for (const t of txs) {
    const kind = kindOf(t)
    const amount = baseAmount(t)
    if (kind === 'transfer') continue
    if (kind === 'income') income += amount
    else if (kind === 'investment') invested -= amount
    else {
      spent -= amount
      const id = resolveId(t.category, 'expense', expenseIds)
      const c = byCategory.get(id) ?? { category: id, spent_minor: 0, count: 0 }
      c.spent_minor -= amount
      c.count++
      byCategory.set(id, c)
    }
  }
  const byId = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0)
  return {
    spent,
    income,
    invested,
    byCategory: [...byCategory.values()].sort((a, b) => b.spent_minor - a.spent_minor || byId(a.category, b.category)),
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

/** Every account's opening balance plus the entries tied to it from its opening day on, at today's rate. */
function netWorth(): NetWorth {
  const out: NetWorth = { as_of: new Date().toISOString(), currency: CURRENCY, holdings: [] }
  const accounts = loadAccounts()
  if (accounts.length === 0) return out

  const balance = new Map(accounts.map((a) => [a.id, a.opening_minor]))
  const earliest = accounts.reduce((d, a) => (a.opening_date < d ? a.opening_date : d), accounts[0].opening_date)
  const find = (id?: string) => accounts.find((a) => a.id === id)
  const credit = (a: Account, minor: number) => balance.set(a.id, (balance.get(a.id) ?? 0) + minor)
  for (const month of monthRange(earliest.slice(0, 7), currentMonth())) {
    for (const t of transactionsFor(month)) {
      const day = toDay(new Date(t.occurred_at))
      const from = find(t.account)
      if (from && day >= from.opening_date) credit(from, accountAmount(t, from))
      const to = find(t.to_account)
      if (to && day >= to.opening_date) credit(to, t.to_minor ?? 0)
    }
  }

  out.holdings = accounts.map((a) => {
    const balance_minor = balance.get(a.id) ?? 0
    return {
      id: a.id,
      name: a.name,
      kind: a.kind === 'credit' ? 'liability' : 'cash',
      currency: a.currency,
      balance_minor,
      base_minor: convert(balance_minor, a.currency, CURRENCY),
    }
  })
  return out
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
      // An edited occurrence stays tied to its rule, so it is never created again.
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
  accounts: (signal) => delay(loadAccounts, signal),
  saveAccounts: (list) =>
    delay(() => {
      const clean = validateAccounts(list, loadAccounts())
      write(ACCOUNTS_KEY, clean)
      return clean
    }),
}

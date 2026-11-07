// Shapes returned by cloud-api (backend/cmd/cloud-api). Transaction mirrors
// backend/internal/transaction; keep the two in sync.

export type Source = 'google_wallet' | 'manual' | 'statement' | 'recurring'

/**
 * expense: money spent. income: money in. investment: money moved from cash into investments; it
 * leaves the cash balance but is not spending, so it is never counted in "spent".
 */
export type TransactionKind = 'expense' | 'income' | 'investment'

export interface Transaction {
  id: string
  kind: TransactionKind
  occurred_at: string // RFC 3339
  amount_minor: number // integer minor units (cents); negative = money out of cash
  currency: string // ISO 4217
  merchant: string // who was paid, or where the money came from
  category?: string // Category.id; unknown or missing ids count as the kind's catch-all
  note?: string
  card_last4?: string
  source: Source
  dedup_key: string
}

/** Body of POST /v1/transactions and PUT /v1/transactions/{id}. The server sets the sign from `kind`. */
export interface TransactionInput {
  kind: TransactionKind
  amount_minor: number // always positive
  currency: string
  occurred_at: string // RFC 3339; never in the future
  merchant: string // 1-80 characters
  category: string // Category.id of the same kind
  note?: string // up to 500 characters
}

export type RepeatUnit = 'week' | 'month' | 'year'

/**
 * Body of POST /v1/recurring and PUT /v1/recurring/{id}. Each occurrence becomes a normal transaction
 * with source "recurring" and dedup_key "recurring:<rule id>:<YYYY-MM-DD>", so re-running the daily
 * job never duplicates. Editing a rule only affects occurrences not created yet.
 */
export interface RecurringInput {
  kind: TransactionKind
  amount_minor: number // always positive
  currency: string
  merchant: string
  category: string
  note?: string
  start_date: string // YYYY-MM-DD, local calendar day; may be in the past (past occurrences are created)
  end_date?: string // YYYY-MM-DD, inclusive
  every: number // 1-12
  unit: RepeatUnit
}

export interface Recurring extends RecurringInput {
  id: string
}

/** A user-configurable category. Each kind has its own list and its own undeletable catch-all. */
export interface Category {
  id: string // stable: renaming a category keeps its transactions
  kind: TransactionKind
  name: string
  color: number // categorical palette slot, 1-8, unique within its kind. Follows the category, never its rank.
}

export interface CategoryTotal {
  category: string // Category.id
  spent_minor: number // positive
  count: number
}

export interface MonthTotal {
  month: string // YYYY-MM
  spent_minor: number // positive
  income_minor: number
  invested_minor: number // positive
  by_category: CategoryTotal[] // expenses only
}

export interface Summary {
  month: string
  currency: string
  spent_minor: number
  income_minor: number
  invested_minor: number
  count: number
  by_category: CategoryTotal[] // expenses only
  months: MonthTotal[] // the 12 months ending at `month`, oldest first
}

export type HoldingKind = 'cash' | 'investment' | 'property' | 'liability'

export interface Holding {
  id: string
  name: string
  kind: HoldingKind
  balance_minor: number // liabilities are negative
}

export interface NetWorth {
  as_of: string // RFC 3339
  currency: string
  holdings: Holding[]
}

export interface Api {
  summary(month: string, signal?: AbortSignal): Promise<Summary>
  transactions(month: string, signal?: AbortSignal): Promise<Transaction[]>
  createTransaction(input: TransactionInput): Promise<Transaction>
  updateTransaction(id: string, input: TransactionInput): Promise<Transaction> // manual and recurring only
  deleteTransaction(id: string): Promise<void> // manual and recurring only; a recurring one is skipped for good
  recurring(signal?: AbortSignal): Promise<Recurring[]>
  createRecurring(input: RecurringInput): Promise<Recurring>
  updateRecurring(id: string, input: RecurringInput): Promise<Recurring>
  deleteRecurring(id: string): Promise<void> // keeps the entries already created
  netWorth(signal?: AbortSignal): Promise<NetWorth>
  categories(signal?: AbortSignal): Promise<Category[]>
  saveCategories(categories: Category[]): Promise<Category[]>
}

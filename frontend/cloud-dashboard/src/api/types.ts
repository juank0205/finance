// Shapes returned by cloud-api (backend/cmd/cloud-api). Transaction mirrors
// backend/internal/transaction; keep the two in sync.

export type Source = 'google_wallet' | 'manual' | 'statement' | 'recurring'

/**
 * expense: money spent. income: money in. investment: money moved from cash into investments; it
 * leaves the cash balance but is not spending, so it is never counted in "spent". Each has its own
 * categories.
 */
export type CategoryKind = 'expense' | 'income' | 'investment'

/** transfer: money moved between two of your own accounts. Neither spending nor income; no category. */
export type TransactionKind = CategoryKind | 'transfer'

export interface Transaction {
  id: string
  kind: TransactionKind
  occurred_at: string // RFC 3339
  amount_minor: number // integer minor units (cents); negative = money out of cash
  currency: string // ISO 4217; every currency is stored with 2 decimals
  merchant: string // who was paid, or where the money came from
  category?: string // Category.id; unknown or missing ids count as the kind's catch-all. None on transfers.
  note?: string
  card_last4?: string
  /** An expense that brought money back (a refund, a friend paying their share): amount_minor is positive. */
  refund?: boolean
  account?: string // Account.id the money left or entered; none = no balance changes
  account_minor?: number // amount_minor in the account's currency (signed)
  /** amount_minor in the summary currency at the day's rate (signed). Missing on old entries in that currency. */
  base_minor?: number
  // Transfers only: the receiving account and what it received, positive, in its currency. A
  // transfer's amount_minor is negative: what left `account`.
  to_account?: string
  to_minor?: number
  source: Source
  dedup_key: string
}

/** Body of POST /v1/transactions and PUT /v1/transactions/{id}. The server sets the sign from `kind`. */
export interface TransactionInput {
  kind: TransactionKind
  amount_minor: number // always positive
  currency: string // any ISO code, converted by the server; a transfer's is its source account's
  occurred_at: string // RFC 3339; never in the future
  merchant: string // 1-80 characters
  category: string // Category.id of the same kind; "" for transfers
  note?: string // up to 500 characters
  account?: string // Account.id; required for transfers
  refund?: boolean // expenses only: money back
  to_account?: string // transfers only
  to_amount_minor?: number // transfers only: what to_account received; 0 or missing = converted at the day's rate
}

export type RepeatUnit = 'week' | 'month' | 'year'

/**
 * Body of POST /v1/recurring and PUT /v1/recurring/{id}. Each occurrence becomes a normal transaction
 * with source "recurring" and dedup_key "recurring:<rule id>:<YYYY-MM-DD>", so re-running the daily
 * job never duplicates. Editing a rule only affects occurrences not created yet.
 */
export interface RecurringInput {
  kind: CategoryKind // rules cannot be transfers
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
  kind: CategoryKind
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

export type AccountKind = 'cash' | 'credit'

/**
 * A place money sits. Its balance is the opening balance plus every entry tied to it from the
 * opening day on. GET and PUT /v1/accounts; a PUT replaces the whole list, at most 12.
 */
export interface Account {
  id: string // ^[a-z0-9][a-z0-9-]{0,39}$, stable
  name: string // 1-30 characters, unique ignoring case
  kind: AccountKind // credit: a card; its balance is negative while you owe
  currency: string // fixed once the account exists
  opening_minor: number // balance at the start of opening_date, signed
  opening_date: string // YYYY-MM-DD
}

export type HoldingKind = 'cash' | 'investment' | 'property' | 'liability'

export interface Holding {
  id: string
  name: string
  kind: HoldingKind // an account is cash, a credit card a liability
  currency: string
  balance_minor: number // in `currency`; liabilities are negative
  base_minor: number // in NetWorth.currency at today's rate
}

export interface NetWorth {
  as_of: string // RFC 3339
  currency: string
  holdings: Holding[]
}

// Statement import (backend/internal/ledger/imports.go): POST /v1/imports/preview parses a bank
// statement and suggests an entry per line; POST /v1/imports books the lines the person confirms.

export type ImportFormat = 'sg' | 'revolut' | 'bancolombia'

/**
 * imported: this exact line was booked before. duplicate: an existing entry of the account has the
 * same amount within 3 days (e.g. from Google Wallet). before_opening: dated before the account's
 * opening date, so it cannot be booked.
 */
export type ImportStatus = 'new' | 'imported' | 'duplicate' | 'before_opening'

export interface ImportLine {
  key: string // stable per statement line; booking it twice is a no-op
  day: string // YYYY-MM-DD
  amount_minor: number // signed: negative = money out of the account
  currency: string
  description: string // the bank's raw text
  status: ImportStatus
  match?: Transaction // the existing entry, for imported and duplicate
  // Suggestions, editable before importing.
  kind: TransactionKind
  merchant: string
  category?: string // Category.id; none for transfers
  other_account?: string // transfers: the account on the other side
  learned: boolean // copied from an earlier import of the same counterparty
}

export interface ImportPreview {
  format: ImportFormat
  account: string
  lines: ImportLine[]
  skipped: number // rows of the file that are not movements
}

/**
 * One confirmed line of POST /v1/imports; the server rejects unknown fields. The sign is the
 * statement's: a positive expense is money back, income must be positive, an investment negative,
 * and a transfer's sign says whether the money went to other_account or came from it.
 */
export interface ImportEntry {
  key: string
  day: string
  amount_minor: number
  currency: string
  kind: TransactionKind
  merchant: string
  category?: string // required unless a transfer
  note?: string
  other_account?: string // transfers only; not the statement's account
}

export interface ImportResult {
  added: number
  already_there: number // booked by an earlier import
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
  accounts(signal?: AbortSignal): Promise<Account[]>
  saveAccounts(accounts: Account[]): Promise<Account[]> // replaces the list; a currency cannot change
  previewImport(account: string, file: string): Promise<ImportPreview> // file: base64, at most 2 MB decoded
  importLines(account: string, lines: ImportEntry[]): Promise<ImportResult> // 1-200 lines per call
}

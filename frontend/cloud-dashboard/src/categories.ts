import type { Category, CategoryKind, Transaction, TransactionKind } from './api/types.ts'

/** The kinds that have categories; transfers have none. */
export const KINDS: CategoryKind[] = ['expense', 'income', 'investment']

export const KIND_LABEL: Record<TransactionKind, string> = {
  expense: 'Expense',
  income: 'Income',
  investment: 'Investment',
  transfer: 'Transfer',
}

export const isCategoryKind = (k: string): k is CategoryKind => (KINDS as string[]).includes(k)

/** Built-in catch-all per kind: can be renamed and recolored, never deleted. */
export const CATCH_ALL: Record<CategoryKind, string> = {
  expense: 'other',
  income: 'income',
  investment: 'investment',
}

/** The categorical palette has 8 validated slots; a 9th category would need a made-up color. */
export const MAX_CATEGORIES = 8

export const DEFAULT_CATEGORIES: Category[] = [
  { id: 'subscriptions', kind: 'expense', name: 'Subscriptions', color: 1 },
  { id: 'mortgage', kind: 'expense', name: 'Mortgage', color: 2 },
  { id: 'bills', kind: 'expense', name: 'Bills', color: 3 },
  { id: 'restaurants', kind: 'expense', name: 'Restaurants', color: 4 },
  { id: 'groceries', kind: 'expense', name: 'Groceries', color: 5 },
  { id: 'clothes', kind: 'expense', name: 'Clothes', color: 6 },
  { id: 'other', kind: 'expense', name: 'Other', color: 7 },
  { id: 'salary', kind: 'income', name: 'Salary', color: 1 },
  { id: 'income', kind: 'income', name: 'Other income', color: 2 },
  { id: 'index-funds', kind: 'investment', name: 'Index funds', color: 1 },
  { id: 'investment', kind: 'investment', name: 'Other investments', color: 2 },
]

export const colorVar = (slot: number) => `var(--cat-${slot})`

/** Transactions from before `kind` existed: the sign decides. */
export function kindOf(t: Pick<Transaction, 'kind' | 'amount_minor'>): TransactionKind {
  return t.kind ?? (t.amount_minor < 0 ? 'expense' : 'income')
}

/** Ids that aren't configured for this kind (deleted, or new from the parser) fall into its catch-all. */
export function resolveId(id: string | undefined, kind: CategoryKind, known: ReadonlySet<string>): string {
  return id && known.has(id) ? id : CATCH_ALL[kind]
}

/** Builds one value per kind, typed as a full record. */
export function perKind<T>(fn: (kind: CategoryKind) => T): Record<CategoryKind, T> {
  return { expense: fn('expense'), income: fn('income'), investment: fn('investment') }
}

export interface CategoryIndex {
  all: Category[]
  byKind: Record<CategoryKind, Category[]>
  ids: Record<CategoryKind, ReadonlySet<string>>
  get(id: string | undefined, kind: CategoryKind): Category
}

export function indexCategories(all: Category[]): CategoryIndex {
  const byId = new Map(all.map((c) => [c.id, c]))
  const byKind = perKind((k) => all.filter((c) => c.kind === k))
  const ids = perKind((k): ReadonlySet<string> => new Set(byKind[k].map((c) => c.id)))
  return {
    all,
    byKind,
    ids,
    get: (id, kind) => {
      const resolved = resolveId(id, kind, ids[kind])
      return byId.get(resolved) ?? { id: resolved, kind, name: KIND_LABEL[kind], color: 0 }
    },
  }
}

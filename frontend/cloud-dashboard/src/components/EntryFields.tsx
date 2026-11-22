import type { ReactNode } from 'react'
import type { Account, TransactionKind } from '../api/types.ts'
import { KINDS, KIND_LABEL, type CategoryIndex } from '../categories.ts'
import { MERCHANT_HINT, MERCHANT_LABEL, type useEntry } from '../entry.ts'

// The fields a single entry and a recurring rule share: type, amount, description, category, note.
// With `accounts` (single entries only) it adds transfers, the account and currency, and money back.

interface Props {
  entry: ReturnType<typeof useEntry>
  categories: CategoryIndex
  /** Single entries only: the accounts to pick from, and the currencies to offer. */
  accounts?: { list: Account[]; currencies: string[] }
  /** Rendered beside the amount: the date, or the start date for a rule. */
  dateField: ReactNode
  /** Rendered after the category: the repeat settings for a rule. */
  children?: ReactNode
}

export function EntryFields({ entry, categories, accounts, dateField, children }: Props) {
  const { values: v, set } = entry
  const kinds: TransactionKind[] = accounts ? [...KINDS, 'transfer'] : KINDS
  const transfer = v.kind === 'transfer'
  const from = accounts?.list.find((a) => a.id === v.account)
  const to = accounts?.list.find((a) => a.id === v.toAccount)
  // Only worth asking when the money changes currency on the way.
  const askReceived = transfer && from && to && from.currency !== to.currency

  const accountOptions = (list: Account[]) =>
    list.map((a) => (
      <option key={a.id} value={a.id}>
        {a.name} ({a.currency})
      </option>
    ))

  return (
    <>
      <div className="segmented" role="radiogroup" aria-label="Type">
        {kinds.map((k) => (
          <button
            key={k}
            type="button"
            role="radio"
            aria-checked={v.kind === k}
            className={v.kind === k ? 'is-on' : undefined}
            onClick={() => set('kind', k)}
          >
            {KIND_LABEL[k]}
          </button>
        ))}
      </div>

      <div className="form-grid">
        <label>
          <span>Amount ({v.currency})</span>
          <input
            inputMode="decimal"
            autoComplete="off"
            placeholder="0.00"
            value={v.amount}
            onChange={(e) => set('amount', e.target.value)}
            autoFocus
            required
          />
        </label>
        {dateField}

        {accounts &&
          (transfer ? (
            <>
              <label>
                <span>From</span>
                <select value={v.account} onChange={(e) => set('account', e.target.value)} required>
                  <option value="" disabled>
                    Pick an account
                  </option>
                  {accountOptions(accounts.list)}
                </select>
              </label>
              <label>
                <span>To</span>
                <select value={v.toAccount} onChange={(e) => set('toAccount', e.target.value)} required>
                  <option value="" disabled>
                    Pick an account
                  </option>
                  {accountOptions(accounts.list.filter((a) => a.id !== v.account))}
                </select>
              </label>
              {askReceived && (
                <label className="span-2">
                  <span>
                    Amount received ({to.currency}) <span className="optional">(optional)</span>
                  </span>
                  <input
                    inputMode="decimal"
                    autoComplete="off"
                    placeholder="Blank: converted at the day's rate"
                    value={v.toAmount}
                    onChange={(e) => set('toAmount', e.target.value)}
                  />
                </label>
              )}
            </>
          ) : (
            <>
              <label>
                <span>Account</span>
                <select value={v.account} onChange={(e) => set('account', e.target.value)}>
                  <option value="">No account</option>
                  {accountOptions(accounts.list)}
                </select>
              </label>
              <label>
                <span>Currency</span>
                <select value={v.currency} onChange={(e) => set('currency', e.target.value)}>
                  {accounts.currencies.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </label>
            </>
          ))}

        <label className="span-2">
          <span>{MERCHANT_LABEL[v.kind]}</span>
          <input
            value={v.merchant}
            maxLength={80}
            placeholder={MERCHANT_HINT[v.kind]}
            onChange={(e) => set('merchant', e.target.value)}
            required
          />
        </label>
        {v.kind !== 'transfer' && (
          <label className="span-2">
            <span>Category</span>
            <select value={v.category} onChange={(e) => set('category', e.target.value)}>
              {categories.byKind[v.kind].map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {accounts && v.kind === 'expense' && (
          <label className="span-2 check">
            <input type="checkbox" checked={v.refund} onChange={(e) => set('refund', e.target.checked)} />
            <span>
              Money back <span className="optional">(a refund, or a friend paying their share)</span>
            </span>
          </label>
        )}
        {children}
        <label className="span-2">
          <span>
            Note <span className="optional">(optional)</span>
          </span>
          <textarea
            rows={2}
            maxLength={500}
            placeholder={v.kind === 'income' ? 'Where the money came from' : 'What it was for'}
            value={v.note}
            onChange={(e) => set('note', e.target.value)}
          />
        </label>
      </div>

      {v.kind === 'investment' && (
        <p className="card-sub form-hint">Investments leave your cash but are not counted as spending.</p>
      )}
      {transfer && (
        <p className="card-sub form-hint">
          Moves money between your own accounts; it is neither spending nor income.
          {accounts && accounts.list.length < 2 && ' Add at least two accounts under Accounts first.'}
        </p>
      )}
      {v.kind === 'expense' && v.refund && accounts && (
        <p className="card-sub form-hint">Money back lowers the category's spending this month.</p>
      )}
    </>
  )
}

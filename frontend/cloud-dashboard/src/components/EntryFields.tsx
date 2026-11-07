import type { ReactNode } from 'react'
import { KINDS, KIND_LABEL, type CategoryIndex } from '../categories.ts'
import { MERCHANT_HINT, MERCHANT_LABEL, type useEntry } from '../entry.ts'

// The fields a single entry and a recurring rule share: type, amount, description, category, note.

interface Props {
  entry: ReturnType<typeof useEntry>
  categories: CategoryIndex
  currency: string
  /** Rendered beside the amount: the date, or the start date for a rule. */
  dateField: ReactNode
  /** Rendered after the category: the repeat settings for a rule. */
  children?: ReactNode
}

export function EntryFields({ entry, categories, currency, dateField, children }: Props) {
  const { values: v, set } = entry
  return (
    <>
      <div className="segmented" role="radiogroup" aria-label="Type">
        {KINDS.map((k) => (
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
          <span>Amount ({currency})</span>
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
    </>
  )
}

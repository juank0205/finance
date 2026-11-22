import { useEffect, useRef, useState } from 'react'
import type { Category, CategoryKind } from '../api/types.ts'
import { CATCH_ALL, KINDS, KIND_LABEL, MAX_CATEGORIES, colorVar, perKind } from '../categories.ts'

interface Props {
  initial: Category[]
  onSave: (categories: Category[]) => Promise<void>
  onClose: () => void
}

type Draft = Record<CategoryKind, Category[]>

const SLOTS = Array.from({ length: MAX_CATEGORIES }, (_, i) => i + 1)

const TAB_LABEL: Record<CategoryKind, string> = {
  expense: 'Expenses',
  income: 'Income',
  investment: 'Investments',
}

function newId(): string {
  return `c-${crypto.randomUUID().slice(0, 8)}`
}

/** Mirrors the server-side checks so most mistakes are caught before saving. */
function problem(draft: Draft): string | null {
  for (const kind of KINDS) {
    const seen = new Set<string>()
    for (const c of draft[kind]) {
      const name = c.name.trim()
      if (!name) return `${TAB_LABEL[kind]}: every category needs a name.`
      if (seen.has(name.toLowerCase())) return `${TAB_LABEL[kind]}: "${name}" appears twice.`
      seen.add(name.toLowerCase())
    }
  }
  return null
}

export function CategorySettings({ initial, onSave, onClose }: Props) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [draft, setDraft] = useState<Draft>(
    () => perKind((k) => initial.filter((c) => c.kind === k)),
  )
  const [tab, setTab] = useState<CategoryKind>('expense')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  // Any edit may fix the last problem, so clear its message.
  const edit = (fn: (list: Category[]) => Category[]) => {
    setError(null)
    setDraft((d) => ({ ...d, [tab]: fn(d[tab]) }))
  }

  const update = (id: string, patch: Partial<Category>) =>
    edit((list) => list.map((c) => (c.id === id ? { ...c, ...patch } : c)))

  const move = (i: number, by: number) =>
    edit((list) => {
      const next = [...list]
      ;[next[i], next[i + by]] = [next[i + by], next[i]]
      return next
    })

  const add = () =>
    edit((list) => {
      const used = new Set(list.map((c) => c.color))
      const color = SLOTS.find((s) => !used.has(s)) ?? 1
      return [...list, { id: newId(), kind: tab, name: '', color }]
    })

  const save = async () => {
    const p = problem(draft)
    if (p) {
      setError(p)
      return
    }
    setSaving(true)
    setError(null)
    try {
      await onSave(KINDS.flatMap((k) => draft[k].map((c) => ({ ...c, name: c.name.trim() }))))
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setSaving(false)
    }
  }

  const list = draft[tab]
  const catchAll = CATCH_ALL[tab]

  return (
    <dialog ref={dialog} className="settings" onClose={onClose} aria-labelledby="settings-title">
      <h2 id="settings-title">Categories</h2>
      <p className="card-sub">
        Order sets the chart and legend order. Up to {MAX_CATEGORIES} per type, one color each. Entries in a
        deleted category move to the type's catch-all.
      </p>

      <div className="segmented" role="tablist" aria-label="Type">
        {KINDS.map((k) => (
          <button
            key={k}
            type="button"
            role="tab"
            aria-selected={tab === k}
            className={tab === k ? 'is-on' : undefined}
            onClick={() => setTab(k)}
          >
            {TAB_LABEL[k]}
          </button>
        ))}
      </div>

      <ol className="cat-edit" role="tabpanel" aria-label={TAB_LABEL[tab]}>
        {list.map((c, i) => {
          const usedByOthers = new Set(list.filter((o) => o.id !== c.id).map((o) => o.color))
          const label = c.name || 'category'
          return (
            <li key={c.id}>
              <div className="cat-edit-row">
                <input
                  value={c.name}
                  maxLength={30}
                  placeholder="Name"
                  aria-label={`${KIND_LABEL[tab]} category ${i + 1} name`}
                  onChange={(e) => update(c.id, { name: e.target.value })}
                />
                <button
                  type="button"
                  className="ghost icon"
                  onClick={() => move(i, -1)}
                  disabled={i === 0}
                  aria-label={`Move ${label} up`}
                >
                  ↑
                </button>
                <button
                  type="button"
                  className="ghost icon"
                  onClick={() => move(i, 1)}
                  disabled={i === list.length - 1}
                  aria-label={`Move ${label} down`}
                >
                  ↓
                </button>
                <button
                  type="button"
                  className="ghost icon"
                  onClick={() => edit((l) => l.filter((o) => o.id !== c.id))}
                  disabled={c.id === catchAll}
                  title={c.id === catchAll ? 'Catches everything uncategorized; cannot be deleted' : undefined}
                  aria-label={`Delete ${label}`}
                >
                  ✕
                </button>
              </div>
              <div className="swatch-picker" role="radiogroup" aria-label={`Color for ${label}`}>
                {SLOTS.map((s) => (
                  <button
                    key={s}
                    type="button"
                    role="radio"
                    aria-checked={c.color === s}
                    aria-label={`Color ${s}${usedByOthers.has(s) ? ' (in use)' : ''}`}
                    disabled={usedByOthers.has(s)}
                    className="swatch-option"
                    style={{ background: colorVar(s) }}
                    onClick={() => update(c.id, { color: s })}
                  />
                ))}
              </div>
            </li>
          )
        })}
      </ol>

      <button type="button" className="ghost" onClick={add} disabled={list.length >= MAX_CATEGORIES}>
        + Add category
      </button>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      <div className="settings-actions">
        <button type="button" className="ghost" onClick={onClose}>
          Cancel
        </button>
        <button type="button" className="primary" onClick={save} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
      </div>
    </dialog>
  )
}

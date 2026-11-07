import type { Category } from '../api/types.ts'
import { colorVar } from '../categories.ts'

export function Swatch({ color }: { color: number }) {
  return <span className="swatch" style={{ background: colorVar(color) }} aria-hidden="true" />
}

export function Legend({ categories }: { categories: Category[] }) {
  return (
    <ul className="legend" aria-label="Legend">
      {categories.map((c) => (
        <li key={c.id}>
          <Swatch color={c.color} />
          {c.name}
        </li>
      ))}
    </ul>
  )
}

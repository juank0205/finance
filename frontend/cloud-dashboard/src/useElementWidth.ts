import { useCallback, useState } from 'react'

/**
 * Tracks an element's content width so SVG charts can draw in real pixels (text never scales).
 * Returns a callback ref, so it keeps working when the element unmounts and remounts.
 */
export function useElementWidth<T extends HTMLElement>() {
  const [width, setWidth] = useState(0)

  const ref = useCallback((el: T | null) => {
    if (!el) return
    const ro = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  return [ref, width] as const
}

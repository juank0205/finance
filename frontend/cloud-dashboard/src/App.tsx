import { useEffect, useMemo, useState } from 'react'
import { authEnabled, signOut } from './api/auth.ts'
import { api, usingMockData } from './api/client.ts'
import type { Account, Category, NetWorth, Summary, Transaction, TransactionInput } from './api/types.ts'
import { indexCategories } from './categories.ts'
import { AccountSettings } from './components/AccountSettings.tsx'
import { CategoryPie } from './components/CategoryPie.tsx'
import { CategorySettings } from './components/CategorySettings.tsx'
import { MonthlyChart } from './components/MonthlyChart.tsx'
import { RecurringPanel } from './components/RecurringPanel.tsx'
import { NetWorthCard } from './components/NetWorthCard.tsx'
import { StatTiles } from './components/StatTiles.tsx'
import { TransactionForm } from './components/TransactionForm.tsx'
import { TransactionsTable } from './components/TransactionsTable.tsx'
import { addMonths, currentMonth, monthLabel } from './format.ts'
import './App.css'

interface Data {
  summary: Summary
  transactions: Transaction[]
}

/** The outcome of the last finished request. While `key` differs from it, a load is in flight. */
interface Result<T> {
  key: string
  data?: T
  error?: string
}

/** ?month=YYYY-MM, so a month can be bookmarked. Anything invalid or in the future falls back to now. */
function monthFromUrl(thisMonth: string): string {
  const m = new URLSearchParams(window.location.search).get('month')
  return m && /^\d{4}-(0[1-9]|1[0-2])$/.test(m) && m <= thisMonth ? m : thisMonth
}

function App() {
  // Read the clock once; a tab left open past midnight keeps its day until reload.
  const [today] = useState(() => new Date())
  const thisMonth = currentMonth(today)
  const [month, setMonth] = useState(() => monthFromUrl(thisMonth))
  const [category, setCategory] = useState<string | null>(null)
  const [categoryList, setCategoryList] = useState<Category[] | null>(null)
  const [accountList, setAccountList] = useState<Account[] | null>(null)
  // Bumped after saving categories, so the summary is re-fetched with the new grouping.
  const [version, setVersion] = useState(0)
  // Bumped after anything that moves a balance: accounts, and entries tied to them.
  const [balancesVersion, setBalancesVersion] = useState(0)
  const [result, setResult] = useState<Result<Data> | null>(null)
  const [netWorth, setNetWorth] = useState<Result<NetWorth> | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [accountsOpen, setAccountsOpen] = useState(false)
  const [recurringOpen, setRecurringOpen] = useState(false)
  // null: closed. {}: adding. {editing}: editing that manual entry.
  const [form, setForm] = useState<{ editing?: Transaction } | null>(null)
  const [notice, setNotice] = useState<{ text: string; month?: string } | null>(null)
  const [setupError, setSetupError] = useState<string | null>(null)

  useEffect(() => {
    const ctrl = new AbortController()
    api
      .then((a) => a.categories(ctrl.signal))
      .then(setCategoryList)
      .catch((err: unknown) => !ctrl.signal.aborted && setSetupError(`Could not load categories: ${String(err)}`))
    api
      .then((a) => a.accounts(ctrl.signal))
      .then(setAccountList)
      .catch((err: unknown) => !ctrl.signal.aborted && setSetupError(`Could not load accounts: ${String(err)}`))
    return () => ctrl.abort()
  }, [])

  useEffect(() => {
    const ctrl = new AbortController()
    const nwKey = String(balancesVersion)
    api
      .then((a) => a.netWorth(ctrl.signal))
      .then((data) => setNetWorth({ key: nwKey, data }))
      .catch((err: unknown) => {
        if (ctrl.signal.aborted) return
        // Keep the last balances on screen under the error.
        setNetWorth((n) => ({ key: nwKey, error: String(err), data: n?.data }))
      })
    return () => ctrl.abort()
  }, [balancesVersion])

  const key = `${month}#${version}`
  useEffect(() => {
    const ctrl = new AbortController()
    api
      .then((a) => Promise.all([a.summary(month, ctrl.signal), a.transactions(month, ctrl.signal)]))
      .then(([summary, transactions]) => setResult({ key, data: { summary, transactions } }))
      .catch((err: unknown) => {
        if (ctrl.signal.aborted) return
        // Keep the previous month's data on screen under the error.
        setResult((r) => ({ key, error: String(err), data: r?.data }))
      })
    return () => ctrl.abort()
  }, [month, key])

  const categories = useMemo(() => (categoryList ? indexCategories(categoryList) : null), [categoryList])

  const selectMonth = (m: string) => {
    setMonth(m)
    setCategory(null)
    const url = new URL(window.location.href)
    if (m === thisMonth) url.searchParams.delete('month')
    else url.searchParams.set('month', m)
    window.history.replaceState(null, '', url)
  }

  const saveCategories = async (list: Category[]) => {
    const saved = await (await api).saveCategories(list)
    setCategoryList(saved)
    setCategory((c) => (c && saved.some((s) => s.id === c) ? c : null))
    setVersion((v) => v + 1)
  }

  const saveAccounts = async (list: Account[]) => {
    const saved = await (await api).saveAccounts(list)
    setAccountList(saved)
    setBalancesVersion((v) => v + 1)
  }

  const submitEntry = async (input: TransactionInput) => {
    const a = await api
    const saved = form?.editing ? await a.updateTransaction(form.editing.id, input) : await a.createTransaction(input)
    const savedMonth = currentMonth(new Date(saved.occurred_at))
    const verb = form?.editing ? 'Saved' : 'Added'
    setNotice(
      savedMonth === month
        ? { text: `${verb} “${saved.merchant}”.` }
        : { text: `${verb} “${saved.merchant}” in ${monthLabel(savedMonth)}.`, month: savedMonth },
    )
    setVersion((v) => v + 1)
    // The entry, or the one it replaced, may be tied to an account.
    if (saved.account || form?.editing?.account) setBalancesVersion((v) => v + 1)
  }

  const deleteEntry = async () => {
    if (!form?.editing) return
    await (await api).deleteTransaction(form.editing.id)
    const skipped = form.editing.source === 'recurring'
    setNotice({ text: `${skipped ? 'Skipped' : 'Deleted'} “${form.editing.merchant}”.` })
    setVersion((v) => v + 1)
    if (form.editing.account) setBalancesVersion((v) => v + 1)
  }

  const loading = result?.key !== key
  const data = result?.data
  const error = loading ? undefined : result?.error
  const currency = data?.summary.currency ?? netWorth?.data?.currency ?? 'EUR'

  return (
    <div className="page" aria-busy={loading}>
      <header className="top">
        <div className="brand">
          <h1>Finance</h1>
          {usingMockData && (
            <span className="badge" title="VITE_API_URL is not set, so the numbers are generated">
              Demo data
            </span>
          )}
        </div>
        <div className="top-actions">
          <button type="button" className="ghost" onClick={() => setRecurringOpen(true)} disabled={!categories}>
            Recurring
          </button>
          <button type="button" className="ghost" onClick={() => setSettingsOpen(true)} disabled={!categoryList}>
            Categories
          </button>
          <button type="button" className="ghost" onClick={() => setAccountsOpen(true)} disabled={!accountList}>
            Accounts
          </button>
          <button type="button" className="primary" onClick={() => setForm({})} disabled={!categories || !accountList}>
            + Add
          </button>
          {authEnabled && (
            <button type="button" className="ghost" onClick={() => void signOut()}>
              Sign out
            </button>
          )}
        </div>
      </header>

      {notice && (
        <div className="notice" role="status">
          <span>{notice.text}</span>
          {notice.month && (
            <button
              type="button"
              className="ghost small"
              onClick={() => {
                selectMonth(notice.month!)
                setNotice(null)
              }}
            >
              Show {monthLabel(notice.month)}
            </button>
          )}
          <button type="button" className="ghost small" onClick={() => setNotice(null)} aria-label="Dismiss">
            ✕
          </button>
        </div>
      )}

      <main>
        <NetWorthCard
          data={netWorth?.data}
          error={netWorth?.error}
          onAddAccounts={accountList ? () => setAccountsOpen(true) : undefined}
        />

        <div className="month-bar">
          <h2 className="month-current" aria-live="polite">
            {monthLabel(month)}
          </h2>
          <nav className="month-picker" aria-label="Month">
            <button
              type="button"
              className="ghost"
              onClick={() => selectMonth(addMonths(month, -1))}
              aria-label="Previous month"
            >
              ‹
            </button>
            <button
              type="button"
              className="ghost"
              onClick={() => selectMonth(addMonths(month, 1))}
              disabled={month >= thisMonth}
              aria-label="Next month"
            >
              ›
            </button>
          </nav>
        </div>

        {(setupError || error) && (
          <p className="error" role="alert">
            {setupError ?? `Could not load data: ${error}`}
          </p>
        )}

        {!data || !categories ? (
          !setupError && <p className="loading">Loading…</p>
        ) : (
          <div className={`month-body ${loading ? 'is-stale' : ''}`}>
            <StatTiles summary={data.summary} today={today} />
            <div className="charts">
              <MonthlyChart
                months={data.summary.months}
                selected={data.summary.month}
                currency={data.summary.currency}
                categories={categories}
                onSelect={selectMonth}
              />
              <CategoryPie
                totals={data.summary.by_category}
                total={data.summary.spent_minor}
                currency={data.summary.currency}
                categories={categories}
                active={category}
                onToggle={(c) => setCategory((cur) => (cur === c ? null : c))}
              />
            </div>
            <TransactionsTable
              transactions={data.transactions}
              categories={categories}
              accounts={accountList ?? []}
              filter={category}
              onFilter={setCategory}
              onEdit={(t) => setForm({ editing: t })}
            />
          </div>
        )}
      </main>

      {form && categories && accountList && (
        <TransactionForm
          categories={categories}
          accounts={accountList}
          currency={currency}
          today={today}
          editing={form.editing}
          onSubmit={submitEntry}
          onDelete={form.editing ? deleteEntry : undefined}
          onClose={() => setForm(null)}
        />
      )}

      {recurringOpen && categories && (
        <RecurringPanel
          categories={categories}
          currency={currency}
          today={today}
          onChanged={(text) => {
            setNotice({ text })
            setVersion((v) => v + 1)
          }}
          onClose={() => setRecurringOpen(false)}
        />
      )}

      {settingsOpen && categoryList && (
        <CategorySettings initial={categoryList} onSave={saveCategories} onClose={() => setSettingsOpen(false)} />
      )}

      {accountsOpen && accountList && (
        <AccountSettings
          initial={accountList}
          currency={currency}
          today={today}
          onSave={saveAccounts}
          onClose={() => setAccountsOpen(false)}
        />
      )}
    </div>
  )
}

export default App

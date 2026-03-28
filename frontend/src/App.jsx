import { useState, useRef } from 'react'
import './App.css'
import MOCK_RESULTS from './mockData'

const THROTTLE_MS = 500

// Fires at most once per THROTTLE_MS regardless of how fast the user types.
function useThrottle(fn, delay) {
  const lastCallAt = useRef(0)

  return function (...args) {
    const now = Date.now()
    if (now - lastCallAt.current >= delay) {
      lastCallAt.current = now
      fn(...args)
    }
  }
}

function ResultCard({ result, maxScore }) {
  const scorePercent = maxScore > 0 ? (result.score / maxScore) * 100 : 0

  return (
    <div className="result-card">
      <div className="result-meta">
        <span className="result-id">Doc #{result.id}</span>
        <span className="result-score">
          {result.score.toFixed(4)}
          <span className="result-score-bar-wrapper">
            <span
              className="result-score-bar"
              style={{ width: `${scorePercent}%` }}
            />
          </span>
        </span>
      </div>
      <p className="result-text">{result.text}</p>
    </div>
  )
}

export default function App() {
  const [query, setQuery]       = useState('')
  const [results, setResults]   = useState([])
  const [loading, setLoading]   = useState(false)
  const [demoMode, setDemoMode] = useState(false)
  const [searched, setSearched] = useState(false)

  async function fetchResults(q) {
    if (!q.trim()) {
      setResults([])
      setSearched(false)
      setDemoMode(false)
      return
    }

    setLoading(true)
    setSearched(true)

    try {
      const res = await fetch(`/search?query=${encodeURIComponent(q)}`)
      if (!res.ok) throw new Error('Bad response')
      const data = await res.json()
      setResults(data ?? [])
      setDemoMode(false)
    } catch {
      // Backend unreachable — fall back to mock data
      setResults(MOCK_RESULTS)
      setDemoMode(true)
    } finally {
      setLoading(false)
    }
  }

  const throttledFetch = useThrottle(fetchResults, THROTTLE_MS)

  function handleChange(e) {
    const q = e.target.value
    setQuery(q)
    throttledFetch(q)
  }

  const maxScore = results.length > 0 ? results[0].score : 1

  return (
    <div className="app">
      <header className="header">
        <div className="logo">Gopher<span>Seek</span></div>
        <p className="tagline">BM25-powered full-text search</p>
      </header>

      <div className="search-wrapper">
        <input
          className="search-input"
          type="text"
          placeholder="Search documents..."
          value={query}
          onChange={handleChange}
          autoFocus
        />
        <span className="search-icon">&#128269;</span>
      </div>

      <p className="search-hint">Throttled — API called at most once per {THROTTLE_MS}ms</p>

      {demoMode && (
        <div className="demo-banner">
          Backend not reachable &mdash; showing demo results. Start the Go server on port 8080 to search live data.
        </div>
      )}

      {loading && (
        <div className="state-message">
          <div className="loading-dots">
            <span /><span /><span />
          </div>
        </div>
      )}

      {!loading && searched && (
        <div className="results">
          <p className="results-count">
            {results.length} result{results.length !== 1 ? 's' : ''} for &ldquo;{query}&rdquo;
          </p>
          {results.map((r) => (
            <ResultCard key={r.id} result={r} maxScore={maxScore} />
          ))}
          {results.length === 0 && (
            <div className="state-message">
              <div className="icon">&#128269;</div>
              <p>No results found for &ldquo;{query}&rdquo;</p>
            </div>
          )}
        </div>
      )}

      {!loading && !searched && (
        <div className="state-message">
          <div className="icon">&#128064;</div>
          <p>Type something to search the document corpus</p>
        </div>
      )}
    </div>
  )
}

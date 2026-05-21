# Concepts

## Context in Go
- Carries a deadline, cancellation signal, and optional key-value metadata across goroutine/API boundaries.
- `context.Background()` = no timeout, never cancels. Fine for scripts; bad for production I/O.
- `context.WithTimeout` creates a context with a deadline. The clock starts the moment it's created and applies to everything sharing that context — not per operation.
- A `ctx` passed to both `pgx.Connect` and `conn.Query` shares the same budget. If connect takes 4s and timeout is 5s, only 1s remains for the query.
- Convention: any function that accepts `ctx context.Context` as its first argument is almost certainly doing I/O.

## defer cancel()
- `context.WithTimeout` starts an internal timer goroutine. `cancel()` stops it early.
- If you don't call `cancel()` and the function returns before the timeout fires, the timer goroutine leaks until it naturally expires.
- `defer cancel()` guarantees cleanup however the function exits — normal return, early error, or panic.

## defer conn.Close() / rows.Close()
- `conn.Close()` returns the TCP connection slot back to PostgreSQL. Unclosed = DB slot occupied indefinitely.
- `rows.Close()` releases the server-side cursor and buffer. DB holds state until you signal you're done.
- `defer` ensures these run regardless of how the function exits — no missed cleanup on early returns.

## pgx Rows and .Next()
- `conn.Query` sends the SQL and starts receiving data — it does NOT wait for all rows.
- Rows are streamed in batches (client-side buffer). `.Next()` advances one row at a time through the buffer.
- When the buffer is empty, pgx fetches the next batch transparently. From your code's perspective it's one row at a time; under the hood it's batched.
- This is why `rows.Close()` matters even if you stop reading halfway — the DB may still have unsent batches in flight.

## SELECT lock in PostgreSQL (MVCC)
- A plain `SELECT` acquires no row locks.
- PostgreSQL uses MVCC (Multi-Version Concurrency Control) — your query gets a snapshot of data at the moment it started.
- Other transactions can freely read and write the same rows; your query is unaffected.

## Connection Pooling
- Keeps N connections open and reuses them across requests instead of opening/closing per request.
- Opening a connection is expensive: TCP handshake + auth + DB allocating a backend process (~5-10MB RAM).
- Pool lends a connection to each request; excess requests queue until one is free.
- The pool is application-side (inside your process). The DB has no concept of a pool — it just sees N open connections.
- **PgBouncer** is an external pool proxy that sits between app and DB — useful when many app instances would otherwise each hold their own pool.
- Default pool size in pgxpool = 4 × CPU cores. This is a guess, not a principled number. Real sizing depends on DB `max_connections`, number of app instances, and DB server RAM.

## Connection Pooling — Three Patterns (benchmarked)

**DirectConnection** (`pgx.Connect` per call)
- Opens a fresh TCP+TLS+auth connection on every call, closes it when done.
- Simple but expensive under load: 100 concurrent callers = 100 simultaneous handshakes.
- Memory scales linearly with concurrency — each connection allocates its own buffers.

**PoolPerCall** (`pgxpool.New` per call)
- Creates a pool object on every call and destroys it immediately after.
- Defeats the entire purpose of pooling — pays pool-management overhead on top of connection overhead.
- Strictly worse than DirectConnection on every metric. Never do this.

**SharedPool** (`pgxpool.New` once, shared across all callers)
- One pool created at startup, all goroutines borrow and return connections from it.
- Connections stay open between requests — subsequent callers skip the handshake entirely.
- Memory stays flat regardless of concurrency: 100 callers share N connections, not 100 separate ones.

## Pool Sizing and the MaxConns bottleneck
- Default `MaxConns = 4 × NumCPU`. At 100 concurrent queries on an 8-core machine that's 32 max connections.
- With 100 goroutines hitting a 32-connection pool, 68 goroutines queue — wall time goes up even though memory goes down.
- Setting `MaxConns = concurrentQueries` removes the queue and matches DirectConnection on speed while keeping the memory advantage.
- Tune `MaxConns` to the DB's `max_connections` limit divided by the number of app instances, not to your concurrency level.

## Cold vs Warm pool (why benchmarks can mislead)
- A single-burst benchmark starts the pool cold — the first wave pays the full handshake cost just like DirectConnection.
- The pool's advantage only shows up across multiple waves of traffic, where connections from wave 1 are reused in waves 2–N.
- Benchmarked with 5 waves of 100 concurrent queries: SharedPool was **1.8× faster** and used **2.6× less memory** than DirectConnection.
- With the pool pre-warmed (steady state, simulating a running server): **7.9× faster** and **20× less memory** than DirectConnection.
- Rule of thumb: connection pools are designed for long-running servers, not scripts or one-shot queries.

## Thread Pooling vs Connection Pooling
- Same class of problem (reuse expensive resources), different resource.
- **Thread pool** — manages OS threads (~1MB each). Throttles how many requests are handled simultaneously.
- **Connection pool** — manages DB connections. Throttles how many simultaneous DB queries run.
- In traditional Java/Python: both pools are needed. Thread pool limits concurrent requests; connection pool limits concurrent DB usage.
- A thread blocked on I/O is wasted — it's alive but doing nothing. With a thread pool of 20, the 21st request queues even if all 20 threads are just waiting on DB responses.

## Go Concurrency vs Node.js vs Java
- **Go**: goroutines are ~2KB, scheduled by the runtime across OS threads. Blocking a goroutine on I/O parks it and runs another — no wasted OS thread. No thread pool needed. True parallelism for CPU-bound work across cores.
- **Node.js**: single-threaded event loop. I/O is non-blocking — thread moves to next request while waiting. Efficient for I/O-bound work. CPU-heavy work blocks the entire event loop and stalls all other requests.
- **Java (pre-21)**: one OS thread per request. Thread pool needed. Threads waste time blocked on I/O.
- **Java 21+**: virtual threads (like goroutines) — same lightweight model, no thread pool needed.
- **For GopherSeek specifically**: Go wins over Node.js because BM25 scoring is CPU-bound. Node would block the event loop during scoring; Go runs it in parallel across cores.

## 3-Tier Architecture
- **Presentation tier**: clients, browsers, mobile apps.
- **Application tier**: backend server.
- **Data tier**: database.
- Connection pool lives inside the application tier — it's not a separate component unless you use an external pooler like PgBouncer, which adds a proxy tier.

## Kafka (where it fits)
- Kafka is for **server ↔ server** communication, not client ↔ server.
- Solves: multiple downstream services need to react to the same event independently and asynchronously.
- Example: driver location update → Kafka topic → ETA service, surge engine, rider push service all consume independently.
- Polling/SSE handles the last mile (server → client). Kafka handles the highway before that (service → service).
- Overkill for single-service apps with no downstream consumers.

## Browser Caching for GET requests
- Not automatic — browser only caches if the server sends explicit `Cache-Control` headers.
- Cache key is always the **full URL including query params**. Not configurable.
- For location-based search, `lat` and `long` change with every movement — same URL rarely repeats, so browser cache rarely hits.
- Coordinate snapping (rounding lat/long to 2 decimal places) makes URLs repeat more often and enables cache hits — but this is client-side logic, not cache configuration.
- Server-side caching (Redis, keyed on geohash + query text) is more effective for this use case.

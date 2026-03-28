// Shown when the backend is unreachable — gives a realistic preview of the UI.
const MOCK_RESULTS = [
  { id: 1,  score: 0.9241, text: "The backend service was rewritten in Go to improve concurrency and reduce memory usage." },
  { id: 4,  score: 0.8873, text: "The API gateway routes requests to different microservices based on path rules." },
  { id: 8,  score: 0.8102, text: "The authentication service validates tokens before processing API requests." },
  { id: 10, score: 0.7654, text: "The engineering team added rate limiting to prevent API abuse." },
  { id: 11, score: 0.7213, text: "A query optimizer can significantly improve database performance." },
  { id: 2,  score: 0.6891, text: "Engineers discussed adding caching to reduce repeated database queries." },
  { id: 9,  score: 0.6340, text: "A monitoring dashboard showed CPU spikes during heavy traffic." },
  { id: 7,  score: 0.5987, text: "Developers often use Docker containers to isolate application environments." },
  { id: 19, score: 0.5421, text: "The developer added pagination to reduce the load on database queries." },
  { id: 16, score: 0.4892, text: "A slow query caused latency in the product catalog service." },
]

export default MOCK_RESULTS

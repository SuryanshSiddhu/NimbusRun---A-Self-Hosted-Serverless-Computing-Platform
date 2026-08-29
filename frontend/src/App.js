// frontend/src/App.js — minimal read-only status page
import React, { useState, useEffect } from 'react';

const API = process.env.REACT_APP_API_URL || 'http://localhost:8080';

function App() {
  const [functions, setFunctions] = useState([]);
  const [workers, setWorkers] = useState([]);
  const [invocations, setInvocations] = useState({});
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState('functions');

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, []);

  async function fetchData() {
    try {
      const [fnRes, wrkRes] = await Promise.all([
        fetch(`${API}/functions`).catch(() => ({ ok: false })),
        fetch(`${API}/workers`).catch(() => ({ ok: false })),
      ]);

      if (fnRes.ok) setFunctions(await fnRes.json());
      if (wrkRes.ok) setWorkers(await wrkRes.json());
    } catch (e) {
      console.error('Failed to fetch data', e);
    } finally {
      setLoading(false);
    }
  }

  async function fetchInvocations(fnId) {
    if (invocations[fnId]) return;
    try {
      const res = await fetch(`${API}/functions/${fnId}/invocations`);
      if (res.ok) {
        const data = await res.json();
        setInvocations(prev => ({ ...prev, [fnId]: data }));
      }
    } catch (e) {
      console.error('Failed to fetch invocations', e);
    }
  }

  if (loading) return <div className="loading">Loading NimbusRun Dashboard...</div>;

  const healthyWorkers = workers.filter(w => w.status === 'HEALTHY').length;

  return (
    <div className="app">
      <header className="header">
        <h1>NimbusRun</h1>
        <p className="subtitle">Serverless Execution Engine</p>
        <div className="stats-row">
          <StatTile label="Functions" value={functions.length} />
          <StatTile label="Workers Online" value={healthyWorkers} />
          <StatTile label="Workers Total" value={workers.length} />
        </div>
      </header>

      <nav className="tabs">
        <button className={tab === 'functions' ? 'active' : ''} onClick={() => setTab('functions')}>Functions</button>
        <button className={tab === 'invocations' ? 'active' : ''} onClick={() => setTab('invocations')}>Invocations</button>
        <button className={tab === 'workers' ? 'active' : ''} onClick={() => setTab('workers')}>Workers</button>
      </nav>

      <main>
        {tab === 'functions' && (
          <section>
            {functions.length === 0 ? (
              <p className="empty">No functions deployed yet.</p>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Memory</th>
                    <th>CPU</th>
                    <th>Timeout</th>
                    <th>Max Concurrency</th>
                    <th>Active Version</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {functions.map(fn => (
                    <tr key={fn.id}>
                      <td>{fn.name}</td>
                      <td>{fn.memory_limit}MB</td>
                      <td>{fn.cpu_limit}m</td>
                      <td>{fn.timeout}s</td>
                      <td>{fn.max_concurrency}</td>
                      <td>{fn.active_version_id ? fn.active_version_id.slice(0, 8) : '—'}</td>
                      <td>{new Date(fn.created_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>
        )}

        {tab === 'invocations' && (
          <section>
            {functions.length === 0 ? (
              <p className="empty">No functions to show invocations for.</p>
            ) : (
              functions.map(fn => {
                fetchInvocations(fn.id);
                const invs = invocations[fn.id] || [];
                return (
                  <div key={fn.id} className="function-section">
                    <h3>{fn.name} — Recent Invocations</h3>
                    {invs.length === 0 ? (
                      <p className="empty-sm">No invocations yet.</p>
                    ) : (
                      <table className="data-table">
                        <thead>
                          <tr>
                            <th>ID</th>
                            <th>Status</th>
                            <th>Duration</th>
                            <th>Cold Start</th>
                            <th>Retries</th>
                            <th>Created</th>
                          </tr>
                        </thead>
                        <tbody>
                          {invs.map(inv => (
                            <tr key={inv.id}>
                              <td className="mono">{inv.id.slice(0, 8)}</td>
                              <td><StatusBadge status={inv.status} /></td>
                              <td>{inv.duration_ms ? `${inv.duration_ms}ms` : '—'}</td>
                              <td>{inv.cold_start ? 'Yes' : 'No'}</td>
                              <td>{inv.retry_count}</td>
                              <td>{new Date(inv.created_at).toLocaleString()}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                );
              })
            )}
          </section>
        )}

        {tab === 'workers' && (
          <section>
            {workers.length === 0 ? (
              <p className="empty">No workers registered.</p>
            ) : (
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Worker ID</th>
                    <th>Hostname</th>
                    <th>Status</th>
                    <th>CPU Capacity</th>
                    <th>Memory Capacity</th>
                    <th>Last Heartbeat</th>
                  </tr>
                </thead>
                <tbody>
                  {workers.map(w => (
                    <tr key={w.id}>
                      <td className="mono">{w.worker_id}</td>
                      <td>{w.hostname}</td>
                      <td><StatusBadge status={w.status} /></td>
                      <td>{w.cpu_capacity}m</td>
                      <td>{w.memory_capacity}MB</td>
                      <td>{new Date(w.last_heartbeat).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>
        )}
      </main>
    </div>
  );
}

function StatTile({ label, value }) {
  return (
    <div className="stat-tile">
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  );
}

function StatusBadge({ status }) {
  const colors = {
    HEALTHY: '#22c55e', UNHEALTHY: '#ef4444', DRAINING: '#f59e0b',
    READY: '#22c55e', BUILDING: '#f59e0b', QUEUED: '#94a3b8',
    FAILED: '#ef4444', SUCCESS: '#22c55e', PENDING: '#94a3b8',
    RUNNING: '#3b82f6', RETRYING: '#f59e0b', TIMEOUT: '#ef4444',
  };
  const color = colors[status] || '#94a3b8';
  return (
    <span style={{ color, fontWeight: 600, fontSize: '0.85em' }}>
      {status}
    </span>
  );
}

export default App;

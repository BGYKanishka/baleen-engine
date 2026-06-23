import { useState, useEffect } from 'react';
import { LedgerEntry } from '../types';

export default function LedgerTab({ port, token }: { port: number, token: string }) {
  const [history, setHistory] = useState<LedgerEntry[]>([]);

  const fetchLedger = () => {
    fetch(`http://127.0.0.1:${port}/api/ledger`, { headers: { Authorization: `Bearer ${token}` } })
      .then(res => res.json())
      .then(data => setHistory(data || []))
      .catch(() => setHistory([]));
  };

  useEffect(() => {
    fetchLedger();
    const interval = setInterval(fetchLedger, 3000);
    return () => clearInterval(interval);
  }, [port, token]);

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
      <table className="w-full text-left">
        <thead className="bg-gray-50 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm">
          <tr>
            <th className="px-4 py-3">Time</th>
            <th className="px-4 py-3">Direction</th>
            <th className="px-4 py-3">Image</th>
            <th className="px-4 py-3">Peer</th>
            <th className="px-4 py-3">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {history.length === 0 && (
            <tr><td colSpan={5} className="text-center py-8 text-gray-500">No ledger entries found.</td></tr>
          )}
          {history.map((record, i) => (
            <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-700 text-sm">
              <td className="px-4 py-3 text-gray-600 dark:text-gray-400">{new Date(record.timestamp).toLocaleString()}</td>
              <td className="px-4 py-3 capitalize">{record.direction}</td>
              <td className="px-4 py-3 font-medium font-mono text-gray-900 dark:text-gray-200">{record.image}</td>
              <td className="px-4 py-3 font-mono text-gray-800 dark:text-gray-300">{record.peer || 'Unknown'}</td>
              <td className="px-4 py-3">
                <span className={
                  record.status === 'Completed' ? 'text-green-600 dark:text-green-400' : 
                  record.status === 'Pending' ? 'text-yellow-600 dark:text-yellow-400' : 
                  'text-red-600 dark:text-red-400'
                }>
                  {record.status}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
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
    <div className="bg-white/50 dark:bg-white/5 border border-gray-200 dark:border-gray-800 rounded-xl overflow-x-auto shadow-sm">
      <table className="w-full text-left">
        <thead className="bg-gray-50/50 dark:bg-black/20 text-gray-700 dark:text-gray-300 text-sm">
          <tr>
            <th className="px-4 py-3 whitespace-nowrap">Time</th>
            <th className="px-4 py-3 whitespace-nowrap">Direction</th>
            <th className="px-4 py-3 whitespace-nowrap">Image</th>
            <th className="px-4 py-3 whitespace-nowrap">Peer</th>
            <th className="px-4 py-3 min-w-[300px]">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-800/50">
          {history.length === 0 && (
            <tr><td colSpan={5} className="text-center py-8 text-gray-500">No ledger entries found.</td></tr>
          )}
          {history.map((record, i) => (
            <tr key={i} className="hover:bg-gray-50/50 dark:hover:bg-white/5 transition-colors text-sm">
              <td className="px-4 py-3 text-gray-600 dark:text-gray-400 whitespace-nowrap">{new Date(record.timestamp).toLocaleString()}</td>
              <td className="px-4 py-3 capitalize whitespace-nowrap">{record.direction}</td>
              <td className="px-4 py-3 font-medium font-mono text-gray-900 dark:text-gray-200 whitespace-nowrap">{record.image}</td>
              <td className="px-4 py-3 font-mono text-gray-800 dark:text-gray-300 whitespace-nowrap">{record.peer || record.author || 'Unknown'}</td>
              <td className="px-4 py-3 min-w-[300px]">
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
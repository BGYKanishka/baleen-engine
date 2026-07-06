import { useState, useEffect } from 'react';
import { TransferStream } from '../types';

export default function TransfersTab({ port, token }: { port: number; token: string }) {
  const [transfers, setTransfers] = useState<TransferStream[]>([]);

  useEffect(() => {
    let cancelled = false;

    const poll = async () => {
      try {
        const res = await fetch(`http://127.0.0.1:${port}/api/stream`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          const data: TransferStream[] = await res.json();
          if (!cancelled) setTransfers(data ?? []);
        }
      } catch {
        // daemon not ready yet, will retry
      }
    };

    poll(); // immediate first fetch
    const interval = setInterval(poll, 500);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [port, token]);

  function statusColor(status: string) {
    switch (status) {
      case 'completed':            return 'text-green-600 dark:text-green-400';
      case 'failed':               return 'text-red-600 dark:text-red-400';
      case 'rejected':             return 'text-red-600 dark:text-red-400';
      case 'waiting for approval': return 'text-yellow-600 dark:text-yellow-400';
      case 'pruning':              return 'text-yellow-600 dark:text-yellow-400';
      default:                     return 'text-gray-600 dark:text-gray-400';
    }
  }

  function barColor(status: string) {
    switch (status) {
      case 'completed':            return 'bg-green-500';
      case 'failed':               return 'bg-red-500';
      case 'rejected':             return 'bg-red-500';
      case 'waiting for approval':
      case 'pruning':              return 'bg-yellow-500';
      default:                     return 'bg-blue-600';
    }
  }

  return (
    <div className="space-y-4">
      {transfers.length === 0 ? (
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 p-10 rounded-lg flex flex-col items-center justify-center text-center">
          <svg className="w-12 h-12 text-gray-300 dark:text-gray-600 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          <p className="text-gray-600 dark:text-gray-300 font-medium">No active transfers</p>
          <p className="text-sm text-gray-400 dark:text-gray-500 mt-1">Transfers you initiate or receive will appear here.</p>
        </div>
      ) : (
        transfers.map((t) => (
          <div
            key={`${t.image}-${t.peer}`}
            className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 p-5 rounded-lg flex flex-col gap-3 shadow-sm hover:shadow-md transition-shadow"
          >
            <div className="flex justify-between items-start">
              <div className="flex items-center gap-3">
                <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${t.direction === 'push' ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-800' : 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-400 border border-teal-200 dark:border-teal-800'}`}>
                  {t.direction}
                </span>
                <span className="font-mono text-sm text-gray-900 dark:text-gray-100 font-medium">{t.image}</span>
              </div>
              {t.speed && t.speed !== "0 B/s" && (
                <span className="text-xs text-gray-500 dark:text-gray-400 font-mono bg-gray-50 dark:bg-gray-900/50 px-2 py-1 rounded border border-gray-100 dark:border-gray-700">
                  {t.speed}
                </span>
              )}
            </div>

            <div className="w-full bg-gray-100 dark:bg-gray-700/50 rounded-full h-2 overflow-hidden">
              <div
                className={`h-full transition-all duration-500 ease-out ${barColor(t.status)}`}
                style={{ width: `${Math.min(100, Math.max(0, t.progress))}%` }}
              />
            </div>

            <div className="flex justify-between items-center text-xs">
              <div className="flex items-center gap-1.5 text-gray-500 dark:text-gray-400">
                <span className="uppercase tracking-wider font-medium text-[10px]">Target</span>
                <span className="font-medium text-gray-700 dark:text-gray-300">{t.peer}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className={`capitalize font-medium ${statusColor(t.status)}`}>
                  {t.status}
                </span>
                <span className="text-gray-600 dark:text-gray-400 font-medium tabular-nums">
                  {t.progress.toFixed(1)}%
                </span>
              </div>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
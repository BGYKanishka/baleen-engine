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
      case 'completed':            return 'text-green-400';
      case 'failed':               return 'text-red-400';
      case 'rejected':             return 'text-red-400';
      case 'waiting for approval': return 'text-yellow-400';
      case 'pruning':              return 'text-yellow-400';
      default:                     return 'text-gray-400';
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
        <div className="bg-gray-800 border border-gray-700 p-8 rounded text-center text-gray-500">
          No active transfers.
        </div>
      ) : (
        transfers.map((t) => (
          <div
            key={`${t.image}-${t.peer}`}
            className="bg-gray-800 border border-gray-700 p-4 rounded-lg flex flex-col gap-2"
          >
            <div className="flex justify-between items-center">
              <span className="font-medium flex items-center gap-2">
                {t.direction === 'push' ? '⬆️ Push' : '⬇️ Pull'}
                {' : '}
                <span className="font-mono text-blue-400">{t.image}</span>
              </span>
              <span className="text-sm text-gray-400 font-mono">{t.speed}</span>
            </div>

            <div className="w-full bg-gray-900 rounded-full h-2.5 border border-gray-700">
              <div
                className={`h-2.5 rounded-full transition-all duration-300 ${barColor(t.status)}`}
                style={{ width: `${Math.min(100, Math.max(0, t.progress))}%` }}
              />
            </div>

            <div className="flex justify-between text-xs">
              <span className="text-gray-400">Peer: {t.peer}</span>
              <span className={`capitalize font-medium ${statusColor(t.status)}`}>
                {t.progress.toFixed(1)}% — {t.status}
              </span>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
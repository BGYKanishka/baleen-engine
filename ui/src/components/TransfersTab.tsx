import { useState, useEffect } from 'react';
import { TransferStream } from '../types';

export default function TransfersTab({ port, token }: { port: number; token: string }) {
  const [transfers, setTransfers] = useState<TransferStream[]>([]);
  // Transfer waiting for cancel confirmation (image-peer).
  const [confirmCancelKey, setConfirmCancelKey] = useState<string | null>(null);
  // Message shown on backend rejection (e.g., wrong side resumes).
  const [controlMsg, setControlMsg] = useState<string | null>(null);

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

    poll();
    const interval = setInterval(poll, 500);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [port, token]);

  /** Send control action. Shows toast on 409 ownership violation. */
  const handleControl = async (t: TransferStream, action: 'pause' | 'resume' | 'cancel') => {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/api/transfer/${action}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ image: t.image, peer: t.peer }),
      });

      if (res.status === 409) {
        // Other side owns the pause. Show server message.
        const text = await res.text();
        showControlMsg(text.trim() || 'Action not allowed right now.');
      } else if (res.ok && action === 'cancel') {
        // Remove immediately, don't wait for hub poll.
        setTransfers(prev => prev.filter(tr => tr.image !== t.image || tr.peer !== t.peer));
      }
    } catch (e) {
      console.error('failed to send control action', e);
    }
  };

  const showControlMsg = (msg: string) => {
    setControlMsg(msg);
    setTimeout(() => setControlMsg(null), 4000);
  };

  const transferKey = (t: TransferStream) => `${t.image}-${t.peer}`;

  function statusColor(status: string) {
    switch (status) {
      case 'completed': return 'text-green-600 dark:text-green-400';
      case 'failed': return 'text-red-600 dark:text-red-400';
      case 'rejected': return 'text-red-600 dark:text-red-400';
      case 'cancelled': return 'text-orange-600 dark:text-orange-400';
      case 'paused': return 'text-blue-600 dark:text-blue-400';
      case 'waiting for approval': return 'text-yellow-600 dark:text-yellow-400';
      case 'pruning': return 'text-yellow-600 dark:text-yellow-400';
      default: return 'text-gray-600 dark:text-gray-400';
    }
  }

  function barColor(status: string) {
    switch (status) {
      case 'completed': return 'bg-green-500';
      case 'failed': return 'bg-red-500';
      case 'rejected': return 'bg-red-500';
      case 'cancelled': return 'bg-orange-500';
      case 'paused': return 'bg-blue-400';
      case 'waiting for approval':
      case 'pruning': return 'bg-yellow-500';
      default: return 'bg-blue-600';
    }
  }

  const activeStatuses = ['transferring', 'paused', 'pruning', 'waiting for approval'];

  return (
    <div className="space-y-4">
      {/* Toast: ownership violation message  */}
      {controlMsg && (
        <div className="fixed bottom-5 right-5 z-50 flex items-center gap-2 bg-orange-50 dark:bg-orange-900/40 border border-orange-200 dark:border-orange-700 text-orange-800 dark:text-orange-300 px-4 py-3 rounded-lg shadow-lg text-sm max-w-xs animate-fade-in">
          <svg className="w-4 h-4 shrink-0 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
              d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
          </svg>
          <span>{controlMsg}</span>
        </div>
      )}

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
            key={transferKey(t)}
            className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 p-5 rounded-lg flex flex-col gap-3 shadow-sm hover:shadow-md transition-shadow"
          >
            <div className="flex justify-between items-start">
              <div className="flex items-center gap-3">
                <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${t.direction === 'push' ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-800' : 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-400 border border-teal-200 dark:border-teal-800'}`}>
                  {t.direction}
                </span>
                <span className="font-mono text-sm text-gray-900 dark:text-gray-100 font-medium">{t.image}</span>
              </div>
              {t.speed && t.speed !== '0 B/s' && (
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

            {/* ── Action buttons for active transfers ──────────────────────── */}
            {activeStatuses.includes(t.status) && (
              <div className="flex items-center gap-2 mt-1 pt-3 border-t border-gray-100 dark:border-gray-700">

                {/* Pause / Resume */}
                {t.status !== 'waiting for approval' && t.status !== 'pruning' && (
                  t.status === 'paused' ? (
                    <button
                      id={`resume-${transferKey(t)}`}
                      onClick={() => handleControl(t, 'resume')}
                      className="px-3 py-1 bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-400 text-xs font-semibold rounded hover:bg-green-100 dark:hover:bg-green-900/50 transition-colors"
                    >
                      Resume
                    </button>
                  ) : (
                    <button
                      id={`pause-${transferKey(t)}`}
                      onClick={() => handleControl(t, 'pause')}
                      className="px-3 py-1 bg-yellow-50 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400 text-xs font-semibold rounded hover:bg-yellow-100 dark:hover:bg-yellow-900/50 transition-colors"
                    >
                      Pause
                    </button>
                  )
                )}

                {/* Cancel — with inline confirmation */}
                <div className="ml-auto flex items-center gap-2">
                  {confirmCancelKey === transferKey(t) ? (
                    <>
                      <span className="text-xs text-gray-500 dark:text-gray-400">Confirm cancel?</span>
                      <button
                        id={`cancel-confirm-${transferKey(t)}`}
                        onClick={async () => {
                          setConfirmCancelKey(null);
                          await handleControl(t, 'cancel');
                        }}
                        className="px-3 py-1 bg-red-600 text-white text-xs font-semibold rounded hover:bg-red-700 transition-colors"
                      >
                        Yes, cancel
                      </button>
                      <button
                        id={`cancel-dismiss-${transferKey(t)}`}
                        onClick={() => setConfirmCancelKey(null)}
                        className="px-3 py-1 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs font-semibold rounded hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
                      >
                        No
                      </button>
                    </>
                  ) : (
                    <button
                      id={`cancel-${transferKey(t)}`}
                      onClick={() => setConfirmCancelKey(transferKey(t))}
                      className="px-3 py-1 bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400 text-xs font-semibold rounded hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors"
                    >
                      Cancel
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        ))
      )}
    </div>
  );
}
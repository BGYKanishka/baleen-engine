import { useState, useEffect, useCallback } from 'react';

interface PendingApproval {
  image: string;
  size: number;
  hash: string;
  author: string;
  image_arch: string;
  layers: string[];
}

function formatBytes(bytes: number): string {
  const gb = bytes / (1024 ** 3);
  if (gb >= 1) return `${gb.toFixed(2)} GB`;
  const mb = bytes / (1024 ** 2);
  if (mb >= 1) return `${mb.toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

export default function ApprovalNotification({ port, token }: { port: number; token: string }) {
  const [pending, setPending] = useState<PendingApproval | null>(null);
  const [acting, setActing] = useState<'approve' | 'reject' | null>(null);

  useEffect(() => {
    let cancelled = false;

    const poll = async () => {
      try {
        const res = await fetch(`http://127.0.0.1:${port}/api/pending`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (cancelled) return;
        if (res.status === 200) {
          setPending(await res.json());
        } else {
          setPending(null);
        }
      } catch {
        if (!cancelled) setPending(null);
      }
    };

    poll();
    const id = setInterval(poll, 500);
    return () => { cancelled = true; clearInterval(id); };
  }, [port, token]);

  const decide = useCallback(async (action: 'approve' | 'reject') => {
    setActing(action);
    try {
      await fetch(`http://127.0.0.1:${port}/api/${action}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      });
    } catch { }
    setActing(null);
    setPending(null);
  }, [port, token]);

  if (!pending) return null;

  return (
    <div className="fixed bottom-6 right-6 z-50 w-[340px] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-xl overflow-hidden flex flex-col">
    {/* Top bar */}
    <div className="flex items-center gap-2.5 px-4 py-3 bg-gray-50 dark:bg-gray-800/50 border-b border-gray-200 dark:border-gray-700">
      <span className="relative flex h-2.5 w-2.5">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
        <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-blue-500" />
      </span>
      <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 tracking-wide">Incoming Transfer Request</h3>
    </div>

    {/* Body */}
    <div className="p-5 space-y-4">
      <div>
        <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1.5 font-medium">Image</p>
        <div className="bg-gray-50 dark:bg-gray-900/50 px-3 py-2.5 rounded-md border border-gray-200 dark:border-gray-700">
          <p className="font-mono text-sm text-gray-900 dark:text-gray-200 break-all leading-snug">
            {pending.image}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 text-sm">
        <div>
          <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1 font-medium">From</p>
          <p className="text-gray-900 dark:text-gray-200 font-medium truncate" title={pending.author}>{pending.author}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1 font-medium">Size</p>
          <p className="text-gray-900 dark:text-gray-200 font-medium">{formatBytes(pending.size)}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1 font-medium">Arch</p>
          <p className="text-gray-900 dark:text-gray-200 font-mono text-xs mt-0.5">{pending.image_arch || 'unknown'}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1 font-medium">Layers</p>
          <p className="text-gray-900 dark:text-gray-200 font-medium">{pending.layers?.length ?? 0}</p>
        </div>
      </div>

      <div className="flex gap-3 pt-2">
        <button
          onClick={() => decide('reject')}
          disabled={acting !== null}
          className="flex-1 py-2 rounded-md bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm font-medium border border-gray-300 dark:border-gray-600 transition-colors disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-700"
        >
          {acting === 'reject' ? 'Rejecting...' : 'Reject'}
        </button>
        <button
          onClick={() => decide('approve')}
          disabled={acting !== null}
          className="flex-1 py-2 rounded-md bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium transition-colors disabled:opacity-50 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 dark:focus:ring-offset-gray-800"
        >
          {acting === 'approve' ? 'Approving...' : 'Approve'}
        </button>
      </div>
    </div>
  </div>
  );
}
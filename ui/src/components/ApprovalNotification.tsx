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
    } catch {}
    setActing(null);
    setPending(null);
  }, [port, token]);

  if (!pending) return null;

  return (
    <div className="fixed bottom-5 right-5 z-50 w-80 bg-gray-800 border border-yellow-500/50 rounded-lg shadow-2xl shadow-black/60 overflow-hidden">
      {/* Top bar */}
      <div className="flex items-center gap-2 px-3 py-2 bg-yellow-500/10 border-b border-yellow-500/30">
        <span className="relative flex h-2 w-2">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75" />
          <span className="relative inline-flex rounded-full h-2 w-2 bg-yellow-400" />
        </span>
        <span className="text-yellow-400 text-xs font-semibold uppercase tracking-wide">
          Incoming Transfer
        </span>
      </div>

      {/* Body */}
      <div className="p-3 space-y-3">
        <div>
          <p className="text-xs text-gray-500 uppercase tracking-wider mb-0.5">Image</p>
          <p className="font-mono text-blue-400 text-sm font-medium break-all leading-snug">
            {pending.image}
          </p>
        </div>

        <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
          <div>
            <p className="text-gray-500 uppercase tracking-wider mb-0.5">From</p>
            <p className="text-gray-200 font-medium truncate">{pending.author}</p>
          </div>
          <div>
            <p className="text-gray-500 uppercase tracking-wider mb-0.5">Size</p>
            <p className="text-gray-200 font-medium">{formatBytes(pending.size)}</p>
          </div>
          <div>
            <p className="text-gray-500 uppercase tracking-wider mb-0.5">Arch</p>
            <p className="text-gray-200 font-mono">{pending.image_arch || 'unknown'}</p>
          </div>
          <div>
            <p className="text-gray-500 uppercase tracking-wider mb-0.5">Layers</p>
            <p className="text-gray-200 font-medium">{pending.layers?.length ?? 0}</p>
          </div>
        </div>

        <p className="font-mono text-xs text-gray-500 bg-gray-900 px-2 py-1 rounded border border-gray-700 truncate">
          {pending.hash.slice(0, 16)}…
        </p>

        <div className="flex gap-2 pt-0.5">
          <button
            onClick={() => decide('approve')}
            disabled={acting !== null}
            className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded bg-green-600 hover:bg-green-500 active:bg-green-700 text-white text-xs font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {acting === 'approve'
              ? <svg className="animate-spin h-3 w-3" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/></svg>
              : '✓'}
            Approve
          </button>
          <button
            onClick={() => decide('reject')}
            disabled={acting !== null}
            className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded bg-gray-700 hover:bg-red-800 active:bg-red-900 text-gray-200 hover:text-white text-xs font-semibold border border-gray-600 hover:border-red-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {acting === 'reject'
              ? <svg className="animate-spin h-3 w-3" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/></svg>
              : '✕'}
            Reject
          </button>
        </div>
      </div>
    </div>
  );
}
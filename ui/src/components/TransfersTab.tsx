import { useState, useEffect } from 'react';
import { TransferStream } from '../types';

export default function TransfersTab({ port, token }: { port: number, token: string }) {
  const [transfers, setTransfers] = useState<TransferStream[]>([]);

  useEffect(() => {
    const source = new EventSource(`http://127.0.0.1:${port}/api/stream?token=${token}`);
    
    source.onmessage = (event) => {
      try {
        const data: TransferStream = JSON.parse(event.data);
        setTransfers(prev => {
          const filtered = prev.filter(t => t.image !== data.image || t.peer !== data.peer);
          return [data, ...filtered].slice(0, 10);
        });
      } catch (e) {
        // parsing error
      }
    };
    return () => source.close();
  }, [port, token]);

  return (
    <div className="space-y-4">
      {transfers.length === 0 ? (
        <div className="bg-gray-800 border border-gray-700 p-8 rounded text-center text-gray-500">
          No active transfers.
        </div>
      ) : (
        transfers.map((t, i) => (
          <div key={i} className="bg-gray-800 border border-gray-700 p-4 rounded-lg flex flex-col gap-2">
            <div className="flex justify-between items-center">
              <span className="font-medium flex items-center gap-2">
                {t.direction === 'push' ? '⬆️ Push' : '⬇️ Pull'} : <span className="font-mono text-blue-400">{t.image}</span>
              </span>
              <span className="text-sm text-gray-400 font-mono">{t.speed}</span>
            </div>
            <div className="w-full bg-gray-900 rounded-full h-2.5 border border-gray-700">
              <div className="bg-blue-600 h-2.5 rounded-full transition-all duration-300" style={{ width: `${t.progress}%` }}></div>
            </div>
            <div className="flex justify-between text-xs text-gray-400">
              <span>Peer: {t.peer}</span>
              <span className="capitalize">{t.progress}% - {t.status}</span>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
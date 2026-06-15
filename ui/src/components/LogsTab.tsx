import { useEffect, useRef } from 'react';

export default function LogsTab({ logs }: { logs: string[] }) {
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  return (
    <div className="bg-black border border-gray-700 rounded-lg h-full overflow-hidden flex flex-col">
      <div className="bg-gray-800 px-4 py-2 border-b border-gray-700 flex justify-between items-center">
        <span className="text-sm font-semibold">Daemon Process Logs</span>
        <span className="text-xs text-gray-400">Live Output</span>
      </div>
      <div className="flex-1 overflow-auto p-4 font-mono text-xs space-y-1">
        {logs.length === 0 ? (
          <span className="text-gray-500">Awaiting log output...</span>
        ) : (
          logs.map((log, i) => {
            const isErr = log.includes('[STDERR]') || log.includes('[ERROR]');
            return (
              <div key={i} className={`whitespace-pre-wrap break-all ${isErr ? 'text-red-400' : 'text-green-400'}`}>
                {log}
              </div>
            );
          })
        )}
        <div ref={endRef} />
      </div>
    </div>
  );
}
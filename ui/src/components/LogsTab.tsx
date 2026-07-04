import { useEffect, useRef, useState } from 'react';

export default function LogsTab({ logs }: { logs: string[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [isAutoScroll, setIsAutoScroll] = useState(true);

  const handleScroll = () => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    // If the user scrolls up, turn off auto-scroll. Turn it back on if they scroll near the bottom.
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    setIsAutoScroll(isAtBottom);
  };

  useEffect(() => {
    if (isAutoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [logs, isAutoScroll]);

  return (
    <div className="bg-gray-50 dark:bg-black border border-gray-200 dark:border-gray-700 rounded-lg h-full overflow-hidden flex flex-col">
      <div className="bg-gray-100 dark:bg-gray-800 px-4 py-2 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
        <span className="text-sm font-semibold text-gray-900 dark:text-white">Daemon Process Logs</span>
        <span className="text-xs text-gray-500 dark:text-gray-400">
          {isAutoScroll ? 'Live Output (Auto-scrolling)' : 'Auto-scroll Paused'}
        </span>
      </div>
      <div 
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-auto p-4 font-mono text-xs space-y-1"
      >
        {logs.length === 0 ? (
          <span className="text-gray-500">Awaiting log output...</span>
        ) : (
          logs.map((log, i) => {
            const isErr = log.includes('[STDERR]') || log.includes('[ERROR]');
            return (
              <div key={i} className={`whitespace-pre-wrap break-all ${isErr ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`}>
                {log}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
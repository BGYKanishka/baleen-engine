import { createDockerDesktopClient } from '@docker/extension-api-client';
import { useDaemon } from './hooks/useDaemon';

import PeersTab from './components/PeersTab';
import ImagesTab from './components/ImagesTab';
import TransfersTab from './components/TransfersTab';
import LedgerTab from './components/LedgerTab';
import LogsTab from './components/LogsTab';
import ApprovalNotification from './components/ApprovalNotification';
import { useState } from 'react';

const ddClient = createDockerDesktopClient();

export default function App() {
  const { status, port, token, nodeName, logs, errorMsg, startDaemon, stopDaemon } = useDaemon(ddClient);
  const [activeTab, setActiveTab] = useState('peers');
  const [isInstalling, setIsInstalling] = useState(false);

  const handleInstallCLI = async () => {
    try {
      setIsInstalling(true);
      if (!ddClient.extension?.host) {
        throw new Error("Extension host is not available");
      }
      const result = await ddClient.extension.host.cli.exec('baleen', ['install-cli']);
      // If the command succeeds, show a success toast
      ddClient.desktopUI.toast.success("CLI installed! You can now run 'docker baleen' in your terminal.");
    } catch (e: any) {
      // e could contain e.stderr or e.message
      const errText = e.stderr || e.stdout || e.message || String(e);
      ddClient.desktopUI.toast.error(`Failed to install CLI: ${errText}`);
    } finally {
      setIsInstalling(false);
    }
  };

  // ── Checking / connecting ─────────────────────────────────────────────────
  if (status === 'checking') {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-3 bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-400">
        <svg className="animate-spin h-6 w-6 text-blue-500" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
        </svg>
        <span className="text-sm font-medium">Checking Baleen service…</span>
      </div>
    );
  }

  // Service is stopped — show a start button and instructions.
  if (status === 'stopped') {
    return (
      <div className="flex flex-col h-screen items-center justify-center gap-6 bg-white dark:bg-gray-900">
        {/* Icon */}
        <svg width="48" height="48" viewBox="0 0 24 24" fill="currentColor" className="text-gray-300 dark:text-gray-600">
          <rect x="2" y="5"    width="20" height="3" rx="1.5" />
          <rect x="2" y="10.5" width="20" height="3" rx="1.5" />
          <rect x="2" y="16"  width="20" height="3" rx="1.5" />
        </svg>

        <div className="text-center space-y-1">
          <div className="text-gray-700 dark:text-gray-300 font-semibold text-lg">Baleen is not running</div>
          <p className="text-gray-400 dark:text-gray-500 text-sm max-w-xs text-center">
            The background service is stopped. Click Start to begin sharing Docker images on your network.
          </p>
        </div>

        <button
          id="start-baleen-btn"
          onClick={startDaemon}
          className="bg-blue-600 hover:bg-blue-700 active:scale-95 text-white px-8 py-3 rounded-xl font-semibold text-base shadow-lg transition-all duration-150"
        >
          Start Baleen
        </button>
      </div>
    );
  }

  // ── Error / lost connection ───────────────────────────────────────────────
  if (status === 'error') {
    return (
      <div className="flex flex-col h-screen items-center justify-center gap-5 bg-white dark:bg-gray-900">
        <div className="text-red-500 font-bold text-xl">Service Unavailable</div>
        <div className="bg-gray-100 dark:bg-gray-800 p-4 rounded-lg text-gray-700 dark:text-gray-300 font-mono text-sm max-w-lg text-center break-all">
          {errorMsg}
        </div>
        <button
          id="restart-baleen-btn"
          onClick={startDaemon}
          className="bg-blue-600 hover:bg-blue-700 active:scale-95 text-white px-6 py-2.5 rounded-lg font-semibold transition-all duration-150"
        >
          Restart Service
        </button>

        {/* Show logs so user can see what went wrong */}
        {logs.length > 0 && (
          <details className="mt-2 w-full max-w-lg">
            <summary className="text-xs text-gray-400 cursor-pointer select-none">Show logs</summary>
            <div className="mt-2 bg-gray-100 dark:bg-gray-800 rounded p-3 font-mono text-xs text-gray-600 dark:text-gray-300 max-h-40 overflow-auto">
              {logs.map((l, i) => <div key={i}>{l}</div>)}
            </div>
          </details>
        )}
      </div>
    );
  }

  // ── Running ───────────────────────────────────────────────────────────────
  return (
    <div className="flex flex-col h-screen bg-white dark:bg-gray-900 text-gray-900 dark:text-white overflow-hidden">

      {/* Header */}
      <header className="bg-gray-50 dark:bg-gray-800 p-4 flex justify-between items-center border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
        <h1 className="text-xl font-bold flex items-center gap-3">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor" className="text-blue-600 dark:text-blue-400">
            <rect x="2" y="5"    width="20" height="3" rx="1.5" />
            <rect x="2" y="10.5" width="20" height="3" rx="1.5" />
            <rect x="2" y="16"  width="20" height="3" rx="1.5" />
          </svg>
          <span className="bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 px-3 py-1 rounded-md text-sm font-semibold shadow-sm">
            Baleen
          </span>
        </h1>

        <div className="flex items-center gap-4">
          <button
            onClick={handleInstallCLI}
            disabled={isInstalling}
            className="text-sm bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-800 dark:text-gray-200 px-3 py-1 rounded transition flex items-center gap-2"
          >
            {isInstalling ? 'Installing...' : 'Install Terminal CLI'}
          </button>
          
          {/* Running indicator */}
          <span className="flex items-center gap-2 text-sm font-medium text-green-400">
            <span className="h-2 w-2 bg-green-400 rounded-full animate-pulse" />
            Running (Port {port}) - {nodeName}
          </span>

          {/* Stop is the only control when running */}
          <button
            id="stop-baleen-btn"
            onClick={stopDaemon}
            className="text-sm border border-red-600 hover:bg-red-100 dark:hover:bg-red-900/30 text-red-600 dark:text-red-400 px-3 py-1 rounded transition"
          >
            Stop
          </button>
        </div>
      </header>

      {/* Navigation tabs */}
      <div className="flex border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-4 pt-3 flex-shrink-0 gap-1">
        {(['peers', 'images', 'transfers', 'ledger', 'logs'] as const).map((tab) => (
          <button
            key={tab}
            id={`tab-${tab}`}
            onClick={() => setActiveTab(tab)}
            className={`px-5 py-2 capitalize font-semibold rounded-t-lg transition-colors duration-200 ${
              activeTab === tab
                ? 'bg-white dark:bg-gray-900 text-blue-600 dark:text-blue-400 border-t border-l border-r border-gray-200 dark:border-gray-700'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700'
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6 bg-white dark:bg-gray-900">
        {activeTab === 'peers'     && <PeersTab     port={port!} token={token} />}
        {activeTab === 'images'    && <ImagesTab    port={port!} token={token} ddClient={ddClient} />}
        {activeTab === 'transfers' && <TransfersTab port={port!} token={token} />}
        {activeTab === 'ledger'    && <LedgerTab    port={port!} token={token} />}
        {activeTab === 'logs'      && <LogsTab logs={logs} />}
      </div>

      {/* Floating transfer approval */}
      {port && <ApprovalNotification port={port} token={token} />}
    </div>
  );
}
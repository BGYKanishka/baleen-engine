import { createDockerDesktopClient } from '@docker/extension-api-client';
import { useDaemon } from './hooks/useDaemon';

import PeersTab from './components/PeersTab';
import ImagesTab from './components/ImagesTab';
import TransfersTab from './components/TransfersTab';
import LedgerTab from './components/LedgerTab';
import LogsTab from './components/LogsTab';
import ApprovalNotification from './components/ApprovalNotification';
import NetworkSphere from './components/NetworkSphere';
import SettingsSidebar from './components/SettingsSidebar';
import UpdateBanner from './components/UpdateBanner';
import { useState } from 'react';

const ddClient = createDockerDesktopClient();

export default function App() {
  const { status, port, token, nodeName, logs, errorMsg, startDaemon, stopDaemon, setNodeName } = useDaemon(ddClient);
  const [activeTab, setActiveTab] = useState('peers');
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  // Custom node name — persisted in localStorage, falls back to daemon-assigned name
  const [customName, setCustomName] = useState<string>(
    () => localStorage.getItem('baleen-custom-node-name') ?? ''
  );
  const displayName = customName || nodeName;

  // ── Checking / stopped ─────────────────────────────────────────────────
  if (status === 'checking' || status === 'stopped') {
    const isChecking = status === 'checking';
    return (
      <div className="flex flex-col h-screen items-center justify-center gap-6 bg-transparent">
        {/* Icon */}
        <NetworkSphere className="w-64 h-64 opacity-80" />

        <div className={`text-center space-y-1 ${isChecking ? 'invisible' : ''}`}>
          <div className="text-gray-700 dark:text-gray-300 font-semibold text-lg">Baleen is not running</div>
          <p className="text-gray-400 dark:text-gray-500 text-sm max-w-xs text-center">
            The background service is stopped. Click Start to begin sharing Docker images on your network.
          </p>
        </div>

        <button
          id="start-baleen-btn"
          onClick={startDaemon}
          disabled={isChecking}
          className={`bg-blue-600 hover:bg-blue-700 active:scale-95 text-white px-8 py-3 rounded-xl font-semibold text-base shadow-lg transition-all duration-150 ${isChecking ? 'invisible' : ''}`}
        >
          Start Baleen
        </button>
      </div>
    );
  }

  // ── Error / lost connection ───────────────────────────────────────────────
  if (status === 'error') {
    return (
      <div className="flex flex-col h-screen items-center justify-center gap-5 bg-transparent">
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
    <div className="flex flex-col h-screen bg-transparent text-gray-900 dark:text-white overflow-hidden">
      {port && <UpdateBanner port={port} token={token} />}

      {/* Settings sidebar */}
      <SettingsSidebar
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
        nodeName={nodeName}
        customName={customName}
        onNameChange={(name) => {
          setCustomName(name);
          localStorage.setItem('baleen-custom-node-name', name);
        }}
        setNodeName={setNodeName}
        port={port}
        token={token}
      />

      {/* Header */}
      <header className="p-4 flex justify-between items-center border-b border-gray-200 dark:border-gray-800 flex-shrink-0">
        <h1 className="text-xl font-bold flex items-center gap-3">
          {/* Hamburger menu button */}
          <button
            id="open-settings-btn"
            onClick={() => setIsSidebarOpen(true)}
            aria-label="Open settings"
            className="flex flex-col justify-center items-center gap-[5px] w-8 h-8 rounded-md hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors duration-150"
          >
            <span className="block w-5 h-[2px] rounded-full bg-gray-600 dark:bg-gray-300" />
            <span className="block w-5 h-[2px] rounded-full bg-gray-600 dark:bg-gray-300" />
            <span className="block w-5 h-[2px] rounded-full bg-gray-600 dark:bg-gray-300" />
          </button>
          <span className="text-gray-900 dark:text-white text-base font-semibold">
            Baleen
          </span>
        </h1>

        <div className="flex items-center gap-4">
          {/* Running indicator */}
          <span className="flex items-center gap-2 text-sm font-medium text-green-400">
            <span className="h-2 w-2 bg-green-400 rounded-full animate-pulse" />
            Running - {displayName}
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
      <div className="flex border-b border-gray-200 dark:border-gray-800 px-6 pt-4 flex-shrink-0 gap-6">
        {(['peers', 'images', 'transfers', 'ledger', 'logs'] as const).map((tab) => (
          <button
            key={tab}
            id={`tab-${tab}`}
            onClick={() => setActiveTab(tab)}
            className={`px-1 py-2 capitalize font-medium text-sm transition-all duration-200 border-b-2 -mb-[1px] ${activeTab === tab
              ? 'text-blue-600 dark:text-blue-400 border-blue-500'
              : 'text-gray-500 dark:text-gray-400 border-transparent hover:text-gray-800 dark:hover:text-gray-200 hover:border-gray-300 dark:hover:border-gray-600'
              }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6 bg-transparent">
        {activeTab === 'peers' && <PeersTab port={port!} token={token} />}
        {activeTab === 'images' && <ImagesTab port={port!} token={token} ddClient={ddClient} />}
        {activeTab === 'transfers' && <TransfersTab port={port!} token={token} />}
        {activeTab === 'ledger' && <LedgerTab port={port!} token={token} />}
        {activeTab === 'logs' && <LogsTab logs={logs} />}
      </div>

      {/* Floating transfer approval */}
      {port && <ApprovalNotification port={port} token={token} />}
    </div>
  );
}
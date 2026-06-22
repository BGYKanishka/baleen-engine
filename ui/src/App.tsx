import { useState } from 'react';
import { createDockerDesktopClient } from '@docker/extension-api-client';
import { useDaemon } from './hooks/useDaemon';

import PeersTab from './components/PeersTab';
import ImagesTab from './components/ImagesTab';
import TransfersTab from './components/TransfersTab';
import LedgerTab from './components/LedgerTab';
import LogsTab from './components/LogsTab';
import ApprovalNotification from './components/ApprovalNotification';

const ddClient = createDockerDesktopClient();

export default function App() {
  const { status, port, token, logs, errorMsg, startDaemon, stopDaemon } = useDaemon(ddClient);
  const [activeTab, setActiveTab] = useState('peers');

  if (status === 'checking') {
    return <div className="flex h-screen items-center justify-center bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-400 animate-pulse">Initializing Extension...</div>;
  }

  if (status === 'error') {
    return (
      <div className="flex flex-col h-screen items-center justify-center space-y-4 bg-white dark:bg-gray-900">
        <div className="text-red-500 font-bold text-xl">Daemon Connection Lost</div>
        <div className="bg-gray-100 dark:bg-gray-800 p-4 rounded text-gray-700 dark:text-gray-300 font-mono text-sm max-w-lg text-center break-all">{errorMsg}</div>
        <button onClick={startDaemon} className="bg-blue-600 px-4 py-2 rounded">Start Daemon</button>
      </div>
    );
  }

  if (!port && status !== 'stopped') {
    return <div className="flex h-screen items-center justify-center bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-400">Booting host process...</div>;
  }

  return (
    <div className="flex flex-col h-screen bg-white dark:bg-gray-900 text-gray-900 dark:text-white overflow-hidden">
      {/* Header */}
      <header className="bg-gray-50 dark:bg-gray-800 p-4 flex justify-between items-center border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
        <h1 className="text-xl font-bold flex items-center gap-3">
          {/* Baleen layers icon */}
          <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor" className="text-blue-600 dark:text-blue-400">
            <rect x="2" y="5" width="20" height="3" rx="1.5" />
            <rect x="2" y="10.5" width="20" height="3" rx="1.5" />
            <rect x="2" y="16" width="20" height="3" rx="1.5" />
          </svg>
          <span className="bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 px-3 py-1 rounded-md text-sm font-semibold shadow-sm">Baleen</span>
        </h1>
        <div className="flex items-center gap-4">
          {status === 'running' ? (
            <span className="flex items-center gap-2 text-sm font-medium text-green-400">
              <span className="h-2 w-2 bg-green-400 rounded-full animate-pulse"></span>
              Daemon Running (Port {port})
            </span>
          ) : (
            <span className="flex items-center gap-2 text-sm font-medium text-gray-600 dark:text-gray-400">
              <span className="h-2 w-2 bg-gray-500 rounded-full"></span>
              Daemon Stopped
            </span>
          )}
          <div className="flex items-center gap-2">
            <button
              onClick={startDaemon}
              disabled={status === 'running'}
              className={`text-sm border px-3 py-1 rounded transition ${status === 'running' ? 'border-gray-300 dark:border-gray-700 text-gray-400 dark:text-gray-600 cursor-not-allowed' : 'border-green-600 hover:bg-green-100 dark:hover:bg-green-900/30 text-green-600 dark:text-green-400'}`}
            >
              Start
            </button>
            <button
              onClick={stopDaemon}
              disabled={status !== 'running'}
              className={`text-sm border px-3 py-1 rounded transition ${status !== 'running' ? 'border-gray-300 dark:border-gray-700 text-gray-400 dark:text-gray-600 cursor-not-allowed' : 'border-red-600 hover:bg-red-100 dark:hover:bg-red-900/30 text-red-600 dark:text-red-400'}`}
            >
              Stop
            </button>
          </div>
        </div>
      </header>

      {/* Navigation */}
      <div className="flex border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-4 pt-3 flex-shrink-0 gap-1">
        {['peers', 'images', 'transfers', 'ledger', 'logs'].map((tab) => (
          <button
            key={tab}
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

      {/* Content Area */}
      <div className="flex-1 overflow-auto p-6 bg-white dark:bg-gray-900">
        {status === 'stopped' ? (
          <div className="flex h-full items-center justify-center text-gray-500 font-medium">
            Daemon is stopped. Click Start to resume.
          </div>
        ) : (
          <>
            {activeTab === 'peers'     && <PeersTab port={port!} token={token} />}
            {activeTab === 'images'    && <ImagesTab port={port!} token={token} ddClient={ddClient} />}
            {activeTab === 'transfers' && <TransfersTab port={port!} token={token} />}
            {activeTab === 'ledger'    && <LedgerTab port={port!} token={token} />}
            {activeTab === 'logs'      && <LogsTab logs={logs} />}
          </>
        )}
      </div>

      {/* Floating approval notification */}
      {status === 'running' && port && (
        <ApprovalNotification port={port} token={token} />
      )}
    </div>
  );
}
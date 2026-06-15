import { useState } from 'react';
import { createDockerDesktopClient } from '@docker/extension-api-client';
import { useDaemon } from './hooks/useDaemon';

import PeersTab from './components/PeersTab';
import ImagesTab from './components/ImagesTab';
import TransfersTab from './components/TransfersTab';
import LedgerTab from './components/LedgerTab';
import LogsTab from './components/LogsTab';

const ddClient = createDockerDesktopClient();

export default function App() {
  const { status, port, token, logs, errorMsg, startDaemon } = useDaemon(ddClient);
  const [activeTab, setActiveTab] = useState('peers');

  if (status === 'checking') {
    return <div className="flex h-screen items-center justify-center text-gray-400 animate-pulse">Initializing Extension...</div>;
  }

  if (status === 'error') {
    return (
      <div className="flex flex-col h-screen items-center justify-center space-y-4">
        <div className="text-red-500 font-bold text-xl">Daemon Connection Lost</div>
        <div className="bg-gray-800 p-4 rounded text-gray-300 font-mono text-sm max-w-lg text-center break-all">{errorMsg}</div>
        <button onClick={startDaemon} className="bg-blue-600 px-4 py-2 rounded">Restart Daemon</button>
      </div>
    );
  }

  if (!port) {
    return <div className="flex h-screen items-center justify-center text-gray-400">Booting host process...</div>;
  }

  return (
    <div className="flex flex-col h-screen bg-gray-900 text-white overflow-hidden">
      {/* Header */}
      <header className="bg-gray-800 p-4 flex justify-between items-center border-b border-gray-700 flex-shrink-0">
        <h1 className="text-xl font-bold flex items-center gap-3">
          <span className="bg-blue-600 px-2 py-1 rounded text-sm shadow">Baleen Engine</span>
        </h1>
        <div className="flex items-center gap-4">
          <span className="flex items-center gap-2 text-sm font-medium text-green-400">
            <span className="h-2 w-2 bg-green-400 rounded-full animate-pulse"></span>
            Daemon Running (Port {port})
          </span>
          <button onClick={startDaemon} className="text-sm border border-gray-600 hover:bg-gray-700 px-3 py-1 rounded transition">
            Restart
          </button>
        </div>
      </header>
      
      {/* Navigation */}
      <div className="flex border-b border-gray-700 bg-gray-800 px-4 pt-3 flex-shrink-0 gap-1">
        {['peers', 'images', 'transfers', 'ledger', 'logs'].map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-5 py-2 capitalize font-semibold rounded-t-lg transition-colors duration-200 ${
              activeTab === tab 
                ? 'bg-gray-900 text-blue-400 border-t border-l border-r border-gray-700' 
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-750'
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-auto p-6 bg-gray-900">
        {activeTab === 'peers' && <PeersTab port={port} token={token} />}
        {activeTab === 'images' && <ImagesTab port={port} token={token} ddClient={ddClient} />}
        {activeTab === 'transfers' && <TransfersTab port={port} token={token} />}
        {activeTab === 'ledger' && <LedgerTab port={port} token={token} />}
        {activeTab === 'logs' && <LogsTab logs={logs} />}
      </div>
    </div>
  );
}
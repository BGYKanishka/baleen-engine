import { useState, useEffect } from 'react';
import { Peer } from '../types';

export default function PeersTab({ port, token }: { port: number, token: string }) {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [manualIp, setManualIp] = useState('');

  const fetchPeers = async () => {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/api/peers`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) setPeers(await res.json() || []);
    } catch (e) {
      console.error("Failed to fetch peers");
    }
  };

  useEffect(() => {
    fetchPeers();
    const interval = setInterval(fetchPeers, 1000); 
    return () => clearInterval(interval);
  }, [port, token]);

  const addManualPeer = async () => {
    if (!manualIp) return;
    try {
      await fetch(`http://127.0.0.1:${port}/api/peers`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ ip: manualIp })
      });
      setManualIp('');
      fetchPeers();
    } catch (e) {
      console.error("Failed to add peer");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex gap-4 items-end">
        <div className="flex-1">
          <label className="block text-sm font-medium text-gray-600 dark:text-gray-400 mb-1">Add Manual Peer (IP Address)</label>
          <input 
            type="text" 
            value={manualIp}
            onChange={(e) => setManualIp(e.target.value)}
            placeholder="192.168.1.100" 
            className="w-full bg-gray-50 dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded px-3 py-2 text-gray-900 dark:text-white focus:outline-none focus:border-blue-500"
          />
        </div>
        <button onClick={addManualPeer} className="bg-blue-600 hover:bg-blue-500 text-white px-4 py-2 rounded font-medium transition">
          Add Peer
        </button>
      </div>

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <table className="w-full text-left">
          <thead className="bg-gray-50 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm">
            <tr>
              <th className="px-4 py-3">Hostname</th>
              <th className="px-4 py-3">IP Address</th>
              <th className="px-4 py-3">Source</th>
              <th className="px-4 py-3">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
            {peers.map((peer, i) => (
              <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{peer.hostname}</td>
                <td className="px-4 py-3 font-mono text-sm text-gray-600 dark:text-gray-400">{peer.ip}</td>
                <td className="px-4 py-3">
                  <span className={`text-xs px-2 py-1 rounded ${peer.source === 'mdns' ? 'bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-300' : 'bg-gray-200 dark:bg-gray-600 text-gray-800 dark:text-gray-300'}`}>
                    {peer.source.toUpperCase()}
                  </span>
                </td>
                <td className="px-4 py-3 flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full ${peer.status === 'reachable' ? 'bg-green-500' : 'bg-red-500'}`}></span>
                  {peer.status}
                </td>
              </tr>
            ))}
            {peers.length === 0 && (
              <tr><td colSpan={4} className="text-center py-8 text-gray-500">No peers discovered yet.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
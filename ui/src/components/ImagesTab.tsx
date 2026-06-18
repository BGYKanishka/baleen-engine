import { useState, useEffect } from 'react';
import { DockerImage, Peer } from '../types';

export default function ImagesTab({ port, token, ddClient }: { port: number, token: string, ddClient: any }) {
  const [images, setImages] = useState<DockerImage[]>([]);
  const [peers, setPeers] = useState<Peer[]>([]);
  const [pushModal, setPushModal] = useState<{ isOpen: boolean, image: string }>({ isOpen: false, image: '' });
  const [selectedPeer, setSelectedPeer] = useState<string>('');

useEffect(() => {
    const fetchLocalImages = async () => {
      try {
        // Use the native Docker API instead of CLI commands
        const localImages = await ddClient.docker.listImages();
        
        const formattedImages: DockerImage[] = [];
        
        localImages.forEach((img: any) => {
          if (!img.RepoTags || img.RepoTags.length === 0) return;
          
          img.RepoTags.forEach((repoTag: string) => {
            if (repoTag === '<none>:<none>' || repoTag.startsWith('<none>')) return;
            const lastColonIndex = repoTag.lastIndexOf(':');
            const name = lastColonIndex !== -1 ? repoTag.substring(0, lastColonIndex) : repoTag;
            const tag = lastColonIndex !== -1 ? repoTag.substring(lastColonIndex + 1) : 'latest';
            
            const sizeMB = (img.Size / (1024 * 1024)).toFixed(2) + ' MB';
            
            formattedImages.push({
              name: name || '<unknown>',
              tag: tag || 'latest',
              size: sizeMB
            });
          });
        });
        
        setImages(formattedImages);
      } catch (err) {
        console.error("Failed to fetch native docker images", err);
      }
    };
    
    fetchLocalImages();
  }, [ddClient]);

  const openPushModal = async (imageName: string, imageTag: string) => {
    const fullImage = `${imageName}:${imageTag}`;
    setPushModal({ isOpen: true, image: fullImage });
    try {
      const res = await fetch(`http://127.0.0.1:${port}/api/peers`, { headers: { Authorization: `Bearer ${token}` } });
      const peerList: Peer[] = await res.json();
      const reachablePeers = peerList.filter(p => p.status === 'reachable');
      setPeers(reachablePeers);
      if (reachablePeers.length > 0) {
        setSelectedPeer(reachablePeers[0].hostname);
      } else {
        setSelectedPeer('');
      }
    } catch (e) {
      console.error("Failed to fetch peers for modal");
    }
  };

const confirmPush = async () => {
    if (!selectedPeer) {
      alert("No peer selected!");
      return;
    }
    try {
      console.log(`Sending push request for ${pushModal.image} to ${selectedPeer}...`);
      
      const res = await fetch(`http://127.0.0.1:${port}/api/push`, {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json', 
          'Authorization': `Bearer ${token}` 
        },
        body: JSON.stringify({ image: pushModal.image, peer: selectedPeer })
      });

      // check if the Go backend rejected the request
      if (!res.ok) {
        const errorText = await res.text();
        alert(`Backend rejected the push request.\nStatus: ${res.status}\nMessage: ${errorText}`);
        return;
      }
      setPushModal({ isOpen: false, image: '' });

    } catch (e: any) {
      // Handle network errors or other unexpected issues
      alert(`Network Error: Failed to reach the backend API.\nDetails: ${e.message}`);
      console.error("Push API Error:", e);
    }
  };

  return (
    <div className="relative">
      <div className="bg-gray-800 border border-gray-700 rounded-lg overflow-hidden">
        <table className="w-full text-left">
          <thead className="bg-gray-700 text-gray-300 text-sm">
            <tr>
              <th className="px-4 py-3">Image Name</th>
              <th className="px-4 py-3">Tag</th>
              <th className="px-4 py-3">Size</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-700">
            {images.map((img, i) => (
              <tr key={i} className="hover:bg-gray-750">
                <td className="px-4 py-3 font-medium">{img.name}</td>
                <td className="px-4 py-3 text-gray-400">{img.tag}</td>
                <td className="px-4 py-3 text-gray-400">{img.size}</td>
                <td className="px-4 py-3 text-right">
                  <button onClick={() => openPushModal(img.name, img.tag)} className="bg-blue-600 hover:bg-blue-500 text-sm px-3 py-1 rounded transition">
                    Push
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {pushModal.isOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-50">
          <div className="bg-gray-800 border border-gray-700 p-6 rounded-lg shadow-xl w-96">
            <h3 className="text-lg font-bold mb-4">Push Image</h3>
            <p className="text-sm text-gray-300 mb-4">Pushing <span className="font-mono text-blue-400">{pushModal.image}</span> to peer:</p>
            
            <select 
              className="w-full bg-gray-900 border border-gray-700 rounded p-2 text-white mb-6"
              value={selectedPeer}
              onChange={(e) => setSelectedPeer(e.target.value)}
            >
              <option value="" disabled>Select target peer...</option>
              {peers.map(p => (
                <option key={p.hostname} value={p.hostname}>{p.hostname} ({p.ip})</option>
              ))}
            </select>

            <div className="flex justify-end gap-3">
              <button onClick={() => setPushModal({ isOpen: false, image: '' })} className="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded">Cancel</button>
              <button onClick={confirmPush} disabled={!selectedPeer} className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-blue-800 rounded font-medium">
                Confirm Push
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
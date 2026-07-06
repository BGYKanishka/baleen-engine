import { useState, useEffect } from 'react';
import { DockerImage, Peer } from '../types';

export default function ImagesTab({ port, token, ddClient }: { port: number; token: string; ddClient: any }) {
  const [images, setImages] = useState<DockerImage[]>([]);
  const [peers, setPeers] = useState<Peer[]>([]);
  const [pushModal, setPushModal] = useState<{ isOpen: boolean; image: string }>({ isOpen: false, image: '' });
  const [selectedPeer, setSelectedPeer] = useState<string>('');
  const [buildContext, setBuildContext] = useState<string>('');
  const [pushing, setPushing] = useState(false);
  const [imageArch, setImageArch] = useState<string>('');

  useEffect(() => {
    const fetchLocalImages = async () => {
      try {
        // Use the native Docker API instead of CLI commands
        const localImages = await ddClient.docker.listImages();

        let hostArch = 'arm64';
        try {
          const infoRes = await ddClient.docker.cli.exec('info', ['--format', '{{.Architecture}}']);
          hostArch = infoRes.stdout.trim();
          if (hostArch === 'aarch64') hostArch = 'arm64';
          if (hostArch === 'x86_64') hostArch = 'amd64';
        } catch (e) {
          console.error("Failed to get host arch", e);
        }

        const uniqueIds = Array.from(new Set(localImages.map((img: any) => img.Id)));
        let archMap: Record<string, string> = {};

        if (uniqueIds.length > 0) {
           const chunkSize = 50;
           for (let i = 0; i < uniqueIds.length; i += chunkSize) {
             const chunk = uniqueIds.slice(i, i + chunkSize);
             try {
                const inspectRes = await ddClient.docker.cli.exec('inspect', chunk as string[]);
                const inspectData = JSON.parse(inspectRes.stdout);
                inspectData.forEach((data: any) => {
                   archMap[data.Id] = data.Architecture;
                });
             } catch (e) {
                console.error("Failed to inspect chunk of images", e);
             }
           }
        }

        const formattedImages: DockerImage[] = [];

        localImages.forEach((img: any) => {
          if (!img.RepoTags || img.RepoTags.length === 0) return;

          const arch = archMap[img.Id] || 'unknown';
          const isMismatch = arch !== 'unknown' && arch !== hostArch;

          img.RepoTags.forEach((repoTag: string) => {
            if (repoTag === '<none>:<none>' || repoTag.startsWith('<none>')) return;
            const lastColonIndex = repoTag.lastIndexOf(':');
            const name = lastColonIndex !== -1 ? repoTag.substring(0, lastColonIndex) : repoTag;
            const tag = lastColonIndex !== -1 ? repoTag.substring(lastColonIndex + 1) : 'latest';
            formattedImages.push({
              name: name || '<unknown>',
              tag: tag || 'latest',
              size: (img.Size / (1024 * 1024)).toFixed(2) + ' MB',
              arch,
              isMismatch,
            });
          });
        });

        setImages(formattedImages);
      } catch (err) {
        console.error('Failed to fetch native docker images', err);
      }
    };

    fetchLocalImages();
  }, [ddClient]);

  const openPushModal = async (imageName: string, imageTag: string) => {
    const fullImage = `${imageName}:${imageTag}`;
    setPushModal({ isOpen: true, image: fullImage });
    setBuildContext('');
    setImageArch('');

    try {
      const inspectRes = await ddClient.docker.cli.exec('inspect', [fullImage]);
      const inspectData = JSON.parse(inspectRes.stdout);
      if (inspectData && inspectData.length > 0) {
        setImageArch(inspectData[0].Os + '/' + inspectData[0].Architecture);
      }
    } catch (err) {
      console.error('Failed to inspect image arch', err);
    }

    try {
      const res = await fetch(`http://127.0.0.1:${port}/api/peers`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const peerList: Peer[] = await res.json();
      const reachablePeers = peerList.filter((p) => p.status === 'reachable');
      setPeers(reachablePeers);
      setSelectedPeer(reachablePeers.length > 0 ? reachablePeers[0].hostname : '');
    } catch {
      console.error('Failed to fetch peers for modal');
    }
  };

  const confirmPush = async () => {
    if (!selectedPeer) return;
    setPushing(true);
    try {
      const body: Record<string, string> = { image: pushModal.image, peer: selectedPeer };
      if (buildContext.trim()) body.buildContext = buildContext.trim();

      const res = await fetch(`http://127.0.0.1:${port}/api/push`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(body),
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
    } finally {
      setPushing(false);
    }
  };

  const selectedPeerObj = peers.find(p => p.hostname === selectedPeer);
  const hideBuildContext = selectedPeerObj && selectedPeerObj.arch && selectedPeerObj.arch !== 'unknown' && imageArch && selectedPeerObj.arch === imageArch;

  return (
    <div className="relative">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
        <table className="w-full text-left">
          <thead className="bg-gray-50 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-sm">
            <tr>
              <th className="px-4 py-3">Image Name</th>
              <th className="px-4 py-3">Tag</th>
              <th className="px-4 py-3">Size</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
            {images.map((img, i) => (
              <tr key={i} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">
                  <div className="flex items-center gap-2">
                    {img.name}
                    {img.isMismatch && (
                      <span title={`Architecture mismatch: Image is ${img.arch}`} className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-400 border border-yellow-200 dark:border-yellow-800 uppercase tracking-wider">
                        {img.arch}
                      </span>
                    )}
                  </div>
                </td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{img.tag}</td>
                <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{img.size}</td>
                <td className="px-4 py-3 text-right">
                  <button
                    onClick={() => openPushModal(img.name, img.tag)}
                    className="bg-blue-600 hover:bg-blue-500 text-white text-sm px-3 py-1 rounded transition"
                  >
                    Push
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {pushModal.isOpen && (
        <div className="fixed inset-0 bg-gray-900/50 dark:bg-black/70 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 p-6 rounded-lg shadow-xl w-[420px] space-y-5">
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Push Image</h3>

            {/* Image name */}
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-1">Image</p>
              <p className="font-mono text-blue-600 dark:text-blue-400 text-sm">{pushModal.image}</p>
            </div>

            {/* Peer selector */}
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-1">Target Peer</p>
              <select
                className="w-full bg-gray-50 dark:bg-gray-900 border border-gray-300 dark:border-gray-700 rounded p-2 text-gray-900 dark:text-white"
                value={selectedPeer}
                onChange={(e) => setSelectedPeer(e.target.value)}
              >
                <option value="" disabled>Select target peer...</option>
                {peers.map((p) => (
                  <option key={p.hostname} value={p.hostname}>
                    {p.hostname} ({p.ip})
                  </option>
                ))}
              </select>
            </div>

            {/* Build context — optional, for cross-compilation */}
            {hideBuildContext ? (
              <div className="bg-green-50 dark:bg-green-900/30 p-3 rounded border border-green-200 dark:border-green-800">
                <p className="text-sm text-green-800 dark:text-green-300">
                  <span className="font-semibold">Match!</span> Image architecture ({imageArch}) matches the destination peer. No cross-compilation needed.
                </p>
              </div>
            ) : (
              <div>
                <p className="text-xs text-gray-500 uppercase tracking-wider mb-1">
                  Build Context <span className="normal-case text-gray-600">(optional — for cross-compilation)</span>
                </p>
                <input
                  type="text"
                  placeholder="/path/to/your/project"
                  value={buildContext}
                  onChange={(e) => setBuildContext(e.target.value)}
                  className="w-full bg-gray-50 dark:bg-gray-900 border border-gray-300 dark:border-gray-700 rounded p-2 text-gray-900 dark:text-white font-mono text-sm placeholder-gray-400 dark:placeholder-gray-600 focus:outline-none focus:border-blue-500"
                />
                <p className="text-xs text-gray-600 mt-1">
                  If the receiver has a different CPU architecture, the engine will rebuild the image
                  using the Dockerfile found here. Leave blank to send as-is.
                </p>
              </div>
            )}

            {/* Actions */}
            <div className="flex justify-end gap-3 pt-1">
              <button
                onClick={() => setPushModal({ isOpen: false, image: '' })}
                className="px-4 py-2 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-800 dark:text-white rounded"
              >
                Cancel
              </button>
              <button
                onClick={confirmPush}
                disabled={!selectedPeer || pushing}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed rounded text-white font-medium flex items-center gap-2"
              >
                {pushing && (
                  <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                  </svg>
                )}
                {pushing ? 'Queued…' : 'Confirm Push'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
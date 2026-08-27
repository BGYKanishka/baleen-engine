import { useEffect, useState } from 'react';
import { createDockerDesktopClient } from '@docker/extension-api-client';

interface UpdateResponse {
  update_available: boolean;
  latest_version: string;
  current_version: string;
}

export default function UpdateBanner({ port, token }: { port: number | null, token: string }) {
  const [updateData, setUpdateData] = useState<UpdateResponse | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);
  const ddClient = createDockerDesktopClient();

  const handleUpdate = async () => {
    setIsUpdating(true);
    try {
      if (ddClient.docker?.cli) {
        ddClient.docker.cli.exec('extension', ['update', '-f', 'yehankanishka/baleen-engine:latest']).catch(e => {
          console.log("Expected disconnect during extension update:", e);
        });
      } else {
        throw new Error("Docker CLI is not available");
      }
    } catch (e: any) {
      console.error("Update failed details:", e);
      setIsUpdating(false);
    }
  };

  useEffect(() => {
    if (!port) return;

    // Call the local daemon to check for updates
    fetch(`http://127.0.0.1:${port}/api/update`, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
      .then(res => res.json())
      .then(data => {
        if (data.update_available) {
          setUpdateData(data);
        }
      })
      .catch(err => console.error("Failed to check for updates:", err));
  }, [port, token]);

  if (!updateData || dismissed) return null;

  return (
    <div className="bg-blue-600 text-white px-4 py-3 flex items-center justify-between shadow-md flex-shrink-0">
      <div className="flex items-center gap-3">
        <span className="text-xl"></span>
        <div>
          <span className="font-semibold">Update Available!</span>
          <span className="ml-2 text-sm opacity-90">
            Version {updateData.latest_version} is out (you are on {updateData.current_version}).
          </span>
          <div className="mt-2 flex items-center gap-3">
            <button
              onClick={handleUpdate}
              disabled={isUpdating}
              className="px-3 py-1 bg-white text-blue-600 hover:bg-blue-50 transition-colors rounded text-sm font-medium disabled:opacity-75"
            >
              {isUpdating ? 'Updating...' : 'Update Now'}
            </button>
            <div className="font-mono text-xs bg-blue-700/50 px-2 py-1 rounded inline-block">
              docker extension update yehankanishka/baleen-engine:latest
            </div>
          </div>
        </div>
      </div>
      <button
        onClick={() => setDismissed(true)}
        className="text-white hover:text-blue-200 transition-colors bg-blue-700 hover:bg-blue-800 p-1 rounded-md"
        aria-label="Dismiss"
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 6 6 18" /><path d="m6 6 12 12" /></svg>
      </button>
    </div>
  );
}

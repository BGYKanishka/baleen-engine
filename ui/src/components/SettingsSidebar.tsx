import { useEffect, useRef, useState } from 'react';

interface SettingsSidebarProps {
  isOpen: boolean;
  onClose: () => void;
  nodeName: string;
  customName: string;
  onNameChange: (name: string) => void;
  port: number | null;
  token: string;
}

export default function SettingsSidebar({ isOpen, onClose, nodeName, customName, onNameChange, port, token }: SettingsSidebarProps) {
  const sidebarRef = useRef<HTMLDivElement>(null);

  // Editable node name state
  const [isEditingName, setIsEditingName] = useState(false);
  const [draftName, setDraftName] = useState('');
  const nameInputRef = useRef<HTMLInputElement>(null);

  // Network feature flags — loaded from the backend on open
  const [mdnsDiscovery, setMdnsDiscovery] = useState(true);
  const [broadcastPresence, setBroadcastPresence] = useState(true);
  const [networkLoaded, setNetworkLoaded] = useState(false);

  const displayName = customName || nodeName || '—';

  // Fetch current network settings whenever the sidebar opens (or port becomes available)
  useEffect(() => {
    if (!isOpen || !port) return;
    fetch(`http://127.0.0.1:${port}/api/network/settings`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data) => {
        setMdnsDiscovery(data.mdns_discovery ?? true);
        setBroadcastPresence(data.broadcast_presence ?? true);
        setNetworkLoaded(true);
      })
      .catch(() => setNetworkLoaded(true)); // fail gracefully; keep defaults
  }, [isOpen, port, token]);

  const patchNetworkSetting = async (key: 'mdns_discovery' | 'broadcast_presence', value: boolean) => {
    if (!port) return;
    try {
      await fetch(`http://127.0.0.1:${port}/api/network/settings`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ [key]: value }),
      });
    } catch {
      // API unreachable — toggle UI state was already optimistically updated
    }
  };

  const handleDiscoveryChange = (checked: boolean) => {
    setMdnsDiscovery(checked);
    patchNetworkSetting('mdns_discovery', checked);
  };

  const handleBroadcastChange = (checked: boolean) => {
    setBroadcastPresence(checked);
    patchNetworkSetting('broadcast_presence', checked);
  };

  const startEditing = () => {
    setDraftName(customName || nodeName || '');
    setIsEditingName(true);
    setTimeout(() => nameInputRef.current?.select(), 30);
  };

  const saveName = async () => {
    const trimmed = draftName.trim();
    if (trimmed) {
      // Persist to backend
      if (port) {
        try {
          await fetch(`http://127.0.0.1:${port}/api/node/name`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': `Bearer ${token}`,
            },
            body: JSON.stringify({ name: trimmed }),
          });
        } catch {
          // API unreachable — still update display
        }
      }
      onNameChange(trimmed); // update App state + localStorage
    }
    setIsEditingName(false);
  };

  const cancelEditing = () => {
    setIsEditingName(false);
    setDraftName('');
  };

  // Close on Escape key
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    if (isOpen) document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [isOpen, onClose]);

  // Close on outside click
  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (sidebarRef.current && !sidebarRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    if (isOpen) {
      const t = setTimeout(() => document.addEventListener('mousedown', handleClick), 100);
      return () => {
        clearTimeout(t);
        document.removeEventListener('mousedown', handleClick);
      };
    }
  }, [isOpen, onClose]);

  return (
    <>
      {/* Backdrop */}
      <div
        className={`fixed inset-0 z-40 transition-opacity duration-300 ${isOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
          }`}
        style={{ background: 'rgba(0,0,0,0.35)' }}
        aria-hidden="true"
      />

      {/* Sidebar panel */}
      <div
        ref={sidebarRef}
        className={`fixed top-0 left-0 z-50 h-full flex flex-col transition-all duration-300 ease-in-out
          bg-white dark:bg-[#11151A]
          border-r border-gray-200 dark:border-white/5
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}`}
        style={{
          width: '300px',
          boxShadow: isOpen ? '4px 0 24px rgba(0,0,0,0.15)' : 'none',
          visibility: isOpen ? 'visible' : 'hidden',
        }}
        role="dialog"
        aria-modal="true"
        aria-label="Baleen Settings"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 flex-shrink-0 border-b border-gray-200 dark:border-white/5">
          <div className="flex items-center gap-3">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-gray-500 dark:text-gray-400">
              <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"></path>
              <circle cx="12" cy="12" r="3"></circle>
            </svg>
            <span className="text-gray-900 dark:text-white font-semibold text-base tracking-wide">Baleen Settings</span>
          </div>
          <button
            id="close-settings-btn"
            onClick={onClose}
            aria-label="Close settings"
            className="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 transition-colors duration-150 rounded p-1"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {/* Scrollable body */}
        <div className="flex-1 overflow-y-auto px-5 py-5 space-y-6">

          <Section title="Node Info">
            {/* Editable node name */}
            <div className="flex justify-between items-center px-4 py-3 gap-2">
              <span className="text-sm text-gray-500 dark:text-gray-400 flex-shrink-0">Node Name</span>
              {isEditingName ? (
                <div className="flex items-center gap-1 flex-1 justify-end">
                  <input
                    ref={nameInputRef}
                    id="node-name-input"
                    type="text"
                    value={draftName}
                    onChange={(e) => setDraftName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') saveName();
                      if (e.key === 'Escape') cancelEditing();
                    }}
                    onBlur={saveName}
                    className="text-sm font-medium text-right bg-transparent border-b border-blue-500 outline-none text-gray-900 dark:text-gray-100 w-full max-w-[160px]"
                    autoFocus
                  />
                  {/* Cancel */}
                  <button onClick={cancelEditing} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 flex-shrink-0">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                      <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
                    </svg>
                  </button>
                </div>
              ) : (
                <div className="flex items-center gap-1.5">
                  <span className="text-sm text-gray-900 dark:text-gray-200 font-medium truncate max-w-[140px]">{displayName}</span>
                  <button
                    id="edit-node-name-btn"
                    onClick={startEditing}
                    aria-label="Edit node name"
                    className="text-gray-400 hover:text-blue-500 dark:hover:text-blue-400 transition-colors flex-shrink-0"
                  >
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                    </svg>
                  </button>
                </div>
              )}
            </div>
            <InfoRow label="API Port" value={port ? String(port) : '—'} />
            <InfoRow label="Protocol" value="Baleen P2P v1" />
          </Section>

          <Section title="Network">
            <ToggleRow
              id="setting-discovery"
              label="mDNS Discovery"
              description="Automatically discover peers on local network"
              checked={networkLoaded ? mdnsDiscovery : true}
              onChange={handleDiscoveryChange}
            />
            <ToggleRow
              id="setting-broadcast"
              label="Broadcast Presence"
              description="Let other Baleen nodes find this node"
              checked={networkLoaded ? broadcastPresence : true}
              onChange={handleBroadcastChange}
            />
          </Section>

          <Section title="Transfers">
            <ToggleRow id="setting-auto-approve" label="Auto-approve requests" description="Automatically approve incoming image requests" checked={false} />
            <SliderRow id="setting-bandwidth" label="Max Bandwidth" unit="MB/s" min={1} max={100} defaultValue={50} />
          </Section>

          <Section title="About">
            <InfoRow label="Extension" value="Baleen Engine" />
            <InfoRow label="Transport" value="HTTP/REST" />
            <InfoRow label="Image Scope" value="Local Docker daemon" />
            <InfoRow label="Auth Mode" value="Bearer Token" />
            <InfoRow label="Encryption" value="TLS" />
          </Section>
        </div>

        {/* Footer */}
        <div className="px-5 py-4 flex-shrink-0 border-t border-gray-200 dark:border-white/5">
          <p className="text-xs text-gray-400 dark:text-gray-600 text-center">
            Baleen © {new Date().getFullYear()} · Docker Extension
          </p>
        </div>
      </div>
    </>
  );
}

/* Helper sub-components  */

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-widest mb-3 text-gray-400 dark:text-gray-500">
        {title}
      </p>
      <div className="rounded-xl overflow-hidden divide-y divide-gray-100 dark:divide-white/5 bg-gray-50 dark:bg-white/[0.03] border border-gray-200 dark:border-white/[0.06]">
        {children}
      </div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-center px-4 py-3">
      <span className="text-sm text-gray-500 dark:text-gray-400">{label}</span>
      <span className="text-sm text-gray-900 dark:text-gray-200 font-medium truncate ml-2 max-w-[140px]" title={value}>
        {value}
      </span>
    </div>
  );
}

function ToggleRow({
  id,
  label,
  description,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onChange?: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-3 px-4 py-3">
      <div className="flex-1 min-w-0">
        <p className="text-sm text-gray-800 dark:text-gray-200">{label}</p>
        <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5 leading-snug">{description}</p>
      </div>
      <label className="relative inline-flex items-center cursor-pointer flex-shrink-0 mt-0.5">
        <input
          id={id}
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange?.(e.target.checked)}
          className="sr-only peer"
        />
        <div className="w-9 h-5 rounded-full peer bg-gray-300 dark:bg-gray-600 peer-checked:bg-blue-600 after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full" />
      </label>
    </div>
  );
}

function SliderRow({
  id,
  label,
  unit,
  min,
  max,
  defaultValue,
}: {
  id: string;
  label: string;
  unit: string;
  min: number;
  max: number;
  defaultValue: number;
}) {
  return (
    <div className="px-4 py-3">
      <div className="flex justify-between items-center mb-2">
        <span className="text-sm text-gray-800 dark:text-gray-200">{label}</span>
        <span id={`${id}-value`} className="text-xs font-medium text-blue-500 dark:text-blue-400">
          {defaultValue} {unit}
        </span>
      </div>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        defaultValue={defaultValue}
        onChange={(e) => {
          const el = document.getElementById(`${id}-value`);
          if (el) el.textContent = `${e.target.value} ${unit}`;
        }}
        className="w-full h-1.5 rounded-full appearance-none cursor-pointer bg-gray-200 dark:bg-gray-700"
        style={{ accentColor: '#2563eb' }}
      />
    </div>
  );
}

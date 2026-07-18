import { useEffect, useRef } from 'react';

interface SettingsSidebarProps {
  isOpen: boolean;
  onClose: () => void;
  nodeName: string;
  port: number | null;
}

export default function SettingsSidebar({ isOpen, onClose, nodeName, port }: SettingsSidebarProps) {
  const sidebarRef = useRef<HTMLDivElement>(null);

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
        className={`fixed inset-0 z-40 transition-opacity duration-300 ${
          isOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
        }`}
        style={{ background: 'rgba(0,0,0,0.35)' }}
        aria-hidden="true"
      />

      {/* Sidebar panel */}
      <div
        ref={sidebarRef}
        className={`fixed top-0 left-0 z-50 h-full flex flex-col transition-all duration-300 ease-in-out
          bg-white dark:bg-[#1a1c23]
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
            <svg width="22" height="22" viewBox="0 0 32 32" fill="none" aria-hidden="true">
              <path d="M4 26 C8 18, 16 8, 28 6 C22 12, 18 20, 16 26" stroke="#3b82f6" strokeWidth="2.5" strokeLinecap="round" fill="none" />
              <path d="M16 26 C14 22, 10 20, 4 26" stroke="#3b82f6" strokeWidth="2" strokeLinecap="round" fill="none" />
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
            <InfoRow label="Node Name" value={nodeName || '—'} />
            <InfoRow label="API Port" value={port ? String(port) : '—'} />
            <InfoRow label="Protocol" value="Baleen P2P v1" />
          </Section>

          <Section title="Network">
            <ToggleRow id="setting-discovery" label="mDNS Discovery" description="Automatically discover peers on local network" defaultChecked />
            <ToggleRow id="setting-broadcast" label="Broadcast Presence" description="Let other Baleen nodes find this node" defaultChecked />
          </Section>

          <Section title="Transfers">
            <ToggleRow id="setting-auto-approve" label="Auto-approve requests" description="Automatically approve incoming image requests" defaultChecked={false} />
            <SliderRow id="setting-bandwidth" label="Max Bandwidth" unit="MB/s" min={1} max={100} defaultValue={50} />
          </Section>

          <Section title="Security">
            <InfoRow label="Auth Mode" value="Bearer Token" />
            <ToggleRow id="setting-tls" label="TLS Encryption" description="Encrypt all peer-to-peer communication" defaultChecked />
          </Section>

          <Section title="About">
            <InfoRow label="Extension" value="Baleen Engine" />
            <InfoRow label="Transport" value="HTTP/REST" />
            <InfoRow label="Image Scope" value="Local Docker daemon" />
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
  defaultChecked,
}: {
  id: string;
  label: string;
  description: string;
  defaultChecked: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-3 px-4 py-3">
      <div className="flex-1 min-w-0">
        <p className="text-sm text-gray-800 dark:text-gray-200">{label}</p>
        <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5 leading-snug">{description}</p>
      </div>
      <label className="relative inline-flex items-center cursor-pointer flex-shrink-0 mt-0.5">
        <input id={id} type="checkbox" defaultChecked={defaultChecked} className="sr-only peer" />
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

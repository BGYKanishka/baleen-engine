export interface Peer {
  hostname: string;
  ip: string;
  source: string;
  status: string;
  lastSeen: string;
}

export interface DockerImage {
  name: string;
  tag: string;
  size: string;
}

export interface TransferStream {
  direction: string;
  image: string;
  peer: string;
  progress: number;
  speed: string;
  status: string;
}

export interface LedgerEntry {
  timestamp: string;
  image: string;
  direction: string;
  peer: string;
  status: string;
}

export type AppStatus = 'checking' | 'setup' | 'running' | 'error';
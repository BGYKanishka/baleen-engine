import { useState, useEffect, useCallback } from 'react';
import type { AppStatus } from '../types';

export function useDaemon(ddClient: any) {
  const [status, setStatus] = useState<AppStatus>('checking');
  const [port, setPort] = useState<number | null>(null);
  const [token, setToken] = useState<string>('');
  const [logs, setLogs] = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState<string>('');

  const addLog = useCallback((log: string) => {
    setLogs((prev) => [...prev, log].slice(-500)); 
  }, []);

  const startDaemon = useCallback(async () => {
    const newToken = crypto.randomUUID();
    setToken(newToken);
    setLogs([]); 

    try {
      ddClient.extension.host.cli.exec('baleen', ['daemon', '--token', newToken], {
        stream: {
          onOutput(data: { stdout?: string; stderr?: string }) {
            if (data.stdout) {
              addLog(`[STDOUT] ${data.stdout}`);
              try {
                const parsed = JSON.parse(data.stdout);
                if (parsed.status === 'ready' && parsed.port) {
                  setPort(parsed.port);
                  setStatus('running');
                }
              } catch {
                // Ignore non-JSON stdout
              }
            }
            if (data.stderr) {
              addLog(`[STDERR] ${data.stderr}`);
            }
          },
          onError(error: any) {
            addLog(`[ERROR] Process exited: ${error}`);
            setPort(null);
            setStatus('error');
            setErrorMsg('Daemon crashed or stopped.');
          },
          onClose(exitCode: number) {
            addLog(`[SYSTEM] Daemon closed with exit code ${exitCode}`);
            setPort(null);
            setStatus('error');
          }
        }
      });
    } catch (err: any) {
      setErrorMsg(err.message || 'Failed to execute host CLI');
      setStatus('error');
    }
  }, [ddClient, addLog]);

  useEffect(() => {
    // Docker automatically installs the binary
    startDaemon();
  }, [startDaemon]);

  // Heartbeat to keep daemon alive
  useEffect(() => {
    if (status !== 'running' || !port || !token) return;
    const interval = setInterval(() => {
      fetch(`http://127.0.0.1:${port}/api/health`, {
        headers: { Authorization: `Bearer ${token}` }
      }).catch(() => console.error("Heartbeat failed"));
    }, 5000);
    return () => clearInterval(interval);
  }, [status, port, token]);

  return { status, port, token, logs, errorMsg, startDaemon, setStatus };
}
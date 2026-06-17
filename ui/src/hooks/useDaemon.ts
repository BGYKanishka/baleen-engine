import { useState, useEffect, useCallback, useRef } from 'react';
import type { AppStatus } from '../types';

export function useDaemon(ddClient: any) {
  const [status, setStatus] = useState<AppStatus>('checking');
  const [port, setPort] = useState<number | null>(null);
  const [token, setToken] = useState<string>('');
  const [logs, setLogs] = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState<string>('');
  const isStartingRef = useRef(false);  // prevents double-start
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const addLog = useCallback((log: string) => {
    setLogs((prev) => [...prev, log].slice(-500));
  }, []);

  const startDaemon = useCallback(async () => {
    // Prevent concurrent starts
    if (isStartingRef.current) return;
    isStartingRef.current = true;

    const newToken = crypto.randomUUID();
    setToken(newToken);
    setStatus('checking');
    setPort(null);
    setLogs([]);

    // Timeout: if daemon doesn't become ready in 30s, show error
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    timeoutRef.current = setTimeout(() => {
      setStatus('error');
      setErrorMsg('Daemon did not start within 30 seconds. Check logs tab.');
      isStartingRef.current = false;
    }, 30000);

    try {
      ddClient.extension.host.cli.exec('baleen', ['daemon', '--token', newToken], {
        stream: {
          onOutput(data: { stdout?: string; stderr?: string }) {
            const line = data.stdout || data.stderr || '';
            if (line) addLog(`[${data.stdout ? 'OUT' : 'ERR'}] ${line}`);

            if (data.stdout) {
              // Search for the JSON anywhere in the output line
              const jsonMatch = data.stdout.match(/\{.*"status"\s*:\s*"ready".*\}/);
              if (jsonMatch) {
                try {
                  const parsed = JSON.parse(jsonMatch[0]);
                  if (parsed.status === 'ready' && parsed.port) {
                    if (timeoutRef.current) clearTimeout(timeoutRef.current);
                    setPort(parsed.port);
                    setStatus('running');
                    isStartingRef.current = false;
                  }
                } catch {
                  // Malformed JSON despite match — log it
                  addLog(`[WARN] JSON parse failed on: ${jsonMatch[0]}`);
                }
              }
            }
          },
          onError(error: any) {
            if (timeoutRef.current) clearTimeout(timeoutRef.current);
            addLog(`[ERROR] Process error: ${error}`);
            setPort(null);
            setStatus('error');
            setErrorMsg(`Daemon process error: ${error}`);
            isStartingRef.current = false;
          },
          onClose(exitCode: number) {
            if (timeoutRef.current) clearTimeout(timeoutRef.current);
            addLog(`[SYSTEM] Daemon closed with exit code ${exitCode}`);
            setPort(null);
            isStartingRef.current = false;
            if (exitCode === 0) {
              addLog(`[SYSTEM] Daemon exited cleanly, restarting in 1s...`);
              setTimeout(() => startDaemon(), 1000);
            } else {
              setStatus('error');
              setErrorMsg(`Daemon exited unexpectedly (code ${exitCode}). Check logs.`);
            }
          }
        }
      });
    } catch (err: any) {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      setErrorMsg(err.message || 'Failed to execute host CLI');
      setStatus('error');
      isStartingRef.current = false;
    }
  }, [ddClient, addLog]);

  useEffect(() => {
    startDaemon();
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, []); 

  // Heartbeat to keep daemon alive
  useEffect(() => {
    if (status !== 'running' || !port || !token) return;
    const interval = setInterval(() => {
      fetch(`http://127.0.0.1:${port}/api/health`, {
        headers: { Authorization: `Bearer ${token}` }
      }).catch(() => addLog('[WARN] Heartbeat failed'));
    }, 5000);
    return () => clearInterval(interval);
  }, [status, port, token, addLog]);

  return { status, port, token, logs, errorMsg, startDaemon, setStatus };
}
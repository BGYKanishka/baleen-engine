import { useState, useEffect, useCallback, useRef } from 'react';
import type { AppStatus } from '../types';

export function useDaemon(ddClient: any) {
  const [status, setStatus]     = useState<AppStatus>('checking');
  const [port, setPort]         = useState<number | null>(null);
  const [token, setToken]       = useState<string>('');
  const [nodeName, setNodeName] = useState<string>('');
  const [logs, setLogs]         = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState<string>('');

  const isStartingRef          = useRef(false);
  const isIntentionalStopRef   = useRef(false);
  const timeoutRef             = useRef<ReturnType<typeof setTimeout> | null>(null);
  const heartbeatIntervalRef   = useRef<ReturnType<typeof setInterval> | null>(null);
  const logIntervalRef         = useRef<ReturnType<typeof setInterval> | null>(null);
  const tokenRef               = useRef<string>('');
  const portRef                = useRef<number | null>(null);

  const addLog = useCallback((log: string) => {
    setLogs((prev) => [...prev, log].slice(-500));
  }, []);

  // Called once we have a confirmed port and token.
  
  const connectToService = useCallback((p: number, t: string, nName: string) => {
    portRef.current  = p;
    tokenRef.current = t;
    setPort(p);
    setToken(t);
    setNodeName(nName || '');
    setStatus('running');
    isStartingRef.current = false;
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    addLog(`[SYSTEM] Connected to Baleen service on port ${p}`);
  }, [addLog]);

  // Check the status of the daemon.
  const checkDaemonStatus = useCallback(() => {
    setStatus('checking');

    ddClient.extension.host.cli.exec('baleen', ['status'], {
      stream: {
        onOutput(data: { stdout?: string; stderr?: string }) {
          const line = (data.stdout || '').trim();
          if (!line) return;
          try {
            const jsonMatch = line.match(/\{[^}]*\}/);
            if (jsonMatch) {
              const parsed = JSON.parse(jsonMatch[0]);
              if (parsed.status === 'running' && parsed.port) {
                addLog(`[SYSTEM] Found running service on port ${parsed.port}`);
                connectToService(Number(parsed.port), parsed.token || '', parsed.node_name || '');
              }
              // status === 'stopped' is handled in onClose
            }
          } catch {
            // non-JSON line — ignore
          }
        },
        onError(_err: any) {
          // `baleen status` errored (binary not found etc.)
          setStatus('stopped');
        },
        onClose(_exitCode: number) {
          // If we didn't connect above, the daemon is stopped.
          if (portRef.current === null) {
            setStatus('stopped');
          }
        },
      },
    });
  }, [ddClient, addLog, connectToService]);

  // Start the daemon process via `baleen daemon --token <token>`.
  const startDaemon = useCallback(async () => {
    if (isStartingRef.current) return;
    isStartingRef.current = true;
    isIntentionalStopRef.current = false;

    const newToken = crypto.randomUUID();
    setStatus('checking');
    setPort(null);
    portRef.current  = null;
    tokenRef.current = '';
    setLogs([]);
    setErrorMsg('');

    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    timeoutRef.current = setTimeout(() => {
      if (isStartingRef.current) {
        setStatus('error');
        setErrorMsg('Service did not respond within 30 seconds. Check logs tab.');
        isStartingRef.current = false;
      }
    }, 30_000);

    try {
      ddClient.extension.host.cli.exec(
        'baleen',
        ['daemon', '--token', newToken],
        {
          stream: {
            onOutput(data: { stdout?: string; stderr?: string }) {
              const line = data.stdout || data.stderr || '';
              if (line) addLog(`[${data.stdout ? 'OUT' : 'ERR'}] ${line}`);

              if (data.stdout) {
                const jsonMatch = data.stdout.match(/\{[^}]*"(?:status|port)"[^}]*\}/);
                if (jsonMatch) {
                  try {
                    const parsed = JSON.parse(jsonMatch[0]);
                    if (parsed.port) {
                      // Use the token from service.json when reconnecting,
                      // or our newly generated token for a fresh start.
                      const resolvedToken = parsed.token || newToken;
                      connectToService(Number(parsed.port), resolvedToken, parsed.node_name || '');
                    }
                  } catch {
                    addLog(`[WARN] JSON parse failed: ${jsonMatch[0]}`);
                  }
                }
              }
            },
            onError(error: any) {
              if (timeoutRef.current) clearTimeout(timeoutRef.current);
              addLog(`[ERROR] ${error}`);
              setStatus('error');
              setErrorMsg(`Service process error: ${error}`);
              isStartingRef.current = false;
            },
            onClose(exitCode: number) {
              if (timeoutRef.current) clearTimeout(timeoutRef.current);
              isStartingRef.current = false;
              // If the service exited with code 0, it means it stopped intentionally.
              // If already connected, don't need to change the status.
              if (exitCode === 0) {
                if (portRef.current !== null) return; // already connected — all good

                if (isIntentionalStopRef.current) {
                  isIntentionalStopRef.current = false;
                  setStatus('stopped');
                  setPort(null);
                  portRef.current = null;
                }
                return;
              }

              // Non-zero exit: unexpected failure.
              setPort(null);
              portRef.current = null;
              setStatus('error');
              setErrorMsg(`Service exited unexpectedly (code ${exitCode}). Check logs.`);
            },
          },
        }
      );
    } catch (err: any) {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      setErrorMsg(err.message || 'Failed to execute host CLI');
      setStatus('error');
      isStartingRef.current = false;
    }
  }, [ddClient, addLog, connectToService]);


  // Stop the daemon process via `baleen stop`.
  
  const stopDaemon = useCallback(async () => {
    isIntentionalStopRef.current = true;
    const p = portRef.current;
    const t = tokenRef.current;

    if (!p) {
      setStatus('stopped');
      return;
    }

    addLog('[SYSTEM] Sending stop request to service...');
    try {
      await fetch(`http://127.0.0.1:${p}/api/stop`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${t}` },
      });
    } catch {
      // Service may have already exited — that's fine.
    }

    setPort(null);
    portRef.current  = null;
    tokenRef.current = '';
    setStatus('stopped');
    addLog('[SYSTEM] Service stopped.');
  }, [addLog]);

  
  // Cleanup on unmount: clear any pending timeouts or intervals.
  useEffect(() => {
    checkDaemonStatus();
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
      if (heartbeatIntervalRef.current) clearInterval(heartbeatIntervalRef.current);
      if (logIntervalRef.current) clearInterval(logIntervalRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const heartbeatFailsRef = useRef(0);
  useEffect(() => {
    if (heartbeatIntervalRef.current) clearInterval(heartbeatIntervalRef.current);
    if (status !== 'running' || !port || !token) return;

    heartbeatFailsRef.current = 0;
    heartbeatIntervalRef.current = setInterval(async () => {
      try {
        const res = await fetch(`http://127.0.0.1:${port}/api/health`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          heartbeatFailsRef.current = 0;
        } else {
          throw new Error(`HTTP ${res.status}`);
        }
      } catch {
        heartbeatFailsRef.current++;
        addLog(`[WARN] Heartbeat failed (${heartbeatFailsRef.current}/3)`);
        if (heartbeatFailsRef.current >= 3) {
          clearInterval(heartbeatIntervalRef.current!);
          setPort(null);
          portRef.current  = null;
          tokenRef.current = '';

          // Ask the binary whether the daemon is truly gone or just crashed.
          ddClient.extension.host.cli.exec('baleen', ['status'], {
            stream: {
              onOutput(data: { stdout?: string }) {
                const line = (data.stdout || '').trim();
                try {
                  const jsonMatch = line.match(/\{[^}]*\}/);
                  if (jsonMatch) {
                    const parsed = JSON.parse(jsonMatch[0]);
                    if (parsed.status === 'stopped') {
                      addLog('[SYSTEM] Service was stopped externally.');
                      setStatus('stopped');
                      return;
                    }
                  }
                } catch { /* ignore */ }
              },
              onError() {
                setStatus('error');
                setErrorMsg('Lost connection to the background service. Click Restart to reconnect.');
              },
              onClose() {
                setStatus((current) => {
                  if (current === 'running') {
                    setErrorMsg('Lost connection to the background service. Click Restart to reconnect.');
                    return 'error';
                  }
                  return current;
                });
              },
            },
          });
        }
      }
    }, 2000);

    return () => {
      if (heartbeatIntervalRef.current) clearInterval(heartbeatIntervalRef.current);
    };
  }, [status, port, token, addLog, ddClient]);


  // Log Polling: fetch /api/logs every 2s while running.
  useEffect(() => {
    if (logIntervalRef.current) clearInterval(logIntervalRef.current);
    if (status !== 'running' || !port || !token) return;

    // Fetch immediately, then every 2s
    const fetchLogs = async () => {
      try {
        const res = await fetch(`http://127.0.0.1:${port}/api/logs`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          const fetchedLogs = await res.json();
          setLogs(fetchedLogs);
        }
      } catch {
        // ignore fetch errors, heartbeat handles disconnects
      }
    };
    
    fetchLogs();
    logIntervalRef.current = setInterval(fetchLogs, 2000);

    return () => {
      if (logIntervalRef.current) clearInterval(logIntervalRef.current);
    };
  }, [status, port, token]);

  return { status, port, token, nodeName, logs, errorMsg, startDaemon, stopDaemon, checkDaemonStatus, setStatus };
}
import { useState, useEffect, useCallback, useRef } from "react";

interface UseSSEOptions {
  url: string;
  initialState?: any;
}

export function useSSE<T>({ url, initialState = null }: UseSSEOptions) {
  const [data, setData] = useState<T | null>(initialState);
  const [error, setError] = useState<Error | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  const closeEventSource = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  const setupSSE = useCallback(() => {
    closeEventSource(); // Close any existing connection
    setError(null); // Reset any previous errors

    const newEventSource = new EventSource(url);
    eventSourceRef.current = newEventSource;

    newEventSource.onmessage = (event) => {
      try {
        const parsedData = JSON.parse(event.data);
        setData(parsedData);
      } catch (err) {
        setError(new Error("Failed to parse SSE data"));
      }
    };

    newEventSource.onerror = (err) => {
      setError(err instanceof Error ? err : new Error("SSE error"));
      closeEventSource();
    };
  }, [url, closeEventSource]);

  useEffect(() => {
    setupSSE();
    return closeEventSource;
  }, [setupSSE, closeEventSource]);

  const refresh = useCallback(() => {
    setupSSE();
  }, [setupSSE]);

  return { data, error, refresh };
}

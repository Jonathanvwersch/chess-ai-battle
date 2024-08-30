import { useState, useEffect, useCallback } from "react";

interface QueryIntervalOptions<T> {
  queryFn: () => Promise<T>;
  interval: number;
  initialFetch?: boolean;
}

export function useQueryInterval<T>({
  queryFn,
  interval,
  initialFetch = true,
}: QueryIntervalOptions<T>) {
  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async () => {
    setIsLoading(true);
    try {
      const result = await queryFn();
      setData(result);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e : new Error("An error occurred"));
    } finally {
      setIsLoading(false);
    }
  }, [queryFn]);

  useEffect(() => {
    if (initialFetch) {
      fetchData();
    }

    const intervalId = setInterval(fetchData, interval);

    return () => clearInterval(intervalId);
  }, [fetchData, interval, initialFetch]);

  const refetch = useCallback(() => {
    fetchData();
  }, [fetchData]);

  return { data, isLoading, error, refetch };
}

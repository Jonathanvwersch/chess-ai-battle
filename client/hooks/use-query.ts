import { useCallback, useEffect, useRef, useState } from "react";

type Props = Readonly<{ callback: () => void; interval: number | null }>;

export function useQuery<T>(queryFn?: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const handleQuery = useCallback(async (queryFn: () => Promise<T>) => {
    try {
      setIsLoading(true);
      const res = await queryFn();
      setData(res);
    } catch (e) {
      setError(e instanceof Error ? e : new Error("Something went wrong"));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (queryFn) {
      void handleQuery(queryFn);
    }
  }, [queryFn]);

  return {
    data,
    error,
    isLoading,
    handleQuery,
  };
}

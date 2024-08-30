import { useEffect, useRef } from "react";

type Props = Readonly<{ callback: () => void; interval: number | null }>;

export function useInterval({ callback, interval }: Props) {
  const callbackRef = useRef(callback);

  useEffect(() => {
    callbackRef.current = callback;
  }, []);

  useEffect(() => {
    function tick() {
      callbackRef.current();
    }

    if (interval !== null) {
      const id = setInterval(tick, interval);
      return () => clearInterval(id);
    }
  }, []);
}

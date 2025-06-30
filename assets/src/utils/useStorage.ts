
import { useEffect, useState } from 'react';
import { isSSR } from '@utils/is';

const getDefaultStorage = (key: string) => {
  if (!isSSR) {
    return localStorage.getItem(key);
  } else {
    return undefined;
  }
};

function useStorage(
  key: string,
  defaultValue?: string
): [string, (string: string) => void, () => void,(string: string) => void, ] {
  const [storedValue, setStoredValue] = useState<string>(
    getDefaultStorage(key) ?? defaultValue ?? ''
  );

  const setJsonValue = (obj: unknown) => {
    setStorageValue(JSON.stringify(obj));
  };

  const setStorageValue = (value: string) => {
    if (!isSSR) {
      localStorage.setItem(key, value);
      if (value !== storedValue) {
        setStoredValue(value);
      }
    }
  };

  const removeStorage = () => {
    if (!isSSR) {
      localStorage.removeItem(key);
    }
  };

  useEffect(() => {
    const storageValue = localStorage.getItem(key);
    setStoredValue(storageValue ?? defaultValue ?? '');
  }, []);

  return [storedValue, setStorageValue, removeStorage, setJsonValue];
}

export default useStorage;

import { useCallback, useEffect, useRef } from 'react';

interface VSCodeApi {
    postMessage: (message: unknown) => void;
    getState: () => unknown;
    setState: (state: unknown) => void;
}

declare global {
    interface Window {
        acquireVsCodeApi?: () => VSCodeApi;
    }
}

// Singleton to ensure we only acquire the API once
let vscodeApi: VSCodeApi | null = null;

function getVSCodeApi(): VSCodeApi | null {
    if (vscodeApi) return vscodeApi;
    if (typeof window.acquireVsCodeApi === 'function') {
        vscodeApi = window.acquireVsCodeApi();
        return vscodeApi;
    }
    return null;
}

type MessageHandler = (message: { type: string; payload?: unknown }) => void;

/**
 * Hook for communicating with VSCode extension host.
 * Returns postMessage function and onMessage subscriber.
 */
export function useVSCodeApi() {
    const handlersRef = useRef<Set<MessageHandler>>(new Set());
    const api = getVSCodeApi();

    useEffect(() => {
        const handleMessage = (event: MessageEvent) => {
            const message = event.data;
            handlersRef.current.forEach(handler => handler(message));
        };

        window.addEventListener('message', handleMessage);
        return () => window.removeEventListener('message', handleMessage);
    }, []);

    const postMessage = useCallback((message: unknown) => {
        if (api) {
            api.postMessage(message);
        } else {
            // Development mode - log to console
            console.log('[DEV] postMessage:', message);
        }
    }, [api]);

    const onMessage = useCallback((handler: MessageHandler) => {
        handlersRef.current.add(handler);
        return () => handlersRef.current.delete(handler);
    }, []);

    const getState = useCallback(() => api?.getState(), [api]);
    const setState = useCallback((state: unknown) => api?.setState(state), [api]);

    return { postMessage, onMessage, getState, setState };
}

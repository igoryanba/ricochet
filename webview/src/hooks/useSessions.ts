import { useState, useCallback, useEffect } from 'react';
import { useVSCodeApi } from './useVSCodeApi';
import { SessionMetadata } from '../types/session';

export function useSessions() {
    const { postMessage, onMessage } = useVSCodeApi();
    const [sessions, setSessions] = useState<SessionMetadata[]>([]);
    const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);

    useEffect(() => {
        const unsubscribe = onMessage((message) => {
            switch (message.type) {
                case 'session_list':
                    setSessions((message.payload as { sessions: SessionMetadata[] }).sessions || []);
                    break;
                case 'session_created':
                    setCurrentSessionId((message.payload as { id: string }).id);
                    // Refresh list
                    postMessage({ type: 'list_sessions' });
                    break;
                case 'session_loaded':
                    // Handled by useChat mostly, but we update current ID here
                    setCurrentSessionId((message.payload as { id: string }).id);
                    break;
            }
        });

        // Initial fetch
        postMessage({ type: 'list_sessions' });

        return () => { unsubscribe(); };
    }, [postMessage, onMessage]);

    const createSession = useCallback(() => {
        postMessage({ type: 'create_session' });
    }, [postMessage]);

    const loadSession = useCallback((id: string) => {
        setCurrentSessionId(id);
        postMessage({ type: 'load_session', payload: { id } });
    }, [postMessage]);

    const deleteSession = useCallback((id: string) => {
        postMessage({ type: 'delete_session', payload: { id } });
    }, [postMessage]);

    return {
        sessions,
        currentSessionId,
        createSession,
        loadSession,
        deleteSession
    };
}

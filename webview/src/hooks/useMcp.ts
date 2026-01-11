import { useState, useEffect, useCallback } from 'react';
import { useVSCodeApi } from './useVSCodeApi';
import { McpServer } from '../types/mcp';

export type McpServerState = McpServer;

export function useMcp() {
    const { postMessage, onMessage } = useVSCodeApi();
    const [servers, setServers] = useState<McpServerState[]>([]);
    const [isLoading, setIsLoading] = useState(false);

    useEffect(() => {
        setIsLoading(true);
        postMessage({ type: 'get_mcp_servers' });

        const removeListener = onMessage((message) => {
            if (message.type === 'mcp_servers') {
                const payload = message.payload as { servers: McpServerState[] };
                setServers(payload.servers);
                setIsLoading(false);
            }
        });

        return () => { removeListener(); };
    }, [postMessage, onMessage]);

    const connectServer = useCallback((name: string, config: any) => {
        postMessage({
            type: 'connect_mcp_server',
            payload: { name, config }
        });
    }, [postMessage]);

    const refreshServers = useCallback(() => {
        setIsLoading(true);
        postMessage({ type: 'get_mcp_servers' });
    }, [postMessage]);

    return {
        servers,
        isLoading,
        connectServer,
        refreshServers
    };
}

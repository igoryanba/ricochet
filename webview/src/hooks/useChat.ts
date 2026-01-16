import { useState, useCallback, useEffect, useRef } from 'react';
import { useVSCodeApi } from './useVSCodeApi';

export interface ChatMessage {
    id: string;
    role: 'user' | 'assistant' | 'system';
    content: string;
    timestamp: number;
    isStreaming?: boolean;
    toolCalls?: ToolCall[];
    activities?: ActivityItem[]; // Files analyzed, edited, searched
    steps?: ProgressStep[]; // Granular agent activity
    metadata?: TaskMetadata; // Usage stats (tokens, cost)
    via?: 'telegram' | 'discord' | 'ide';  // Ether: message source
    remoteUsername?: string;  // Ether: remote user name
    checkpointHash?: string;  // Workspace checkpoint for restore
}

export interface TaskMetadata {
    tokensIn: number;
    tokensOut: number;
    totalCost: number;
    contextLimit: number;
}

export interface ProgressStep {
    id: string;
    label: string;
    status: 'pending' | 'running' | 'completed' | 'error';
    details?: string[]; // Sub-items like "Analyzed file.ts", "Edited main.go"
}

export interface ToolCall {
    id: string;
    name: string;
    arguments: Record<string, unknown>;
    result?: string;
    status: 'pending' | 'running' | 'completed' | 'error';
}

export interface ActivityItem {
    type: 'search' | 'analyze' | 'edit' | 'command';
    file?: string;
    lineRange?: string;    // "L16-815"
    results?: number;      // for search
    additions?: number;    // for edit
    deletions?: number;    // for edit
    query?: string;        // for search
}

export interface ContextStatus {
    tokens_used: number;
    tokens_max: number;
    percentage: number;
    was_condensed?: boolean;
    was_truncated?: boolean;
    cumulative_cost?: number;
}

export interface TaskProgress {
    task_name: string;
    status: string;
    summary?: string;
    mode?: 'planning' | 'execution' | 'verification';
    steps: string[];
    files: string[];
    is_active: boolean;
}

export interface Todo {
    text: string;
    status: 'pending' | 'current' | 'completed';
}

/**
 * Hook for managing chat state.
 * Handles message sending, receiving, and history.
 */
export function useChat(sessionId: string = 'default') {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [todos, setTodos] = useState<Todo[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [inputValue, setInputValue] = useState('');
    const [currentMode, setCurrentMode] = useState<string>('code');
    const [contextStatus, setContextStatus] = useState<ContextStatus | null>(null);
    const [taskProgress, setTaskProgress] = useState<TaskProgress | null>(null);
    const { postMessage, onMessage } = useVSCodeApi();

    const [fileResults, setFileResults] = useState<FileSearchResult[]>([]);

    // Debounce for streaming updates - max 5 updates per second
    const pendingUpdateRef = useRef<ChatMessage | null>(null);
    const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const DEBOUNCE_MS = 400; // Aggressive debounce to prevent crash during heavy streaming

    const flushPendingUpdate = useCallback(() => {
        debounceTimerRef.current = null;
        if (pendingUpdateRef.current) {
            const update = pendingUpdateRef.current;
            pendingUpdateRef.current = null;
            setMessages(prev => {
                const existing = prev.find(m => m.id === update.id);
                if (existing) {
                    return prev.map(m => m.id === update.id ? update : m);
                }
                return [...prev, update];
            });
        }
    }, []);

    // Listen for chat updates from extension
    useEffect(() => {
        const unsubscribe = onMessage((message) => {
            switch (message.type) {
                case 'chat_update':
                    const update = message.payload as { message: ChatMessage; session_id?: string };
                    if (update.session_id && update.session_id !== sessionId) return;

                    // Final messages apply immediately
                    if (!update.message.isStreaming) {
                        // Clear any pending debounce
                        if (debounceTimerRef.current) {
                            clearTimeout(debounceTimerRef.current);
                            debounceTimerRef.current = null;
                        }
                        pendingUpdateRef.current = null;
                        setMessages(prev => {
                            const existing = prev.find(m => m.id === update.message.id);
                            if (existing) {
                                return prev.map(m => m.id === update.message.id ? update.message : m);
                            }
                            return [...prev, update.message];
                        });
                        setIsLoading(false);
                    } else {
                        // Streaming: debounce updates (only update every 200ms)
                        pendingUpdateRef.current = update.message;
                        if (!debounceTimerRef.current) {
                            debounceTimerRef.current = setTimeout(flushPendingUpdate, DEBOUNCE_MS);
                        }
                    }
                    break;
                case 'chat_cleared':
                    setMessages([]);
                    break;
                case 'state':
                    // ... existing
                    const state = message.payload as { messages?: ChatMessage[]; mode?: string; todos?: Todo[]; session_id?: string };
                    if (state.messages) setMessages(state.messages);
                    if (state.mode) setCurrentMode(state.mode);
                    if (state.todos) setTodos(state.todos);
                    break;
                case 'mode_changed':
                    // ... existing
                    const { mode } = message.payload as { mode: string };
                    setCurrentMode(mode);
                    break;
                case 'task_state_updated':
                    // ... existing
                    const taskState = message.payload as { todos: Todo[]; session_id?: string };
                    if (taskState.session_id && taskState.session_id !== sessionId) return;
                    setTodos(taskState.todos);
                    break;
                case 'file_search_results':
                    setFileResults(message.payload as FileSearchResult[]);
                    break;
                case 'context_status':
                    setContextStatus(message.payload as ContextStatus);
                    break;
                case 'task_progress':
                    const progress = message.payload as TaskProgress;
                    setTaskProgress(prev => {
                        // If same task, accumulate steps; otherwise replace
                        if (prev && prev.task_name === progress.task_name) {
                            return {
                                ...progress,
                                steps: [...prev.steps, progress.status],
                                files: [...new Set([...prev.files, ...progress.files])]
                            };
                        }
                        return {
                            ...progress,
                            steps: progress.status ? [progress.status] : [],
                        };
                    });
                    break;
                case 'error':
                    const errMsg = (message.payload as { message: string }).message;
                    setMessages(prev => [...prev, {
                        id: `err-${Date.now()}`,
                        role: 'assistant',
                        content: `⚠️ **Error**: ${errMsg}`,
                        timestamp: Date.now()
                    }]);
                    setIsLoading(false);
                    break;
            }
        });
        return () => { unsubscribe(); };
    }, [onMessage, sessionId]);

    // Request initial state on mount (restores history when switching views)
    useEffect(() => {
        postMessage({ type: 'get_state' });
    }, [postMessage]);

    // ... existing initialization

    const sendMessage = useCallback((content: string) => {
        // ... existing
        if (!content.trim()) return;

        // Slash Command Interception
        if (content.trim().startsWith('/')) {
            const [cmd] = content.trim().split(' ');
            if (cmd === '/clear' || cmd === '/reset') {
                postMessage({ type: 'clear_chat' });
                return;
            }
            // /mode is handled by backend text processing usually, or we can handle it here if we want explicit event.
            // For now, let other commands pass through to backend (e.g. /mode)
        }

        const userMessage: ChatMessage = {
            id: `msg-${Date.now()}`,
            role: 'user',
            content: content.trim(),
            timestamp: Date.now()
        };

        setMessages(prev => [...prev, userMessage]);
        setIsLoading(true);
        setInputValue('');

        postMessage({
            type: 'send_message',
            payload: { content: content.trim(), session_id: sessionId }
        });
    }, [postMessage, sessionId]);

    const switchMode = useCallback((mode: string) => {
        postMessage({
            type: 'send_message',
            payload: { content: `/mode ${mode}` }
        });
    }, [postMessage]);

    const searchFiles = useCallback((query: string) => {
        postMessage({
            type: 'search_files',
            payload: { query }
        });
    }, [postMessage]);

    const executeCommand = useCallback((command: string) => {
        postMessage({
            type: 'execute_command',
            payload: { command }
        });
    }, [postMessage]);

    const saveCheckpoint = useCallback((message?: string) => {
        postMessage({
            type: 'save_checkpoint',
            payload: { message }
        });
    }, [postMessage]);

    const restoreCheckpoint = useCallback((hash: string) => {
        postMessage({
            type: 'checkpoint_restore',
            payload: { hash }
        });
    }, [postMessage]);

    const cancelGeneration = useCallback(() => {
        postMessage({ type: 'cancel_generation' });
        setIsLoading(false);
    }, [postMessage]);

    return {
        messages,
        todos,
        isLoading,
        inputValue,
        currentMode,
        contextStatus,
        taskProgress,
        fileResults,
        setInputValue,
        sendMessage,
        switchMode,
        searchFiles,
        executeCommand,
        saveCheckpoint,
        restoreCheckpoint,
        cancelGeneration
    };
}

export interface FileSearchResult {
    path: string;
    name: string;
}

import { useRef, useState, useMemo, useEffect } from 'react';
import { useChat } from '@hooks/useChat';
import { ChatMessage } from './ChatMessage';
import { ChatInput } from './ChatInput';
import { AutoApprovePanel } from './AutoApprovePanel';
import { EtherPanel } from './EtherPanel';
import { TodoTracker } from './TodoTracker';
// import { PermissionRequestPanel } from './PermissionRequestPanel'';
// import { Virtuoso, VirtuosoHandle } from 'react-virtuoso'; // TEMPORARILY DISABLED
import { useLiveMode } from '@hooks/useLiveMode';
import { useVSCodeApi } from '@hooks/useVSCodeApi';
import { useSessions } from '@hooks/useSessions';
import { ChevronDown, ChevronRight } from 'lucide-react';

interface ChatViewProps {
    onOpenSettings: () => void;
    onOpenHistory: () => void;
    onOpenAgent: () => void;
}

// interface PermissionRequest {
//     id: string;
//     question: string;
// }

/**
 * Main Chat View for Ricochet.
 * Single panel, settings in bottom toolbar, minimal top bar.
 */
export function ChatView({ onOpenSettings }: ChatViewProps) {
    const { currentSessionId } = useSessions();
    const { messages, todos, isLoading, inputValue, setInputValue, sendMessage, cancelGeneration, executeCommand, restoreCheckpoint } = useChat(currentSessionId || 'default');
    const { status: liveStatus, toggleLiveMode } = useLiveMode();
    const scrollRef = useRef<HTMLDivElement>(null);
    // Auto-respond to permission requests to prevent Promise deadlock
    // (Actual approval comes from Telegram in Live Mode)
    const { onMessage } = useVSCodeApi();
    useEffect(() => {
        const unsubscribe = onMessage((message: any) => {
            if (message.type === 'request_permission') {
                // Just log it - don't block, let Telegram handle actual approval
                console.log('[ChatView] Permission request received (handled by Telegram):', message.payload?.question?.slice(0, 50));
                // Note: We don't auto-respond here because Telegram will respond via core
            }
        });
        return () => { unsubscribe(); };
    }, [onMessage]);

    const handleSend = (text?: string) => {
        const msg = typeof text === 'string' ? text : inputValue;
        if (!msg.trim()) return;
        sendMessage(msg);
    };

    // Extract task topic from first user message (Kilo Code style)
    const fullTaskTopic = useMemo(() => {
        const firstUserMsg = messages.find(m => m.role === 'user');
        return firstUserMsg?.content || null;
    }, [messages]);

    const [taskExpanded, setTaskExpanded] = useState(false);

    // Show truncated in header, full when expanded
    const displayTopic = fullTaskTopic
        ? (taskExpanded ? fullTaskTopic : (fullTaskTopic.length > 80 ? fullTaskTopic.slice(0, 80) + '...' : fullTaskTopic))
        : null;

    return (
        <div className="flex flex-col h-full bg-vscode-sideBar-background text-vscode-fg">
            <style>{`
                @keyframes fadeIn {
                    from { opacity: 0; transform: translateY(4px); }
                    to { opacity: 1; transform: translateY(0); }
                }
                .animate-fade-in {
                    animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1) forwards;
                }
            `}</style>

            {/* Task Header - Kilo Code style sticky top */}
            {fullTaskTopic && (
                <div className="sticky top-0 z-10 bg-[#1e1e1e] border-b border-[#333] shadow-md">
                    <button
                        onClick={() => setTaskExpanded(!taskExpanded)}
                        className="w-full flex items-start gap-2 px-3 py-2 hover:bg-[#252526] transition-colors text-left"
                    >
                        {taskExpanded ? (
                            <ChevronDown className="w-4 h-4 text-vscode-fg/50 flex-shrink-0 mt-0.5" />
                        ) : (
                            <ChevronRight className="w-4 h-4 text-vscode-fg/50 flex-shrink-0 mt-0.5" />
                        )}
                        <span className="text-xs text-blue-400 font-medium flex-shrink-0 mt-0.5">Task:</span>
                        <span className={`text-xs text-vscode-fg/80 flex-1 ${taskExpanded ? 'whitespace-pre-wrap' : 'truncate'}`}>
                            {displayTopic}
                        </span>
                    </button>
                </div>
            )}

            {/* Content area */}
            <div className="flex-1 min-h-0 flex flex-col pt-0.5">
                <TodoTracker todos={todos} />

                <div className="flex-1 min-h-0">
                    {messages.length === 0 ? (
                        <WelcomeMessage />
                    ) : (
                        <div
                            ref={scrollRef}
                            className="h-full overflow-y-auto"
                        >
                            {messages.map((message) => (
                                <ChatMessage
                                    key={message.id}
                                    message={message}
                                    onExecuteCommand={executeCommand}
                                    onRestore={restoreCheckpoint}
                                />
                            ))}
                        </div>
                    )}
                </div>
            </div>

            {/* Bottom Input Area */}
            <div className="bg-vscode-editor-background">
                <AutoApprovePanel />
                {liveStatus.enabled && <EtherPanel status={liveStatus} onToggleLiveMode={() => toggleLiveMode()} />}

                {/* Permission Request Panel - TEMPORARILY DISABLED FOR DEBUGGING */}
                {/* {permissionRequest && (
                    <PermissionRequestPanel
                        request={permissionRequest}
                        onResponse={handlePermissionResponse}
                    />
                )} */}

                <ChatInput
                    value={inputValue}
                    onChange={setInputValue}
                    onSend={handleSend}
                    isLoading={isLoading}
                    onCancel={cancelGeneration}
                    onOpenSettings={onOpenSettings}
                    liveStatus={liveStatus}
                    onToggleLiveMode={toggleLiveMode}
                />
            </div>
        </div>
    );
}

function WelcomeMessage() {
    return (
        <div className="flex flex-col items-center justify-center h-full text-center px-6 py-12">
            {/* Logo */}
            <div className="mb-6">
                <svg width="64" height="64" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M9.75 7H9.25C9.11193 7 9 7.44772 9 8V23C9 23.5523 9.11193 24 9.25 24H9.75C9.88807 24 10 23.5523 10 23V8C10 7.44772 9.88807 7 9.75 7Z" fill="currentColor" fillOpacity="0.8" />
                    <path d="M23 11.5C23 9.567 21.433 8 19.5 8C17.567 8 16 9.567 16 11.5C16 13.433 17.567 15 19.5 15C21.433 15 23 13.433 23 11.5Z" fill="currentColor" fillOpacity="0.8" />
                    <path d="M19 14C19 12.3431 17.6569 11 16 11C14.3431 11 13 12.3431 13 14C13 15.6569 14.3431 17 16 17C17.6569 17 19 15.6569 19 14Z" fill="currentColor" fillOpacity="0.5" />
                </svg>
            </div>

            {/* Title */}
            <h2 className="text-lg font-medium text-vscode-fg mb-2">
                What can I help you with?
            </h2>
            <p className="text-sm text-vscode-fg/50 max-w-[280px]">
                Generate, refactor, and debug code with AI.
            </p>
        </div>
    );
}

import { useRef } from 'react';
import { useChat } from '@hooks/useChat';
import { ChatMessage } from './ChatMessage';
import { ChatInput } from './ChatInput';
import { AutoApprovePanel } from './AutoApprovePanel';
import { EtherPanel } from './EtherPanel';
import { TodoTracker } from './TodoTracker';
import { Virtuoso, VirtuosoHandle } from 'react-virtuoso';
import { useLiveMode } from '@hooks/useLiveMode';

interface ChatViewProps {
    onOpenSettings: () => void;
    onOpenHistory: () => void;
    onOpenAgent: () => void;
}

/**
 * Main Chat View for Ricochet.
 * Single panel, settings in bottom toolbar, minimal top bar.
 */
export function ChatView({ onOpenSettings }: ChatViewProps) {
    const { messages, todos, isLoading, inputValue, setInputValue, sendMessage, cancelGeneration, executeCommand, restoreCheckpoint } = useChat('default');
    const { status: liveStatus } = useLiveMode();
    const virtuosoRef = useRef<VirtuosoHandle>(null);

    const handleSend = (text?: string) => {
        const msg = typeof text === 'string' ? text : inputValue;
        if (!msg.trim()) return;
        sendMessage(msg);
    };

    return (
        <div className="flex flex-col h-full bg-vscode-sideBar-background text-vscode-fg">
            {/* Content area */}
            <div className="flex-1 min-h-0 flex flex-col pt-0.5">
                <TodoTracker todos={todos} />

                <div className="flex-1 min-h-0">
                    {messages.length === 0 ? (
                        <WelcomeMessage />
                    ) : (
                        <Virtuoso
                            ref={virtuosoRef}
                            className="h-full"
                            data={messages}
                            initialTopMostItemIndex={messages.length - 1}
                            followOutput="smooth"
                            itemContent={(_index, message) => (
                                <ChatMessage
                                    key={message.id}
                                    message={message}
                                    onExecuteCommand={executeCommand}
                                    onRestore={restoreCheckpoint}
                                />
                            )}
                        />
                    )}
                </div>
            </div>

            {/* Bottom Input Area */}
            <div className="bg-vscode-editor-background">
                <AutoApprovePanel />
                {liveStatus.enabled && <EtherPanel status={liveStatus} />}
                <ChatInput
                    value={inputValue}
                    onChange={setInputValue}
                    onSend={handleSend}
                    isLoading={isLoading}
                    onCancel={cancelGeneration}
                    onOpenSettings={onOpenSettings}
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

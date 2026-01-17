import { useState, useEffect } from 'react';
import { ChatView } from '@components/chat/ChatView';
import { Settings } from '@components/settings/Settings';
import { useVSCodeApi } from '@hooks/useVSCodeApi';

import { McpView } from '@components/mcp/McpView';
import { HistoryView } from './components/history/HistoryView';
import { AgentView } from './components/agent/AgentView';

type View = 'chat' | 'settings' | 'mcp' | 'history' | 'agent';

export default function App() {
    const [currentView, setCurrentView] = useState<View>('chat');
    const { onMessage } = useVSCodeApi();

    // Listen for view change requests from extension
    useEffect(() => {
        const unsubscribe = onMessage((message) => {
            if (message.type === 'open_settings') {
                setCurrentView('settings');
            } else if (message.type === 'open_agent') {
                setCurrentView('agent');
            } else if (message.type === 'open_history') {
                setCurrentView('history');
            } else if (message.type === 'open_mcp') {
                setCurrentView('mcp');
            }
        });
        return () => { unsubscribe(); };
    }, [onMessage]);

    return (
        <div className="flex flex-col h-full w-full overflow-hidden bg-vscode-sideBar-background text-vscode-fg">
            {/* ChatView is ALWAYS mounted to preserve session state */}
            <div className={currentView === 'chat' ? 'flex-1 flex flex-col overflow-hidden' : 'hidden'}>
                <ChatView
                    onOpenSettings={() => setCurrentView('settings')}
                    onOpenHistory={() => setCurrentView('history')}
                    onOpenAgent={() => setCurrentView('agent')}
                />
            </div>

            {/* Modal overlays - ChatView stays mounted behind */}
            {currentView === 'settings' && (
                <div className="absolute inset-0 z-50 bg-vscode-sideBar-background">
                    <Settings onClose={() => setCurrentView('chat')} />
                </div>
            )}
            {currentView === 'history' && (
                <div className="absolute inset-0 z-50 bg-vscode-sideBar-background">
                    <HistoryView onDone={() => setCurrentView('chat')} />
                </div>
            )}
            {currentView === 'mcp' && (
                <div className="absolute inset-0 z-50 bg-vscode-sideBar-background h-full flex flex-col">
                    <div className="p-2">
                        <button onClick={() => setCurrentView('chat')} className="text-xs hover:underline mb-2">← Back to Chat</button>
                    </div>
                    <div className="flex-1 overflow-hidden">
                        <McpView />
                    </div>
                </div>
            )}
            {currentView === 'agent' && (
                <div className="absolute inset-0 z-50 bg-vscode-sideBar-background h-full flex flex-col">
                    <div className="p-2 border-b border-vscode-contrastBorder flex justify-between items-center">
                        <button onClick={() => setCurrentView('chat')} className="text-xs hover:underline">← Back to Chat</button>
                        <span className="text-xs font-bold opacity-70">AGENT MODE</span>
                    </div>
                    <div className="flex-1 overflow-hidden bg-vscode-editor-background">
                        <AgentView />
                    </div>
                </div>
            )}
        </div>
    );
}

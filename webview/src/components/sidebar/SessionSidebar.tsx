import { SessionMetadata } from '../../types/session';
import { Plus, MessageSquare, Trash2, Clock } from 'lucide-react';

interface SessionSidebarProps {
    sessions: SessionMetadata[];
    activeSessionId: string;
    onCreateSession: () => void;
    onSwitchSession: (id: string) => void;
    onDeleteSession: (id: string) => void;
}

export function SessionSidebar({
    sessions,
    activeSessionId,
    onCreateSession,
    onSwitchSession,
    onDeleteSession
}: SessionSidebarProps) {
    const formatTime = (iso: string) => {
        const date = new Date(iso);
        return new Intl.DateTimeFormat('en-US', {
            month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
        }).format(date);
    };

    return (
        <div className="w-56 flex flex-col border-r border-vscode-border bg-ricochet-subtle h-full">
            <div className="p-3 border-b border-vscode-border flex justify-between items-center">
                <h2 className="text-[10px] font-medium uppercase tracking-widest text-vscode-fg/40">Sessions</h2>
                <button
                    onClick={onCreateSession}
                    className="p-1.5 hover:bg-ricochet-muted rounded-md transition-colors"
                    title="New Chat"
                >
                    <Plus size={14} className="text-vscode-fg/60" />
                </button>
            </div>

            <div className="flex-1 overflow-y-auto p-2 space-y-1">
                {sessions.map(session => (
                    <div
                        key={session.id}
                        className={`
                            group flex items-center justify-between p-2 rounded cursor-pointer transition-all duration-200
                            ${activeSessionId === session.id
                                ? 'bg-ricochet-primary/10 border border-ricochet-primary/20 shadow-sm'
                                : 'hover:bg-white/5 border border-transparent'}
                        `}
                        onClick={() => onSwitchSession(session.id)}
                    >
                        <div className="flex items-center gap-2 overflow-hidden">
                            <MessageSquare
                                size={14}
                                className={activeSessionId === session.id ? 'text-ricochet-primary' : 'text-vscode-desc'}
                            />
                            <div className="flex flex-col min-w-0">
                                <span className={`text-sm truncate ${activeSessionId === session.id ? 'font-medium' : ''}`}>
                                    {session.id === 'default' ? 'Default Session' : `Chat ${session.id.slice(-4)}`}
                                </span>
                                <span className="text-[10px] text-vscode-desc flex items-center gap-1">
                                    <Clock size={8} />
                                    {formatTime(new Date(session.lastModified).toISOString())}
                                </span>
                            </div>
                        </div>

                        {session.id !== 'default' && (
                            <button
                                onClick={(e) => {
                                    e.stopPropagation();
                                    onDeleteSession(session.id);
                                }}
                                className="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-500/20 hover:text-red-400 rounded transition-all"
                                title="Delete"
                            >
                                <Trash2 size={12} />
                            </button>
                        )}
                    </div>
                ))}

                {sessions.length === 0 && (
                    <div className="text-xs text-vscode-desc text-center py-4">
                        No active sessions
                    </div>
                )}
            </div>
        </div>
    );
}

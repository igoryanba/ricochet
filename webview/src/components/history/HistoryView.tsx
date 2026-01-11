import { useMemo, useState } from 'react';
import { useSessions } from '../../hooks/useSessions';
import { Trash, MessageSquare, Plus, Search, Calendar } from 'lucide-react';

interface HistoryViewProps {
    onDone: () => void;
}

export function HistoryView({ onDone }: HistoryViewProps) {
    const { sessions, deleteSession, loadSession, createSession } = useSessions();
    const [searchQuery, setSearchQuery] = useState('');

    const filteredSessions = useMemo(() => {
        if (!searchQuery) return sessions;
        const query = searchQuery.toLowerCase();
        return sessions.filter(s =>
            s.title.toLowerCase().includes(query) ||
            s.id.includes(query)
        );
    }, [sessions, searchQuery]);

    const formatDate = (timestamp: number) => {
        return new Date(timestamp).toLocaleString('en-US', {
            month: 'short',
            day: 'numeric',
            hour: 'numeric',
            minute: '2-digit'
        });
    };

    return (
        <div className="flex flex-col h-full bg-vscode-sideBar-background text-vscode-fg">
            {/* Header */}
            <div className="flex items-center justify-between p-4 border-b border-vscode-widget-border/20">
                <h2 className="text-sm font-semibold uppercase tracking-wider text-vscode-fg/80">History</h2>
                <button
                    onClick={onDone}
                    className="px-3 py-1 text-xs bg-vscode-button-background text-vscode-button-foreground hover:bg-vscode-button-hoverBackground rounded-sm transition-colors"
                >
                    Done
                </button>
            </div>

            {/* Search & Actions */}
            <div className="p-4 space-y-3">
                <div className="relative">
                    <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-vscode-fg/40" />
                    <input
                        type="text"
                        placeholder="Search history..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="w-full pl-9 pr-3 py-1.5 bg-vscode-input-background text-vscode-input-foreground border border-vscode-input-border rounded-sm focus:outline-none focus:ring-1 focus:ring-vscode-focusBorder placeholder:text-vscode-input-placeholderForeground text-sm"
                    />
                </div>

                <button
                    onClick={() => {
                        createSession();
                        onDone();
                    }}
                    className="w-full flex items-center justify-center gap-2 py-2 bg-vscode-button-secondaryBackground text-vscode-button-secondaryForeground hover:bg-vscode-button-secondaryHoverBackground rounded-sm transition-colors text-sm font-medium"
                >
                    <Plus className="w-4 h-4" />
                    New Chat
                </button>
            </div>

            {/* List */}
            <div className="flex-1 overflow-y-auto px-2 pb-4">
                {filteredSessions.length === 0 ? (
                    <div className="flex flex-col items-center justify-center h-40 text-vscode-fg/40">
                        <MessageSquare className="w-8 h-8 mb-2 opacity-50" />
                        <span className="text-sm">No history found</span>
                    </div>
                ) : (
                    <div className="space-y-1">
                        {filteredSessions.map((session) => (
                            <div
                                key={session.id}
                                className="group relative flex flex-col gap-1 p-3 rounded-md hover:bg-vscode-list-hoverBackground cursor-pointer border border-transparent hover:border-vscode-list-focusOutline transition-all"
                                onClick={() => {
                                    loadSession(session.id);
                                    onDone();
                                }}
                            >
                                <div className="flex items-start justify-between">
                                    <h3 className="text-sm font-medium text-vscode-fg/90 line-clamp-2 pr-6">
                                        {session.title || 'Untitled Chat'}
                                    </h3>
                                    <button
                                        onClick={(e) => {
                                            e.stopPropagation();
                                            deleteSession(session.id);
                                        }}
                                        className="opacity-0 group-hover:opacity-100 p-1 text-vscode-fg/40 hover:text-red-400 transition-all absolute top-2 right-2"
                                        title="Delete Session"
                                    >
                                        <Trash className="w-4 h-4" />
                                    </button>
                                </div>

                                <div className="flex items-center gap-4 text-xs text-vscode-fg/50 mt-1">
                                    <div className="flex items-center gap-1">
                                        <Calendar className="w-3 h-3" />
                                        <span>{formatDate(session.lastModified)}</span>
                                    </div>
                                    <div className="flex items-center gap-1">
                                        <MessageSquare className="w-3 h-3" />
                                        <span>{session.messageCount} msgs</span>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
}

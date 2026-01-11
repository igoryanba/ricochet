import { useEffect, useRef } from 'react';
import { AgentLogEntry } from '../../services/state-machine/sessionStateMachine';
import { Terminal, CheckCircle, AlertCircle, Info } from 'lucide-react';

interface AgentLogProps {
    logs: AgentLogEntry[];
}

export function AgentLog({ logs }: AgentLogProps) {
    const endRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        endRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [logs]);

    return (
        <div className="flex flex-col gap-2 p-4 overflow-y-auto flex-1 font-mono text-xs">
            {logs.map((log) => (
                <div key={log.id} className={`flex gap-3 p-2 rounded border border-transparent ${getLogStyle(log.type)}`}>
                    <div className="mt-0.5 shrink-0 opacity-70">
                        {getLogIcon(log.type)}
                    </div>
                    <div className="flex-1 whitespace-pre-wrap break-words">
                        {log.type === 'tool_call' ? (
                            <div className="flex flex-col gap-1">
                                <span className="font-bold text-vscode-textLink-foreground">Executing Tool: {log.metadata?.name}</span>
                                <div className="p-2 bg-vscode-editor-background rounded border border-vscode-widget-border opacity-80">
                                    {JSON.stringify(log.metadata?.args, null, 2)}
                                </div>
                            </div>
                        ) : (
                            <span>{log.content}</span>
                        )}
                    </div>
                    <div className="text-[10px] opacity-40 shrink-0 select-none">
                        {new Date(log.timestamp).toLocaleTimeString()}
                    </div>
                </div>
            ))}
            <div ref={endRef} />
        </div>
    );
}

function getLogIcon(type: AgentLogEntry['type']) {
    switch (type) {
        case 'info': return <Info size={14} />;
        case 'tool_call': return <Terminal size={14} />;
        case 'tool_result': return <CheckCircle size={14} />;
        case 'error': return <AlertCircle size={14} />;
        default: return <Info size={14} />;
    }
}

function getLogStyle(type: AgentLogEntry['type']) {
    switch (type) {
        case 'tool_call': return 'bg-vscode-list-hoverBackground border-vscode-list-selectionBackground';
        case 'error': return 'bg-red-500/10 border-red-500/20 text-red-400';
        case 'user': return 'opacity-60';
        default: return '';
    }
}

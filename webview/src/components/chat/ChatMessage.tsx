import { useState, useMemo, useEffect, useRef } from 'react';
import { ChevronDown, ChevronRight, ChevronUp, Search, FileText, Edit3, Terminal, RotateCcw } from 'lucide-react';
import { ChatMessage as ChatMessageType, ToolCall, ActivityItem } from '@hooks/useChat';
import { useVSCodeApi } from '@hooks/useVSCodeApi';
import { DiffView, parseDiff } from '../diff/DiffView';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

interface ChatMessageProps {
    message: ChatMessageType;
    onExecuteCommand?: (command: string) => void;
    onRestore?: (hash: string) => void;
}

const PROGRESS_STYLE = `
@keyframes fadeIn {
    from { opacity: 0; transform: translateY(4px); }
    to { opacity: 1; transform: translateY(0); }
}
.animate-fade-in {
    animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1) forwards;
}
`;

/**
 * Chat message message component with markdown rendering.
 * Matches competitor styling with code blocks and reasoning sections.
 */
export function ChatMessage({ message, onExecuteCommand, onRestore }: ChatMessageProps) {
    const isUser = message.role === 'user';

    return (
        <div className={`py-4 px-2 transition-colors ${!isUser ? 'bg-vscode-sideBar-background/30' : ''}`}>
            <style>{PROGRESS_STYLE}</style>
            {!isUser ? (
                <div className="flex flex-col gap-1 px-4">
                    <div className="flex items-center gap-2 mb-1 pl-1 opacity-50 hover:opacity-100 transition-opacity">
                        <div className="text-[9px] font-bold text-ricochet-primary/60 uppercase tracking-widest bg-ricochet-primary/5 px-1.5 py-0.5 rounded border border-ricochet-primary/10">
                            AGENT
                        </div>
                        {message.via && message.via !== 'ide' && (
                            <span className="text-[9px] font-medium text-blue-400/80 uppercase tracking-widest flex items-center gap-1.5 opacity-80" title={`via ${message.via}`}>
                                <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse shadow-[0_0_8px_rgba(96,165,250,0.5)]" />
                                {message.via === 'telegram' ? 'TELEGRAM' : message.via === 'discord' ? 'DISCORD' : message.via}
                            </span>
                        )}
                        {message.checkpointHash && onRestore && (
                            <button
                                onClick={() => onRestore(message.checkpointHash!)}
                                className="ml-auto text-[9px] font-medium text-white/40 hover:text-white/70 uppercase tracking-widest flex items-center gap-1 transition-colors bg-white/5 px-2 py-0.5 rounded hover:bg-white/10"
                                title={`Restore workspace to checkpoint ${message.checkpointHash.slice(0, 8)}`}
                            >
                                <RotateCcw className="w-3 h-3" />
                                Restore
                            </button>
                        )}
                    </div>
                    <div className="animate-fade-in">
                        <AssistantContent message={message} onExecuteCommand={onExecuteCommand} />
                    </div>
                </div>
            ) : (
                <UserContent content={message.content} via={message.via} remoteUsername={message.remoteUsername} />
            )}
        </div>
    );
}

const UserContent = ({ content, via, remoteUsername }: { content: string; via?: 'telegram' | 'discord' | 'ide'; remoteUsername?: string }) => {
    return (
        <div className="flex flex-col items-end w-full px-4 mb-2">
            <div className="flex items-center gap-2 mb-1 px-1">
                {remoteUsername && (
                    <span className="text-[10px] text-vscode-fg/40 font-medium">{remoteUsername}</span>
                )}
                {via && via !== 'ide' && (
                    <span className="inline-flex items-center gap-1 text-[9px] text-blue-400 font-bold uppercase tracking-wider" title={`via ${via}`}>
                        {via === 'telegram' ? 'TELEGRAM' : via === 'discord' ? 'DISCORD' : via}
                    </span>
                )}
            </div>
            <div
                className="max-w-[90%] py-2.5 px-4 rounded-2xl rounded-tr-sm whitespace-pre-wrap text-[14px] font-medium leading-relaxed border border-white/5 shadow-sm"
                style={{
                    backgroundColor: 'rgba(255, 255, 255, 0.05)',
                    color: '#ffffff',
                }}
            >
                {content}
            </div>
        </div>
    );
};

function AssistantContent({ message, onExecuteCommand }: { message: ChatMessageType; onExecuteCommand?: (cmd: string) => void }) {
    const { thinking, body, isPlan } = useMemo(() => parseContent(message.content), [message.content]);

    return (
        <div className="text-[14px]">
            {((message.activities && message.activities.length > 0) || (message.toolCalls && message.toolCalls.length > 0)) && (
                <ProgressBlock activities={message.activities || []} toolCalls={message.toolCalls || []} />
            )}

            {thinking && <ReasoningBlock content={thinking} isStreaming={message.isStreaming} />}

            {isPlan && (
                <ImplementationPlanCard />
            )}

            <div className="text-vscode-fg leading-relaxed mt-2 overflow-hidden max-w-none space-y-2">
                <MarkdownContent content={body} onExecuteCommand={onExecuteCommand} />
                {message.isStreaming && !thinking && (
                    <span className="ml-1 inline-flex w-1.5 h-3.5 bg-ricochet-primary/60 animate-pulse align-middle shadow-[0_0_8px_rgba(var(--ricochet-primary-rgb),0.4)]" />
                )}
            </div>
        </div>
    );
}

/**
 * Simple markdown renderer for code blocks and inline code.
 * Matches competitor styling without external dependencies.
 */
/**
 * Simple markdown renderer for code blocks and inline code.
 * Matches competitor styling without external dependencies.
 */
function MarkdownContent({ content, onExecuteCommand }: { content: string; onExecuteCommand?: (cmd: string) => void }) {
    return (
        <ReactMarkdown
            remarkPlugins={[remarkGfm]}
            components={{
                code({ node, inline, className, children, ...props }: any) {
                    const match = /language-(\w+)/.exec(className || '');
                    const code = String(children).replace(/\n$/, '');

                    if (!inline) {
                        return (
                            <CodeBlock
                                language={match ? match[1] : 'text'}
                                code={code}
                                onExecuteCommand={onExecuteCommand}
                            />
                        );
                    }

                    return (
                        <code
                            className="px-1.5 py-0.5 rounded text-[12px] bg-vscode-textCodeBlock-background text-vscode-textPreformat-foreground font-mono border border-white/5"
                            {...props}
                        >
                            {children}
                        </code>
                    );
                },
                p: ({ children }) => <p className="mb-3 last:mb-0 leading-relaxed text-[13.5px]">{children}</p>,
                ul: ({ children }) => <ul className="list-disc ml-4 mb-3 space-y-1">{children}</ul>,
                ol: ({ children }) => <ol className="list-decimal ml-4 mb-3 space-y-1">{children}</ol>,
                li: ({ children }) => (
                    <li className="leading-relaxed text-[13.5px] opacity-90 pl-1 mb-1">
                        <div className="inline-block align-top">{children}</div>
                    </li>
                ),
                h1: ({ children }) => <h1 className="text-[17px] font-bold mb-3 mt-5 text-white/90 border-b border-white/5 pb-1">{children}</h1>,
                h2: ({ children }) => <h2 className="text-[15px] font-bold mb-2 mt-4 text-white/90">{children}</h2>,
                h3: ({ children }) => <h3 className="text-[13px] font-bold mb-2 mt-3 text-white/80">{children}</h3>,
                strong: ({ children }) => <strong className="font-bold text-white/95">{children}</strong>,
                a: ({ node, ...props }) => (
                    <a className="text-ricochet-primary/90 hover:text-ricochet-primary hover:underline transition-colors" {...props} target="_blank" rel="noopener noreferrer" />
                ),
                blockquote: ({ children }) => (
                    <blockquote className="border-l-2 border-white/10 pl-4 py-1 my-3 italic text-vscode-fg/60 bg-white/[0.02] rounded-r">
                        {children}
                    </blockquote>
                ),
            }}
        >
            {content}
        </ReactMarkdown>
    );
}

function CodeBlock({ language, code, onExecuteCommand }: { language: string; code: string; onExecuteCommand?: (cmd: string) => void }) {
    const isTerminal = ['sh', 'bash', 'zsh', 'console', 'terminal', 'cmd', 'powershell', 'shell'].includes(language.toLowerCase());
    const lineCount = useMemo(() => code.split('\n').length, [code]);
    const shouldCollapse = lineCount > 10;
    const isShortText = language.toLowerCase() === 'text' && lineCount <= 2 && code.length < 150;
    const [isExpanded, setIsExpanded] = useState(!shouldCollapse);
    const { postMessage } = useVSCodeApi();

    if (code.trim() === '') return null;

    if (isShortText) {
        return (
            <div className="inline-block my-1 px-1.5 py-0.5 bg-white/5 rounded border border-white/5 font-mono text-[11px] text-vscode-fg/50 whitespace-normal break-all align-middle group/snippet hover:bg-white/10 transition-colors">
                {code.trim()}
            </div>
        );
    }

    if (language.toLowerCase() === 'diff') {
        const diffs = parseDiff(code);
        if (diffs.length > 0) {
            return (
                <div className="my-2">
                    <DiffView
                        diffs={diffs}
                        onApprove={() => postMessage({ type: 'send_message', payload: { content: 'I approve these changes.' } })}
                        onReject={() => postMessage({ type: 'send_message', payload: { content: 'I reject these changes.' } })}
                        onViewInVSCode={(path) => {
                            // Since we don't have the "new content" easily for a partial diff in markdown,
                            // we just open the file for now. 
                            // In tool calls we have the full content.
                            postMessage({ type: 'open_file', payload: { path } });
                        }}
                    />
                </div>
            );
        }
    }

    return (
        <div className={`rounded-md overflow-hidden border border-white/5 my-2 ${isTerminal ? 'bg-black/40' : 'bg-[#2d2d2d]/30'}`}>
            {/* Header */}
            <div
                className="flex items-center justify-between px-3 py-1.5 bg-vscode-sideBar-background cursor-pointer group"
                onClick={() => setIsExpanded(!isExpanded)}
            >
                <div className="flex items-center gap-2">
                    <span className="text-xs text-vscode-fg/50 font-mono uppercase">{language || 'code'}</span>
                </div>

                <div className="flex items-center gap-2">
                    {/* Run Button for shell scripts */}
                    {isTerminal && onExecuteCommand && (
                        <button
                            onClick={(e) => {
                                e.stopPropagation();
                                onExecuteCommand(code);
                            }}
                            className="px-2 py-0.5 hover:bg-vscode-list-hoverBackground rounded text-[10px] text-ricochet-primary font-bold transition-colors border border-ricochet-primary/20"
                            title="Run in Terminal"
                        >
                            RUN
                        </button>
                    )}

                    {isExpanded ? (
                        <ChevronDown className="w-3 h-3 text-vscode-fg/40" />
                    ) : (
                        <ChevronRight className="w-3 h-3 text-vscode-fg/40" />
                    )}
                </div>
            </div>

            {/* Code content */}
            <div className="relative">
                <SyntaxHighlighter
                    language={language.toLowerCase()}
                    style={vscDarkPlus}
                    customStyle={{
                        margin: 0,
                        padding: '12px',
                        fontSize: '11px',
                        lineHeight: '1.5',
                        backgroundColor: 'transparent',
                    }}
                    className={`custom-scrollbar ${!isExpanded ? 'max-h-[80px] opacity-40 overflow-hidden' : 'overflow-x-auto'}`}
                >
                    {code}
                </SyntaxHighlighter>

                {/* Terminal Notch / Toggle for long outputs */}
                {!isExpanded && shouldCollapse && (
                    <div
                        className="absolute bottom-0 left-0 right-0 h-8 flex items-end justify-center pb-1 pointer-events-none"
                        style={{ background: 'linear-gradient(transparent, var(--vscode-sideBar-background))' }}
                    >
                        <button
                            onClick={(e) => { e.stopPropagation(); setIsExpanded(true); }}
                            className="pointer-events-auto flex items-center gap-1.5 px-3 py-0.5 rounded-full bg-vscode-descriptionForeground/80 text-vscode-sideBar-background text-[10px] font-bold hover:bg-vscode-descriptionForeground transition-all"
                        >
                            <ChevronDown className="w-3 h-3" />
                            SHOW {lineCount - 4} MORE LINES
                        </button>
                    </div>
                )}

                {isExpanded && shouldCollapse && (
                    <div className="flex justify-center pb-2">
                        <button
                            onClick={(e) => { e.stopPropagation(); setIsExpanded(false); }}
                            className="flex items-center gap-1.5 px-3 py-0.5 rounded-full bg-vscode-descriptionForeground/20 text-vscode-fg/40 text-[10px] font-bold hover:bg-vscode-descriptionForeground/30 transition-all border border-white/5"
                        >
                            <ChevronUp className="w-3 h-3" />
                            COLLAPSE
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
}



function ReasoningBlock({ content, isStreaming }: { content: string; isStreaming?: boolean }) {
    const [isExpanded, setIsExpanded] = useState(false);
    const startTimeRef = useRef<number>(Date.now());
    const [elapsed, setElapsed] = useState<number>(0);

    useEffect(() => {
        if (isStreaming) {
            const tick = () => setElapsed(Math.floor((Date.now() - startTimeRef.current) / 1000));
            const id = setInterval(tick, 1000);
            return () => clearInterval(id);
        }
    }, [isStreaming]);

    return (
        <div className="mb-4 bg-vscode-textCodeBlock-background/20 rounded-lg p-1">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="w-full flex items-center gap-2 py-2 px-3 hover:bg-white/[0.03] rounded-lg transition-all group border border-white/[0.02] hover:border-white/5"
            >
                <div className="w-2 h-2 rounded-full bg-ricochet-primary/40 animate-pulse mr-1" />
                <span className="text-[10px] font-black uppercase tracking-[0.15em] text-vscode-fg/40 flex items-center gap-2">
                    THOUGHT PROCESS
                    {elapsed > 0 && <span className="font-mono opacity-30 tracking-normal">[{elapsed}s]</span>}
                </span>
                {isStreaming && (
                    <span className="mx-2 flex gap-1">
                        <span className="w-1 h-1 bg-ricochet-primary/60 rounded-full animate-bounce [animation-delay:-0.3s]" />
                        <span className="w-1 h-1 bg-ricochet-primary/60 rounded-full animate-bounce [animation-delay:-0.15s]" />
                        <span className="w-1 h-1 bg-ricochet-primary/60 rounded-full animate-bounce" />
                    </span>
                )}
                <div className="flex-1" />
                {isExpanded ? (
                    <ChevronDown className="w-3.5 h-3.5 text-vscode-fg/30 group-hover:text-vscode-fg/50 transition-colors" />
                ) : (
                    <ChevronRight className="w-3.5 h-3.5 text-vscode-fg/30 group-hover:text-vscode-fg/50 transition-colors" />
                )}
            </button>
            {isExpanded && (
                <div className="px-5 py-3 mt-1 text-[13px] text-vscode-fg/70 italic whitespace-pre-wrap leading-relaxed border-l-2 border-ricochet-primary/30 ml-3 mb-2 bg-black/10 rounded-r-lg">
                    {content.trim()}
                </div>
            )}
        </div>
    );
}

function ProgressBlock({ activities, toolCalls }: { activities: ActivityItem[]; toolCalls: ToolCall[] }) {
    const { postMessage } = useVSCodeApi();
    const [isExpanded, setIsExpanded] = useState(true);

    const getActivityIcon = (type: ActivityItem['type']) => {
        switch (type) {
            case 'search': return <Search className="w-3 h-3" />;
            case 'analyze': return <FileText className="w-3 h-3" />;
            case 'edit': return <Edit3 className="w-3 h-3" />;
            case 'command': return <Terminal className="w-3 h-3" />;
        }
    };

    const getActivityLabel = (activity: ActivityItem) => {
        const fileName = activity.file?.split('/').pop() || activity.file;
        switch (activity.type) {
            case 'search':
                return (
                    <div className="flex items-center gap-2 overflow-hidden">
                        <span className="text-vscode-fg/50 flex-shrink-0">Searched</span>
                        <span className="text-blue-400 font-mono truncate">{activity.query}</span>
                        {activity.results !== undefined && (
                            <span className="text-vscode-fg/30 flex-shrink-0 ml-1">— {activity.results} results</span>
                        )}
                    </div>
                );
            case 'analyze':
                return (
                    <div className="flex items-center gap-2 overflow-hidden">
                        <span className="text-vscode-fg/40 flex-shrink-0">Analyzed</span>
                        <button
                            className="text-blue-400/80 hover:text-blue-400 hover:underline font-mono transition-colors truncate"
                            onClick={() => postMessage({ type: 'open_file', payload: { path: activity.file } })}
                        >
                            {fileName}
                        </button>
                        {activity.lineRange && <span className="text-vscode-fg/20 flex-shrink-0 text-[9px]">#{activity.lineRange}</span>}
                    </div>
                );
            case 'edit':
                return (
                    <div className="flex items-center gap-2 overflow-hidden">
                        <span className="text-vscode-fg/50 flex-shrink-0">Edited</span>
                        <button
                            className="text-blue-400 hover:underline font-mono truncate"
                            onClick={() => postMessage({ type: 'open_file', payload: { path: activity.file } })}
                        >
                            {fileName}
                        </button>
                        {(activity.additions !== undefined || activity.deletions !== undefined) && (
                            <span className="flex-shrink-0 flex items-center gap-1 text-[10px] tabular-nums bg-white/5 px-1 rounded-sm">
                                {activity.additions !== undefined && <span className="text-green-400">+{activity.additions}</span>}
                                {activity.deletions !== undefined && <span className="text-red-400">-{activity.deletions}</span>}
                            </span>
                        )}
                    </div>
                );
            case 'command':
                return <span className="text-vscode-fg/50">Ran command</span>;
        }
    };

    const getToolInfo = (name: string) => {
        const n = name.toLowerCase();
        if (n.includes('read') || n.includes('view_file')) return { label: 'READ', color: 'text-blue-400' };
        if (n.includes('edit') || n.includes('write')) return { label: 'EDIT', color: 'text-green-400' };
        if (n.includes('search') || n.includes('grep')) return { label: 'FIND', color: 'text-yellow-400' };
        if (n.includes('run') || n.includes('exec') || n.includes('command')) return { label: 'CMD', color: 'text-purple-400' };
        if (n.includes('list') || n.includes('ls')) return { label: 'LIST', color: 'text-gray-400' };
        return { label: 'TOOL', color: 'text-vscode-fg/30' };
    };

    const getToolRow = (tool: ToolCall) => {
        const info = getToolInfo(tool.name);
        let path: string | undefined;
        try {
            // tool.arguments is a stringified JSON
            const args = typeof tool.arguments === 'string' ? JSON.parse(tool.arguments) : tool.arguments;
            path = args.path || args.TargetFile || args.AbsolutePath || args.file || args.TargetContent;
        } catch (e) { }

        const fileName = (typeof path === 'string' && path.includes('/') ? path.split('/').pop() : path) || null;

        return (
            <div className="flex items-center gap-2 overflow-hidden flex-1 group/tool">
                <span className={`text-[8px] font-bold ${info.color} opacity-60 w-8 text-right flex-shrink-0 group-hover/tool:opacity-100 transition-opacity`}>{info.label}</span>
                <span className="truncate opacity-50 font-mono text-[9px] group-hover/tool:opacity-80 transition-opacity" title={tool.name}>{tool.name}</span>
                {fileName && typeof fileName === 'string' && (
                    <button
                        className="text-[9px] text-blue-400 hover:text-blue-300 hover:underline px-0.5 py-0.5 truncate transition-all flex items-center gap-1"
                        onClick={(e) => {
                            e.stopPropagation();
                            if (typeof path === 'string') {
                                postMessage({ type: 'open_file', payload: { path: path.split('#')[0] } });
                            }
                        }}
                    >
                        <FileText className="w-2.5 h-2.5 opacity-50" />
                        {fileName}
                    </button>
                )}
                {tool.status === 'running' && <span className="w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse ml-auto mr-1" />}
                {tool.status === 'completed' && (
                    <div className="flex items-center gap-1 ml-auto mr-1">
                        {(tool.name === 'write_to_file' || tool.name === 'replace_file_content') && (
                            <button
                                className="p-1 hover:bg-white/10 rounded transition-colors text-vscode-fg/40 hover:text-green-400"
                                title="View Diff in VSCode"
                                onClick={(e) => {
                                    e.stopPropagation();
                                    const args = typeof tool.arguments === 'string' ? JSON.parse(tool.arguments) : tool.arguments;
                                    postMessage({
                                        type: 'show_native_diff',
                                        payload: {
                                            path: args.TargetFile,
                                            newContent: args.CodeContent || args.ReplacementContent,
                                            targetContent: args.TargetContent
                                        }
                                    });
                                }}
                            >
                                <Edit3 className="w-3 h-3" />
                            </button>
                        )}
                        <div className="w-1 h-1 rounded-full bg-green-500/30" />
                    </div>
                )}
            </div>
        );
    };

    // Consolidate list for rendering

    // Consolidate list for rendering

    if (!isExpanded) {
        return (
            <button
                onClick={() => setIsExpanded(true)}
                className="mb-2 flex items-center gap-2 px-2 py-0.5 rounded hover:bg-white/5 text-[10px] text-vscode-fg/30 uppercase tracking-widest font-black transition-all"
            >
                <ChevronRight className="w-3 h-3 opacity-40" />
                Trace Log
                <span className="opacity-20 ml-1">[{activities.length + toolCalls.length}]</span>
            </button>
        );
    }

    return (
        <div className="mb-4 overflow-hidden animate-fade-in max-w-full">
            <div className="flex flex-col gap-1.5 relative pl-3 before:content-[''] before:absolute before:left-0 before:top-2 before:bottom-2 before:w-[1px] before:bg-white/5">
                {/* Render Activities First (High Level) */}
                {activities.map((activity, i) => (
                    <div key={`act-${i}`} className="flex items-center gap-2 text-[11px] text-vscode-fg/60 hover:text-vscode-fg/80 transition-opacity">
                        <span className="text-vscode-fg/20 flex-shrink-0">{getActivityIcon(activity.type)}</span>
                        <div className="flex-1 min-w-0">{getActivityLabel(activity)}</div>
                    </div>
                ))}

                {/* Render Tool Calls (Trace) - Only if notable or no activities */}
                {(activities.length === 0 || toolCalls.length > activities.length) && (
                    <div className="flex flex-col gap-0.5 mt-1 opacity-40 hover:opacity-100 transition-opacity">
                        {toolCalls.map((tool, i) => {
                            // Basic heuristic: if it's update_todos, it's internal noise
                            if (tool.name === 'update_todos') return null;
                            return (
                                <div key={`tool-${i}`} className="flex items-center gap-2 text-[9px] font-mono whitespace-nowrap overflow-hidden">
                                    <span className="w-1 h-1 rounded-full bg-vscode-fg/20" />
                                    {getToolRow(tool)}
                                </div>
                            );
                        })}
                    </div>
                )}
            </div>
        </div>
    );
}

function ImplementationPlanCard() {
    const { postMessage } = useVSCodeApi();

    return (
        <div className="my-4 p-4 bg-vscode-editor-background border border-ricochet-primary/20 rounded-lg shadow-sm space-y-4">
            <div className="flex items-start gap-3">
                <div className="flex-1 min-w-0">
                    <div className="text-[9px] font-black tracking-tighter text-ricochet-primary uppercase mb-1">PROPOSED PLAN</div>
                    <h3 className="text-sm font-bold text-vscode-fg leading-tight">Implementation Plan</h3>
                    <p className="text-[11px] text-vscode-fg/40 mt-1 line-clamp-2 italic">
                        Technical breakdown and proposed changes for the current task.
                    </p>
                </div>
            </div>

            <div className="flex items-center gap-2">
                <button
                    className="flex-1 flex items-center justify-center py-2 px-3 bg-vscode-button-secondaryBackground hover:bg-vscode-button-secondaryHover rounded text-xs font-bold tracking-wide transition-all border border-white/5 text-vscode-fg"
                    onClick={() => {
                        postMessage({ type: 'open_file', payload: { path: '.gemini/antigravity/brain/d85108ee-0f2c-494a-8fed-209f639b42ce/implementation_plan.md' } });
                    }}
                >
                    VIEW FILE
                </button>
                <button
                    className="flex-1 flex items-center justify-center py-2 px-3 bg-ricochet-primary text-white hover:opacity-90 rounded text-xs font-black tracking-wide transition-all shadow-md"
                    onClick={() => {
                        postMessage({ type: 'send_message', payload: { content: 'I approve this plan. Proceed with execution.' } });
                    }}
                >
                    APPROVE
                </button>
            </div>
        </div>
    );
}



function parseContent(content: string) {
    let thinking = "";
    let body = "";
    let lastIndex = 0;

    // Regular expression for <thinking> blocks (handles open/streaming ones too)
    const thinkingRegex = /<thinking>([\s\S]*?)(?:<\/thinking>|$)/g;
    let match;

    while ((match = thinkingRegex.exec(content)) !== null) {
        // Append prefix text to body
        body += content.substring(lastIndex, match.index);
        // Append thinking content
        thinking += (thinking ? "\n\n" : "") + match[1].trim();
        // Update lastIndex to end of match
        lastIndex = match.index + match[0].length;

        // If the match didn't find a closing tag, we've hit the end of the current content
        if (!match[0].endsWith('</thinking>')) break;
    }

    // Append remaining text
    body += content.substring(lastIndex);

    // Temporarily disabled: isPlan was triggering on any mention of "план" in Russian
    // TODO: Implement proper logic that checks for actual implementation_plan.md file in the session
    const isPlan = false;

    return {
        thinking: thinking.trim() || null,
        body: body.trim(),
        isPlan
    };
}

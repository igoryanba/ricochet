import { useState, useMemo, useEffect, useRef } from 'react';
import { ChevronDown, ChevronRight, ChevronUp, FileText, Edit3, Terminal, RotateCcw } from 'lucide-react';
import { ChatMessage as ChatMessageType, ToolCall, ActivityItem, TaskProgress } from '@hooks/useChat';
import { useVSCodeApi } from '@hooks/useVSCodeApi';
import { DiffView, parseDiff } from '../diff/DiffView';
import { TaskProgressCard, ArtifactCard, InlineActivity } from './TaskProgressCard';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

interface ChatMessageProps {
    message: ChatMessageType;
    taskProgress?: TaskProgress | null;
    onExecuteCommand?: (command: string) => void;
    onRestore?: (hash: string) => void;
}



/**
 * Chat message message component with markdown rendering.
 * Matches competitor styling with code blocks and reasoning sections.
 */
export function ChatMessage({ message, taskProgress, onExecuteCommand, onRestore }: ChatMessageProps) {
    const isUser = message.role === 'user';

    return (
        <div className={`py-4 px-2 transition-colors ${!isUser ? 'bg-vscode-sideBar-background/30' : ''}`}>
            {!isUser ? (
                <div className="px-4 animate-fade-in">
                    <div className="flex items-center gap-2 mb-1 pl-1 opacity-50 hover:opacity-100 transition-opacity">
                        {message.checkpointHash && onRestore && (
                            <button
                                onClick={() => onRestore(message.checkpointHash!)}
                                className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-white/5 hover:bg-white/10 border border-white/5 text-[9px] font-medium text-white/50 hover:text-white/80 uppercase tracking-wider transition-all backdrop-blur-sm"
                                title={`Restore workspace to checkpoint ${message.checkpointHash.slice(0, 8)}`}
                            >
                                <RotateCcw className="w-3 h-3" />
                                Restore
                            </button>
                        )}
                    </div>
                    <AssistantContent message={message} taskProgress={taskProgress} onExecuteCommand={onExecuteCommand} />
                </div>
            ) : (
                <UserContent content={message.content} via={message.via} remoteUsername={message.remoteUsername} />
            )}
        </div>
    );
}

const UserContent = ({ content, via, remoteUsername }: { content: string; via?: 'telegram' | 'discord' | 'ide'; remoteUsername?: string }) => {
    // Don't show username if it's just the same as the via badge (e.g. "Telegram")
    const isRedundantName = remoteUsername && via && remoteUsername.toLowerCase() === via.toLowerCase();

    return (
        <div className="flex flex-col items-end w-full px-4 mb-2">
            <div className="flex items-center gap-2 mb-1 px-1">
                {!isRedundantName && remoteUsername && (
                    <span className="text-[10px] text-vscode-fg/40 font-medium">{remoteUsername}</span>
                )}
                {via && via !== 'ide' && (
                    <span className="inline-flex items-center gap-1 text-[9px] text-blue-400 font-bold uppercase tracking-wider" title={`via ${via}`}>
                        {via === 'telegram' ? 'TELEGRAM' : via === 'discord' ? 'DISCORD' : via}
                    </span>
                )}
            </div>
            <div
                className="max-w-[90%] py-2.5 px-4 rounded-2xl rounded-tr-sm whitespace-pre-wrap text-[12px] font-medium leading-relaxed border border-white/10 shadow-lg backdrop-blur-md relative overflow-hidden"
                style={{
                    backgroundColor: 'rgba(255, 255, 255, 0.08)',
                    color: '#ffffff',
                }}
            >
                {/* Glossy gradient overlay */}
                <div className="absolute inset-0 bg-gradient-to-br from-white/10 to-transparent opacity-30 pointer-events-none" />
                {content}
            </div>
        </div>
    );
};


function AssistantContent({ message, taskProgress, onExecuteCommand }: {
    message: ChatMessageType;
    taskProgress?: TaskProgress | null;
    onExecuteCommand?: (cmd: string) => void
}) {
    const { thinking, body, artifacts } = useMemo(() => parseContent(message.content), [message.content]);
    const { postMessage } = useVSCodeApi();

    return (
        <div className="text-[12px]">
            {/* Task Progress Card - Antigravity-style structured progress */}
            {taskProgress && (
                <TaskProgressCard
                    taskName={taskProgress.task_name}
                    summary={taskProgress.summary || ''}
                    mode={taskProgress.mode as any || 'execution'}
                    steps={taskProgress.steps}
                    filesEdited={taskProgress.files}
                    isActive={taskProgress.is_active}
                />
            )}

            {thinking && <ReasoningBlock content={thinking} isStreaming={message.isStreaming} />}

            {((message.activities && message.activities.length > 0) || (message.toolCalls && message.toolCalls.length > 0)) && (
                <ProgressBlock activities={message.activities || []} toolCalls={message.toolCalls || []} />
            )}

            {artifacts.map((artifact, i) => (
                <ArtifactCard
                    key={i}
                    type={artifact.type}
                    title={artifact.title}
                    summary={artifact.summary}
                    onOpen={() => postMessage({ type: 'open_file', payload: { path: artifact.path } })}
                />
            ))}

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
    const components = useMemo(() => ({
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
        p: ({ children }: any) => <p className="mb-3 last:mb-0 leading-relaxed text-[13.5px]">{children}</p>,
        ul: ({ children }: any) => <ul className="list-disc ml-4 mb-3 space-y-1">{children}</ul>,
        ol: ({ children }: any) => <ol className="list-decimal ml-4 mb-3 space-y-1">{children}</ol>,
        li: ({ children }: any) => (
            <li className="leading-relaxed text-[13.5px] opacity-90 pl-1 mb-1">
                <div className="inline-block align-top">{children}</div>
            </li>
        ),
        h1: ({ children }: any) => <h1 className="text-[17px] font-bold mb-3 mt-5 text-white/90 border-b border-white/5 pb-1">{children}</h1>,
        h2: ({ children }: any) => <h2 className="text-[15px] font-bold mb-2 mt-4 text-white/90">{children}</h2>,
        h3: ({ children }: any) => <h3 className="text-[13px] font-bold mb-2 mt-3 text-white/80">{children}</h3>,
        strong: ({ children }: any) => <strong className="font-bold text-white/95">{children}</strong>,
        a: ({ node, ...props }: any) => (
            <a className="text-ricochet-primary/90 hover:text-ricochet-primary hover:underline transition-colors" {...props} target="_blank" rel="noopener noreferrer" />
        ),
        blockquote: ({ children }: any) => (
            <blockquote className="border-l-2 border-white/10 pl-4 py-1 my-3 italic text-vscode-fg/60 bg-white/[0.02] rounded-r">
                {children}
            </blockquote>
        ),
    }), [onExecuteCommand]);

    return (
        <ReactMarkdown
            remarkPlugins={[remarkGfm]}
            components={components}
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
        <div className={`rounded-xl overflow-hidden border border-white/10 my-3 shadow-sm backdrop-blur-sm ${isTerminal ? 'bg-black/60' : 'bg-[#1e1e1e]/40'}`}>
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

    // Format elapsed time as "Xs" or "Xm Ys"
    const formatTime = (seconds: number) => {
        if (seconds < 60) return `${seconds}s`;
        const mins = Math.floor(seconds / 60);
        const secs = seconds % 60;
        return `${mins}m ${secs}s`;
    };

    // Highlight backtick-wrapped references (files, tools) with gray background
    const renderContent = (text: string) => {
        const parts = text.split(/(`[^`]+`)/g);
        return parts.map((part, i) => {
            if (part.startsWith('`') && part.endsWith('`')) {
                const code = part.slice(1, -1);
                return (
                    <span key={i} className="px-1 py-0.5 bg-white/5 rounded text-vscode-fg/60 font-mono text-[11px]">
                        {code}
                    </span>
                );
            }
            return <span key={i}>{part}</span>;
        });
    };

    return (
        <div className="mb-3">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="w-full flex items-center gap-2 py-1 hover:opacity-70 transition-opacity group"
            >
                <span className="text-[11px] text-vscode-fg/40 font-medium">
                    Thought for {formatTime(elapsed)}
                </span>
                <div className="flex-1" />
                {isExpanded ? (
                    <ChevronDown className="w-3 h-3 text-vscode-fg/30" />
                ) : (
                    <ChevronRight className="w-3 h-3 text-vscode-fg/30" />
                )}
            </button>
            {isExpanded && (
                <div className="mt-2 text-[12px] text-vscode-fg/50 whitespace-pre-wrap leading-relaxed">
                    {renderContent(content.trim())}
                </div>
            )}
        </div>
    );
}


function ProgressBlock({ activities, toolCalls }: { activities: ActivityItem[]; toolCalls: ToolCall[] }) {
    const [isExpanded, setIsExpanded] = useState(true);
    const { postMessage } = useVSCodeApi();

    const renderActivity = (activity: ActivityItem, i: number) => {
        if (activity.type === 'analyze') {
            return (
                <InlineActivity
                    key={`act-${i}`}
                    type="analyzed"
                    filename={activity.file || ''}
                    lineRange={activity.lineRange}
                />
            );
        }
        if (activity.type === 'edit') {
            return (
                <InlineActivity
                    key={`act-${i}`}
                    type="edited"
                    filename={activity.file || ''}
                    onView={() => postMessage({ type: 'open_file', payload: { path: activity.file } })}
                />
            );
        }
        if (activity.type === 'search') {
            return (
                <InlineActivity
                    key={`act-${i}`}
                    type="searched"
                    filename={activity.query || 'search'}
                />
            );
        }
        return null;
    };

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
                {/* Render Activities for high-level summary */}
                {activities.map((activity, i) => renderActivity(activity, i))}

                {/* Render Tool Calls (Detailed Trace) */}
                <div className="flex flex-col gap-0.5 mt-1 opacity-100 transition-opacity">
                    {toolCalls.map((tool, i) => {
                        if (tool.name === 'update_todos') return null;
                        return <ToolRow key={`tool-${i}`} tool={tool} />;
                    })}
                </div>
            </div>
        </div>
    );
}

import { DiffLine, FileDiff } from '../diff/DiffView';

function ToolRow({ tool }: { tool: ToolCall }) {
    const { postMessage } = useVSCodeApi();

    // Determine if this is an edit tool early for default expansion
    const isEditTool = tool.name === 'replace_file_content' || tool.name === 'write_to_file' || tool.name === 'write_file';

    // Auto-expand diff for completed edit tools (Kilo Code style)
    const [showDiff, setShowDiff] = useState(isEditTool && tool.status === 'completed');
    const [showDropdown, setShowDropdown] = useState(false);

    const getToolInfo = (name: string) => {
        const n = name.toLowerCase();
        if (n.includes('read') || n.includes('view_file')) return { label: 'READ', color: 'text-blue-400' };
        if (n.includes('edit') || n.includes('write') || n.includes('replace')) return { label: 'EDIT', color: 'text-green-400' };
        if (n.includes('search') || n.includes('grep')) return { label: 'FIND', color: 'text-yellow-400' };
        if (n.includes('run') || n.includes('exec') || n.includes('command')) return { label: 'CMD', color: 'text-purple-400' };
        if (n.includes('list') || n.includes('ls')) return { label: 'LIST', color: 'text-gray-400' };
        return { label: 'TOOL', color: 'text-vscode-fg/30' };
    };

    const info = getToolInfo(tool.name);
    let path: string | undefined;
    let args: any = {};
    try {
        args = typeof tool.arguments === 'string' ? JSON.parse(tool.arguments) : tool.arguments;
        path = args.path || args.TargetFile || args.AbsolutePath || args.file || args.TargetContent;
    } catch (e) { }

    const fileName = (typeof path === 'string' && path.includes('/') ? path.split('/').pop() : path) || null;
    const diff = isEditTool ? getToolDiff(tool, args) : null;

    // Kilo Code style: prominent edit header
    if (isEditTool && diff) {
        const isOverwrite = tool.name === 'write_to_file' || tool.name === 'write_file';
        const headerText = isOverwrite ? 'Ricochet wants to overwrite this file' : 'Ricochet wants to edit this file';

        return (
            <div className="flex flex-col gap-1 my-2">
                {/* Kilo Code style header */}
                <button
                    onClick={() => setShowDiff(!showDiff)}
                    className="flex items-center gap-2 px-3 py-2 bg-[#252526] hover:bg-[#2a2d2e] rounded-lg border border-[#333] transition-colors"
                >
                    <Edit3 className={`w-4 h-4 ${isOverwrite ? 'text-red-400' : 'text-green-400'}`} />
                    <span className="text-xs font-medium text-[#ccc]">
                        {headerText}
                    </span>
                    <span className="ml-auto flex items-center gap-2">
                        {diff && (
                            <span className="text-[10px] text-vscode-fg/50">
                                <span className="text-green-400">+{diff.hunks.flat().filter(l => l.type === 'add').length}</span>
                                {' '}
                                <span className="text-red-400">-{diff.hunks.flat().filter(l => l.type === 'remove').length}</span>
                            </span>
                        )}
                        {showDiff ? <ChevronUp className="w-4 h-4 text-vscode-fg/50" /> : <ChevronDown className="w-4 h-4 text-vscode-fg/50" />}
                    </span>
                </button>

                {/* File name row */}
                <div className="flex items-center gap-2 px-3 py-1.5 bg-[#1e1e1e] rounded-lg border border-[#333]">
                    <FileText className="w-3.5 h-3.5 text-blue-400" />
                    <button
                        className="text-xs text-blue-400 hover:text-blue-300 hover:underline font-mono truncate"
                        onClick={() => {
                            if (typeof path === 'string') {
                                postMessage({ type: 'open_file', payload: { path: path.split('#')[0] } });
                            }
                        }}
                    >
                        {fileName || path}
                    </button>
                    {tool.status === 'running' && <span className="w-2 h-2 rounded-full bg-blue-500 animate-pulse ml-auto" />}
                    {tool.status === 'completed' && <span className="w-2 h-2 rounded-full bg-green-500 ml-auto" title="Applied" />}
                </div>

                {/* Diff content */}
                {showDiff && (
                    <div className="animate-fade-in">
                        <DiffView
                            diffs={[diff]}
                            onApprove={tool.status === 'completed' ? undefined : () => {
                                // Collapse diff after approval
                                setShowDiff(false);
                            }}
                            onReject={tool.status === 'completed' ? undefined : () => {
                                // Collapse diff after rejection
                                setShowDiff(false);
                            }}
                            onViewInVSCode={(p) => postMessage({ type: 'open_file', payload: { path: p } })}
                        />
                    </div>
                )}
            </div>
        );
    }

    // Antigravity style: Terminal block for commands
    const isCommandTool = tool.name === 'run_command' || tool.name === 'execute_command' || tool.name.includes('command');
    const command = args.command || args.CommandLine || args.cmd;

    if (isCommandTool && command) {
        // Using showDiff state from component level (no useState here!)
        const output = tool.result || '';
        const exitCode = args.exitCode ?? (tool.status === 'completed' ? 0 : null);
        const hasOutput = output && output.trim().length > 0;

        return (
            <div className="my-2 rounded-lg border border-[#333] bg-[#0d1117]">
                {/* Terminal Header - clickable to expand output */}
                <button
                    onClick={() => hasOutput && setShowDiff(!showDiff)}
                    className={`w-full flex items-center gap-2 px-3 py-2 bg-[#161b22] rounded-t-lg transition-colors ${hasOutput ? 'hover:bg-[#1c2128] cursor-pointer' : 'cursor-default'} ${!hasOutput ? 'rounded-b-lg' : 'border-b border-[#333]'}`}
                >
                    <Terminal className="w-3.5 h-3.5 text-purple-400" />
                    <span className="text-[10px] text-[#8b949e] font-mono">$</span>
                    <code className="text-xs text-[#c9d1d9] font-mono flex-1 truncate text-left" title={command}>
                        {command.length > 60 ? command.slice(0, 60) + '...' : command}
                    </code>
                    {tool.status === 'running' && (
                        <span className="w-2 h-2 rounded-full bg-yellow-500 animate-pulse" />
                    )}
                    {tool.status === 'completed' && (
                        <span className={`text-[10px] font-mono ${exitCode === 0 ? 'text-green-400' : 'text-red-400'}`}>
                            Exit {exitCode}
                        </span>
                    )}
                    {hasOutput && (
                        showDiff ?
                            <ChevronUp className="w-3.5 h-3.5 text-[#8b949e]" /> :
                            <ChevronDown className="w-3.5 h-3.5 text-[#8b949e]" />
                    )}
                </button>

                {/* Terminal Output - plain text, no syntax highlighting */}
                {showDiff && hasOutput && (
                    <div className="px-3 py-2 max-h-40 overflow-auto bg-[#0d1117]">
                        <pre className="text-[11px] font-mono text-[#8b949e] whitespace-pre-wrap break-all">
                            {output}
                        </pre>
                    </div>
                )}

                {/* Terminal Footer with Always Proceed dropdown - always visible */}
                {hasOutput && (
                    <div className="flex items-center justify-between px-3 py-1.5 bg-[#161b22] border-t border-[#333] rounded-b-lg relative">
                        <span className="text-[10px] text-[#484f58] font-mono">
                            Ran terminal command
                        </span>
                        <div className="relative">
                            <button
                                onClick={(e) => { e.stopPropagation(); setShowDropdown(!showDropdown); }}
                                className="flex items-center gap-1 px-2 py-1 text-[10px] text-[#8b949e] hover:text-white hover:bg-white/10 rounded transition-colors"
                            >
                                Always Proceed
                                <ChevronDown className="w-3 h-3" />
                            </button>
                            {showDropdown && (
                                <>
                                    {/* Backdrop to close */}
                                    <div
                                        className="fixed inset-0 z-[9998]"
                                        onClick={() => setShowDropdown(false)}
                                    />
                                    {/* Dropdown menu - positioned above with high z-index */}
                                    <div className="absolute right-0 bottom-full mb-1 w-48 bg-[#1c2128] border border-[#333] rounded-lg shadow-2xl z-[9999] overflow-hidden">
                                        <button
                                            onClick={() => {
                                                setShowDropdown(false);
                                                postMessage({ type: 'set_auto_approve', payload: { commands: false } });
                                            }}
                                            className="w-full px-3 py-2 text-left text-xs text-[#c9d1d9] hover:bg-[#30363d] flex flex-col"
                                        >
                                            <span className="font-medium">Request Review</span>
                                            <span className="text-[10px] text-[#8b949e]">Always ask for permission</span>
                                        </button>
                                        <button
                                            onClick={() => {
                                                setShowDropdown(false);
                                                postMessage({ type: 'set_auto_approve', payload: { commands: true } });
                                            }}
                                            className="w-full px-3 py-2 text-left text-xs text-[#c9d1d9] hover:bg-[#30363d] flex flex-col"
                                        >
                                            <span className="font-medium">Always Proceed</span>
                                            <span className="text-[10px] text-[#8b949e]">Always run terminal commands</span>
                                        </button>
                                    </div>
                                </>
                            )}
                        </div>
                    </div>
                )}
            </div>
        );
    }

    // Regular tool row for non-edit, non-command tools
    return (
        <div className="flex flex-col gap-1 group/tool">
            <div className="flex items-center gap-2 text-[9px] font-mono whitespace-nowrap overflow-hidden">
                <span className="w-1 h-1 rounded-full bg-vscode-fg/20" />
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
                {tool.status === 'completed' && <div className="w-1 h-1 rounded-full bg-green-500/30 ml-auto mr-1" />}
            </div>
        </div>
    );
}

function getToolDiff(tool: ToolCall, args: any): FileDiff | null {
    if (tool.name === 'replace_file_content') {
        if (!args.TargetContent || !args.ReplacementContent) return null;

        // Naive diff generation
        const targetLines = args.TargetContent.split('\n');
        const replacementLines = args.ReplacementContent.split('\n');
        const startLine = args.StartLine || 1;

        const hunks: DiffLine[] = [
            ...targetLines.map((l: string, i: number) => ({ type: 'remove' as const, content: l, lineNumber: startLine + i })),
            ...replacementLines.map((l: string, i: number) => ({ type: 'add' as const, content: l, lineNumber: startLine + i })) // Line numbers for adds are approximate here
        ];

        return {
            path: args.TargetFile,
            operation: 'modify',
            hunks: [hunks]
        };
    }

    if (tool.name === 'write_to_file' || tool.name === 'write_file') {
        // Support both write_to_file and write_file tools
        const content = args.CodeContent || args.content;
        const filePath = args.TargetFile || args.path;
        if (!content) return null;
        return {
            path: filePath,
            operation: args.Overwrite ? 'modify' : 'create',
            hunks: [[
                ...content.split('\n').map((l: string, i: number) => ({ type: 'add' as const, content: l, lineNumber: i + 1 }))
            ]]
        };
    }
    return null;
}





function parseContent(content: string) {
    let thinking = "";
    let body = "";
    let lastIndex = 0;

    // Regular expression for <thinking> blocks (handles open/streaming ones too)
    // Supports both <thinking> and <think> (DeepSeek style)
    const thinkingRegex = /<(?:thinking|think)>([\s\S]*?)(?:<\/(?:thinking|think)>|$)/g;
    let match;

    while ((match = thinkingRegex.exec(content)) !== null) {
        // Append prefix text to body
        body += content.substring(lastIndex, match.index);

        // Append thinking content
        const blockContent = match[1].trim();
        if (blockContent) {
            thinking += (thinking ? "\n\n" : "") + blockContent;
        }

        // Update lastIndex to end of match
        lastIndex = match.index + match[0].length;

        // Check if this block is unclosed (streaming)
        // If it doesn't end with a generic closing tag, it's the last one
        const isClosed = match[0].endsWith('</thinking>') || match[0].endsWith('</think>');
        if (!isClosed) break;
    }

    // Append remaining text
    body += content.substring(lastIndex);

    // Detect artifacts (implementation_plan.md, walkthrough.md, task.md)
    const artifacts: Array<{
        type: 'walkthrough' | 'implementation_plan' | 'task' | 'other';
        title: string;
        summary: string;
        path: string;
    }> = [];

    // Regex to find markdown links to artifacts
    const artifactRegex = /\[(.*?)\]\((.*?(\.md))\)/g;
    let artMatch;
    while ((artMatch = artifactRegex.exec(content)) !== null) {
        const path = artMatch[2];
        const fileName = path.split('/').pop()?.toLowerCase() || "";

        if (fileName.includes('implementation_plan') || fileName === 'plan.md') {
            artifacts.push({
                type: 'implementation_plan',
                title: 'Implementation Plan',
                summary: 'Technical breakdown and proposed changes.',
                path
            });
        } else if (fileName.includes('walkthrough')) {
            artifacts.push({
                type: 'walkthrough',
                title: 'Walkthrough',
                summary: 'Summary of completed work and verification results.',
                path
            });
        } else if (fileName.includes('task')) {
            artifacts.push({
                type: 'task',
                title: 'Task Tracking',
                summary: 'Checklist of tasks and their current progress.',
                path
            });
        }
    }

    // Deduplicate artifacts by path
    const uniqueArtifacts = Array.from(new Map(artifacts.map(a => [a.path, a])).values());

    return {
        thinking: thinking.trim() || null,
        body: body.trim(),
        artifacts: uniqueArtifacts
    };
}

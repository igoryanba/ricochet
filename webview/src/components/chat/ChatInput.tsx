import { useRef, useEffect, KeyboardEvent, useState } from 'react';
import { Send, Mic, Square, ChevronDown, FileCode, Plus, StopCircle } from 'lucide-react';
import { useAudioRecorder } from '../../hooks/useAudioRecorder';
import { FileSearchResult } from '../../hooks/useChat';
import { ModelPickerModal } from './ModelPickerModal';
import { EtherStatus } from './EtherPanel';

interface ChatInputProps {
    value: string;
    onChange: (value: string) => void;
    onSend: (value?: string) => void;
    onCancel?: () => void;
    isLoading?: boolean;
    placeholder?: string;
    currentMode?: string;
    onModeChange?: (mode: string) => void;
    onOpenSettings?: () => void;
    fileResults?: FileSearchResult[];
    searchFiles?: (query: string) => void;
    liveStatus?: EtherStatus;
    onToggleLiveMode?: () => void;
}

const MODES = [
    { slug: 'code', name: 'Code' },
    { slug: 'architect', name: 'Architect' },
    { slug: 'ask', name: 'Ask' },
    { slug: 'messenger', name: 'Messenger' },
];

// Default model, will be updated from settings
const DEFAULT_MODEL = { id: 'gemini-3-flash', name: 'Gemini 3 Flash', provider: 'gemini' };

const COMMANDS = [
    { command: '/clear', description: 'Clear chat history' },
    { command: '/reset', description: 'Reset session context' },
    { command: '/mode code', description: 'Switch to Code mode' },
    { command: '/mode architect', description: 'Switch to Architect mode' },
    { command: '/mode ask', description: 'Switch to Ask mode' },
];

function VoiceIcon({ className }: { className?: string }) {
    return (
        <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg" className={className}>
            <path d="M17 7C17 6.44772 16.5523 6 16 6C15.4477 6 15 6.44772 15 7V25C15 25.5523 15.4477 26 16 26C16.5523 26 17 25.5523 17 25V7Z" fill="currentColor" />
            <path d="M21 10C21 9.44772 20.5523 9 20 9C19.4477 9 19 9.44772 19 10V22C19 22.5523 19.4477 23 20 23C20.5523 23 21 22.5523 21 22V10Z" fill="currentColor" />
            <path d="M13 10C13 9.44772 12.5523 9 12 9C11.4477 9 11 10C11 10V22C11 22.5523 11.4477 23 12 23C12.5523 23 13 22.5523 13 22V10C13 10 13 10 13 10Z" fill="currentColor" />
            <path d="M25 14C25 13.4477 24.5523 13 24 13C23.4477 13 23 13.4477 23 14V18C23 18.5523 23.4477 19 24 19C24.5523 19 25 18.5523 25 18V14Z" fill="currentColor" />
            <path d="M9 14C9 13.4477 8.55228 13 8 13C7.44772 13 7 13.4477 7 14V18C7 18.5523 7.44772 19 8 19C8.55228 19 9 18.5523 9 18V14Z" fill="currentColor" />
        </svg>
    );
}

/**
 * Chat input with integrated bottom toolbar — competitor-style layout.
 * Matches Roo-Code pattern: Mode + Provider selectors under input.
 */
export function ChatInput(props: ChatInputProps) {
    const {
        value,
        onChange,
        onSend,
        onCancel,
        isLoading = false,
        placeholder = 'Type your message...',
        currentMode = 'code',
        onModeChange,
        // onOpenSettings, // Unused
        fileResults = [],
        searchFiles,
        liveStatus,
        onToggleLiveMode
    } = props;

    // Derived state for Ether
    const isLiveMode = liveStatus?.enabled ?? false;
    const isRemoteProcessing = isLiveMode && (liveStatus?.stage === 'processing' || liveStatus?.stage === 'receiving');
    const isRemoteControl = isLiveMode && liveStatus?.stage !== 'idle';

    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const { isRecording, toggleRecording } = useAudioRecorder();
    const [showModeMenu, setShowModeMenu] = useState(false);
    const [showModelMenu, setShowModelMenu] = useState(false);
    const [currentModel, setCurrentModel] = useState(DEFAULT_MODEL);
    const [images, setImages] = useState<string[]>([]); // Base64 strings
    const [isPlanMode, setIsPlanMode] = useState(false); // Plan/Act toggle

    const closeAllMenus = () => {
        setShowModeMenu(false);
        setShowModelMenu(false);
        setShowFileMenu(false);
        setShowCommandMenu(false);
    };

    // Context System State
    const [contextFiles, setContextFiles] = useState<FileSearchResult[]>([]);
    const [showFileMenu, setShowFileMenu] = useState(false);

    // Slash Command State
    const [showCommandMenu, setShowCommandMenu] = useState(false);
    const [filteredCommands, setFilteredCommands] = useState(COMMANDS);

    // Auto-resize textarea on value change
    useEffect(() => {
        const textarea = textareaRef.current;
        if (textarea) {
            textarea.style.height = 'auto';
            const newHeight = Math.min(textarea.scrollHeight, 200);
            textarea.style.height = `${newHeight} px`;
        }
    }, [value]);

    /* Regex for detecting @mentions at end of word */
    const MENTION_REGEX = /@([\w\-\.\/]*)$/;

    const handleInputChange = (newValue: string) => {
        onChange(newValue);

        // Slash Command Trigger
        if (newValue.startsWith('/')) {
            const query = newValue.substring(1).toLowerCase();
            const matches = COMMANDS.filter(c => c.command.startsWith('/' + query));
            if (matches.length > 0) {
                setFilteredCommands(matches);
                setShowCommandMenu(true);
                setShowFileMenu(false);
                return;
            } else {
                setShowCommandMenu(false);
            }
        } else {
            setShowCommandMenu(false);
        }

        // Context Menu Trigger Logic
        const match = newValue.match(MENTION_REGEX);
        if (match && searchFiles) {
            const query = match[1];
            searchFiles(query);
            setShowFileMenu(true);
        } else {
            setShowFileMenu(false);
        }
    };


    const addContextFile = (file: FileSearchResult) => {
        // Replace @query with nothing or file name? 
        // Competitors usually replace the whole match with a pill, OR just add to context list and clear the query.
        // We will add to context list and remove the @query text.

        const match = value.match(MENTION_REGEX);
        if (match) {
            const matchIndex = match.index!;
            const newValue = value.substring(0, matchIndex) + value.substring(matchIndex + match[0].length);
            onChange(newValue);
        }

        if (!contextFiles.find(f => f.path === file.path)) {
            setContextFiles(prev => [...prev, file]);
        }
        setShowFileMenu(false);
        textareaRef.current?.focus();
    };

    const selectCommand = (command: string) => {
        onChange(command);
        setShowCommandMenu(false);
        textareaRef.current?.focus();
    };

    const removeContextFile = (path: string) => {
        setContextFiles(prev => prev.filter(f => f.path !== path));
    };

    const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            if (showCommandMenu && filteredCommands.length > 0) {
                e.preventDefault();
                selectCommand(filteredCommands[0].command);
                return;
            }
            if (showFileMenu && fileResults && fileResults.length > 0) {
                e.preventDefault();
                addContextFile(fileResults[0]);
                return;
            }

            e.preventDefault();
            if (!isLoading && (value.trim() || images.length > 0 || contextFiles.length > 0)) {
                let messageToSend = value;

                if (contextFiles.length > 0) {
                    const contextString = '\n\nContext Files:\n' + contextFiles.map(f => `@${f.path} `).join('\n');
                    messageToSend += contextString;
                    // Update UI to show what was sent? 
                    // Or just clear? 
                    onChange(messageToSend);
                }

                // TODO: Pass images to onSend logic (requires protocol update)
                onSend(messageToSend);

                setImages([]);
                setContextFiles([]);
            }
        }
    };

    const handleSend = () => {
        if (!isLoading && (value.trim() || images.length > 0 || contextFiles.length > 0)) {
            let messageToSend = value;

            // Prepend Plan Mode tag
            if (isPlanMode) {
                messageToSend = `[PLAN MODE] ${messageToSend} `;
            }

            if (contextFiles.length > 0) {
                const contextString = '\n\nContext Files:\n' + contextFiles.map(f => `@${f.path} `).join('\n');
                messageToSend += contextString;
                onChange(messageToSend);
            }
            onSend(messageToSend);
            setImages([]);
            setContextFiles([]);
        }
    };

    // Image handling
    const handlePaste = (e: React.ClipboardEvent) => {
        const items = e.clipboardData.items;
        for (const item of items) {
            if (item.type.indexOf('image') === 0) {
                const blob = item.getAsFile();
                if (blob) {
                    const reader = new FileReader();
                    reader.onload = (event) => {
                        if (event.target?.result) {
                            setImages(prev => [...prev, event.target!.result as string]);
                        }
                    };
                    reader.readAsDataURL(blob);
                }
            }
        }
    };

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        const files = e.dataTransfer.files;
        for (const file of files) {
            if (file.type.startsWith('image/')) {
                const reader = new FileReader();
                reader.onload = (event) => {
                    if (event.target?.result) {
                        setImages(prev => [...prev, event.target!.result as string]);
                    }
                };
                reader.readAsDataURL(file);
            }
        }
    };

    const removeImage = (index: number) => {
        setImages(prev => prev.filter((_, i) => i !== index));
    };

    const currentModeData = MODES.find(m => m.slug === currentMode) || MODES[0];

    return (
        <div className="w-full relative">
            {/* Overlays / Modals (MUST BE FIRST FOR Z-INDEX) */}
            {(showModeMenu || showModelMenu || showCommandMenu || showFileMenu) && (
                <div
                    className="fixed inset-0 z-[9998] bg-black/20 backdrop-blur-[2px]"
                    onClick={closeAllMenus}
                />
            )}

            {showModelMenu && (
                <ModelPickerModal
                    isOpen={showModelMenu}
                    onClose={() => setShowModelMenu(false)}
                    currentModel={currentModel}
                    onSelectModel={(model) => {
                        setCurrentModel(model);
                        setShowModelMenu(false);
                    }}
                    currentMode={isPlanMode ? 'plan' : 'act'}
                    onModeChange={(mode) => setIsPlanMode(mode === 'plan')}
                />
            )}

            {/* Context/Image Chips */}
            {(images.length > 0 || contextFiles.length > 0) && (
                <div className="flex gap-2 overflow-x-auto pb-2 px-1">
                    {contextFiles.map((file) => (
                        <div key={file.path} className="flex items-center gap-1.5 px-2 py-1 bg-vscode-badge-background text-vscode-badge-foreground rounded text-[10px] whitespace-nowrap border border-vscode-border/50 shrink-0 uppercase font-black">
                            <span className="opacity-40">[F]</span>
                            <span className="max-w-[120px] truncate" title={file.path}>{file.name}</span>
                            <button onClick={() => removeContextFile(file.path)} className="hover:bg-black/10 rounded px-1 ml-1 opacity-40 hover:opacity-100">
                                X
                            </button>
                        </div>
                    ))}
                    {images.map((img, index) => (
                        <div key={index} className="relative group shrink-0">
                            <img src={img} alt="Preview" className="h-12 w-12 object-cover rounded-md border border-vscode-border/50" />
                            <button onClick={() => removeImage(index)} className="absolute -top-1.5 -right-1.5 bg-vscode-input-background text-vscode-fg rounded-full px-1 text-[8px] font-black border border-vscode-border shadow-sm">
                                X
                            </button>
                        </div>
                    ))}
                </div>
            )}

            {/* Menus (Command/File) */}
            {showCommandMenu && filteredCommands.length > 0 && (
                <div className="absolute bottom-full left-0 mb-2 w-full max-h-[200px] bg-[#1e1e1e] border border-white/10 rounded-xl shadow-[0_10px_30px_rgba(0,0,0,0.5)] overflow-hidden z-[9999]">
                    <div className="px-2 py-1.5 text-[10px] uppercase text-white/30 font-bold bg-white/5 border-b border-white/5">Commands</div>
                    <div className="overflow-y-auto max-h-[160px]">
                        {filteredCommands.map((cmd) => (
                            <button
                                key={cmd.command}
                                onClick={() => selectCommand(cmd.command)}
                                className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-white/5 text-white/80 transition-colors"
                            >
                                <span className="font-mono font-bold text-blue-400">{cmd.command}</span>
                                <span className="truncate text-xs opacity-50 flex-1 text-right">{cmd.description}</span>
                            </button>
                        ))}
                    </div>
                </div>
            )}

            {showFileMenu && fileResults && fileResults.length > 0 && (
                <div className="absolute bottom-full left-0 mb-2 w-full max-h-[200px] bg-[#1e1e1e] border border-white/10 rounded-xl shadow-[0_10px_30px_rgba(0,0,0,0.5)] overflow-hidden z-[9999]">
                    <div className="px-2 py-1.5 text-[10px] uppercase text-white/30 font-bold bg-white/5 border-b border-white/5">Suggested Files</div>
                    <div className="overflow-y-auto max-h-[160px]">
                        {fileResults.map((file) => (
                            <button
                                key={file.path}
                                onClick={() => addContextFile(file)}
                                className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-white/5 text-white/80 transition-colors"
                            >
                                <FileCode className="w-4 h-4 opacity-70 shrink-0" />
                                <div className="flex flex-col min-w-0">
                                    <span className="truncate font-medium">{file.name}</span>
                                    <span className="truncate text-xs opacity-50">{file.path}</span>
                                </div>
                            </button>
                        ))}
                    </div>
                </div>
            )}

            {/* Main Input Area */}
            <div
                className={`
                    relative group flex flex-col rounded-xl transition-all duration-300
                    ${isLiveMode ? 'ether-active animate-ether-glow' : 'bg-white/[0.03]'}
                    ${isPlanMode ? 'ring-1 ring-orange-500/20' : ''}
                    ${isRemoteControl ? 'border-blue-400/50 shadow-[0_0_10px_rgba(59,130,246,0.1)]' : ''}
                    hover:bg-white/[0.05] focus-within:bg-white/[0.05]
                `}
            >
                {/* Remote Control Overlay */}
                {isRemoteControl && (
                    <div className="absolute inset-0 z-10 bg-vscode-editor-background/80 backdrop-blur-[1px] flex flex-col items-center justify-center text-center p-4 rounded-xl">
                        <div className="flex items-center gap-2 text-blue-400 font-medium mb-1">
                            <VoiceIcon className="w-5 h-5 animate-spin" />
                            <span>Remote session active...</span>
                        </div>
                        <p className="text-xs text-vscode-fg/50">
                            Check {liveStatus?.connectedVia} for details
                        </p>
                    </div>
                )}
                <textarea
                    ref={textareaRef}
                    value={value}
                    onChange={(e) => handleInputChange(e.target.value)}
                    onKeyDown={handleKeyDown}
                    onPaste={handlePaste}
                    onDrop={handleDrop}
                    placeholder={isRecording ? '🎙️ Listening...' : (isLiveMode ? '🔵 Ether active — control via messenger or type here...' : placeholder)}
                    disabled={isLoading || isRemoteProcessing}
                    rows={1}
                    className={`
                        w-full resize-none py-3 px-3 rounded-t-xl
                        text-vscode-input-foreground text-sm
                        focus:outline-none 
                        bg-transparent
                        placeholder:text-vscode-fg/20
                        disabled:opacity-50 transition-colors
                        min-h-[60px] max-h-[200px]
                        ${isLoading || isRemoteProcessing ? 'cursor-not-allowed opacity-50' : ''}
                    `}
                />

                <div className="flex items-center justify-between gap-2 px-2 pb-2 mt-auto">
                    <div className="flex items-center gap-0.5">
                        <button
                            onClick={() => {
                                const input = document.createElement('input');
                                input.type = 'file';
                                input.multiple = true;
                                input.onchange = (e) => {
                                    const files = (e.target as HTMLInputElement).files;
                                    if (files) {
                                        Array.from(files).forEach(file => {
                                            if (file.type.startsWith('image/')) {
                                                const reader = new FileReader();
                                                reader.onload = () => setImages(prev => [...prev, reader.result as string]);
                                                reader.readAsDataURL(file);
                                            } else {
                                                const reader = new FileReader();
                                                reader.onload = (ev) => {
                                                    const content = ev.target?.result as string;
                                                    const textBlock = `\n\n[FILE: ${file.name}]\n\`\`\`\n${content}\n\`\`\`\n`;
                                                    onChange(value + textBlock);
                                                };
                                                reader.readAsText(file);
                                            }
                                        });
                                    }
                                };
                                input.click();
                            }}
                            className="p-1.5 text-vscode-fg/30 hover:text-white/80 hover:bg-white/5 rounded-md transition-all active:scale-95"
                            title="Attach files"
                        >
                            <Plus className="w-4 h-4" />
                        </button >

                        <div className="flex items-center gap-1 relative">
                            <button
                                onClick={() => setShowModeMenu(!showModeMenu)}
                                className="inline-flex items-center gap-1.5 px-2 py-1.5 text-xs font-bold text-vscode-fg/40 hover:text-white/80 hover:bg-white/5 rounded-md transition-all"
                            >
                                <span className="uppercase tracking-widest text-[9px]">{currentModeData.name}</span>
                                <ChevronDown className="w-3 h-3 opacity-30" />
                            </button>

                            {showModeMenu && (
                                <div className="absolute bottom-full left-0 mb-2 min-w-[160px] bg-[#1e1e1e] rounded-xl shadow-[0_10px_30px_rgba(0,0,0,0.5)] border border-white/10 overflow-hidden z-[9999]">
                                    <div className="p-1 px-2 py-1.5 text-[9px] uppercase tracking-widest text-white/20 font-bold border-b border-white/5 text-center">Mode</div>
                                    <div className="p-1">
                                        {MODES.map(mode => (
                                            <button
                                                key={mode.slug}
                                                onClick={() => { onModeChange?.(mode.slug); setShowModeMenu(false); }}
                                                className={`
                                                    w-full flex items-center px-3 py-2 text-left text-xs rounded-lg transition-all
                                                    ${mode.slug === currentMode
                                                        ? 'bg-[#0e639c] text-white'
                                                        : 'text-white/60 hover:bg-white/5 hover:text-white'}
                                                `}
                                            >
                                                {mode.name}
                                            </button>
                                        ))}
                                    </div>
                                </div>
                            )}

                            <button
                                onClick={() => setShowModelMenu(!showModelMenu)}
                                className="inline-flex items-center gap-1.5 px-2 py-1.5 text-xs font-medium text-vscode-fg/30 hover:text-white/80 hover:bg-white/5 rounded-md transition-all max-w-[140px]"
                            >
                                <span className="truncate text-[10px]">{currentModel.name}</span>
                                <ChevronDown className="w-3 h-3 opacity-30 ml-auto" />
                            </button>

                            <div className="flex items-center bg-white/[0.03] rounded-lg overflow-hidden ml-1 p-0.5 border border-white/5">
                                <button
                                    onClick={() => setIsPlanMode(true)}
                                    className={`px-2.5 py-1 text-[9px] font-bold uppercase tracking-widest transition-all rounded-md ${isPlanMode ? 'bg-[#cc7832] text-white' : 'text-vscode-fg/30 hover:text-vscode-fg/50'}`}
                                >
                                    Plan
                                </button>
                                <button
                                    onClick={() => setIsPlanMode(false)}
                                    className={`px-2.5 py-1 text-[9px] font-bold uppercase tracking-widest transition-all rounded-md ${!isPlanMode ? 'bg-[#0e639c] text-white' : 'text-vscode-fg/30 hover:text-vscode-fg/50'}`}
                                >
                                    Act
                                </button>
                            </div>
                        </div>
                    </div >

                    <div className="flex items-center gap-0.5">
                        <button
                            onClick={() => {
                                onToggleLiveMode?.();
                            }}
                            className={`p-1.5 rounded-md transition-all ${isLiveMode ? 'bg-blue-600 text-white shadow-[0_0_15px_rgba(37,99,235,0.4)] scale-110' : 'text-vscode-fg/30 hover:text-white/80 hover:bg-white/5'} active:scale-95`}
                            title="Live Mode / Messenger Pairing"
                        >
                            <VoiceIcon className={`w-4 h-4 ${isLiveMode ? 'animate-pulse' : ''}`} />
                        </button>
                        <button
                            onClick={toggleRecording}
                            disabled={isLoading}
                            className={`p-1.5 rounded-md transition-all ${isRecording ? 'bg-red-500/30 text-red-500' : 'text-vscode-fg/30 hover:text-white/80 hover:bg-white/5'} active:scale-95`}
                            title="Voice Input"
                        >
                            {isRecording ? <Square className="w-3.5 h-3.5" /> : <Mic className="w-3.5 h-3.5" />}
                        </button>
                        {isLoading || isRemoteProcessing ? (
                            <button
                                onClick={() => onCancel?.()}
                                className="p-1.5 rounded-md transition-all bg-red-600 text-white hover:bg-red-500 shadow-lg shadow-red-500/30 active:scale-95 animate-pulse"
                                title={isRemoteProcessing ? "Take control" : "Stop generation"}
                            >
                                <StopCircle className="w-4 h-4" />
                            </button>
                        ) : (
                            <button
                                onClick={handleSend}
                                disabled={!value.trim() && images.length === 0 && contextFiles.length === 0}
                                className={`p-1.5 rounded-md transition-all ${(value.trim() || images.length > 0 || contextFiles.length > 0) ? 'bg-blue-600 text-white hover:bg-blue-500 shadow-lg shadow-blue-500/20 active:scale-95' : 'text-vscode-fg/10 pointer-events-none'}`}
                                title="Send message"
                            >
                                <Send className="w-4 h-4" />
                            </button>
                        )}
                    </div>
                </div >
            </div >
        </div >
    );
}

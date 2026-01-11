import { useState, useEffect, useCallback } from 'react';
import { History, RotateCcw, Save, Loader2, Check, ChevronDown, ChevronUp } from 'lucide-react';
import { useVSCodeApi } from '../../hooks/useVSCodeApi';

interface Checkpoint {
    hash: string;
    message: string;
    timestamp: number;
}

interface CheckpointPanelProps {
    taskId?: string;
    onRestore?: (hash: string) => void;
}

/**
 * CheckpointPanel - Shows checkpoint history with save/restore controls
 * Like Cline's checkpoint feature - undo/redo workspace state
 */
export function CheckpointPanel({ taskId = 'default', onRestore }: CheckpointPanelProps) {
    const [checkpoints, setCheckpoints] = useState<Checkpoint[]>([]);
    const [isExpanded, setIsExpanded] = useState(false);
    const [isLoading, setIsLoading] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const [isInitialized, setIsInitialized] = useState(false);
    const [baseHash, setBaseHash] = useState('');
    const [lastSaved, setLastSaved] = useState<string | null>(null);

    const { postMessage, onMessage } = useVSCodeApi();

    // Initialize checkpoints on mount
    useEffect(() => {
        postMessage({
            type: 'checkpoint_init',
            payload: { taskId }
        });
    }, [taskId, postMessage]);

    // Listen for checkpoint responses
    useEffect(() => {
        const unsubscribe = onMessage((message) => {
            switch (message.type) {
                case 'checkpoint_initialized':
                    setIsInitialized(true);
                    setBaseHash((message.payload as { baseHash: string }).baseHash);
                    // Immediately request list
                    postMessage({ type: 'checkpoint_list' });
                    break;

                case 'checkpoint_saved':
                    setIsSaving(false);
                    const saved = message.payload as { hash: string; message: string };
                    if (saved.hash) {
                        setLastSaved(saved.hash);
                        setCheckpoints(prev => [...prev, {
                            hash: saved.hash,
                            message: saved.message || 'Checkpoint',
                            timestamp: Date.now()
                        }]);
                        // Clear success indicator after 2s
                        setTimeout(() => setLastSaved(null), 2000);
                    }
                    break;

                case 'checkpoint_restored':
                    setIsLoading(false);
                    onRestore?.((message.payload as { hash: string }).hash);
                    break;

                case 'checkpoint_list':
                    const list = message.payload as { checkpoints: string[]; baseHash: string };
                    setCheckpoints(list.checkpoints.map((hash, i) => ({
                        hash,
                        message: `Checkpoint ${i + 1}`,
                        timestamp: Date.now() - (list.checkpoints.length - i) * 60000
                    })));
                    setBaseHash(list.baseHash);
                    break;
            }
        });

        return () => { unsubscribe(); };
    }, [onMessage, postMessage, onRestore]);

    const handleSave = useCallback(() => {
        if (isSaving) return;
        setIsSaving(true);
        postMessage({
            type: 'checkpoint_save',
            payload: { message: `Manual checkpoint at ${new Date().toLocaleTimeString()}` }
        });
    }, [isSaving, postMessage]);

    const handleRestore = useCallback((hash: string) => {
        if (isLoading) return;
        setIsLoading(true);
        postMessage({
            type: 'checkpoint_restore',
            payload: { hash }
        });
    }, [isLoading, postMessage]);

    if (!isInitialized) {
        return (
            <div className="flex items-center gap-2 px-3 py-2 text-xs text-[#888]">
                <Loader2 className="w-3 h-3 animate-spin" />
                <span>Initializing checkpoints...</span>
            </div>
        );
    }

    return (
        <div className="border-b border-[#333]">
            {/* Header - always visible */}
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#2a2d2e] transition-colors"
            >
                <div className="flex items-center gap-2">
                    <History className="w-4 h-4 text-[#888]" />
                    <span className="text-xs font-medium text-[#ccc]">
                        Checkpoints
                    </span>
                    <span className="text-xs text-[#666]">
                        ({checkpoints.length})
                    </span>
                </div>
                <div className="flex items-center gap-2">
                    {/* Quick save button */}
                    <button
                        onClick={(e) => {
                            e.stopPropagation();
                            handleSave();
                        }}
                        disabled={isSaving}
                        className="p-1 text-[#888] hover:text-[#0e639c] hover:bg-[#0e639c20] rounded transition-colors disabled:opacity-50"
                        title="Save checkpoint"
                    >
                        {isSaving ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : lastSaved ? (
                            <Check className="w-3.5 h-3.5 text-green-400" />
                        ) : (
                            <Save className="w-3.5 h-3.5" />
                        )}
                    </button>
                    {isExpanded ? (
                        <ChevronUp className="w-4 h-4 text-[#666]" />
                    ) : (
                        <ChevronDown className="w-4 h-4 text-[#666]" />
                    )}
                </div>
            </button>

            {/* Expanded checkpoint list */}
            {isExpanded && (
                <div className="px-3 pb-3 space-y-1 max-h-48 overflow-y-auto">
                    {checkpoints.length === 0 ? (
                        <div className="text-xs text-[#666] py-2 text-center">
                            No checkpoints yet. Changes are saved automatically.
                        </div>
                    ) : (
                        <>
                            {/* Base (initial) state */}
                            <div className="flex items-center justify-between px-2 py-1.5 rounded bg-[#2a2d2e]">
                                <div className="flex items-center gap-2">
                                    <span className="text-xs text-[#888]">Initial state</span>
                                    <span className="text-[10px] text-[#555] font-mono">
                                        {baseHash.slice(0, 7)}
                                    </span>
                                </div>
                                <button
                                    onClick={() => handleRestore(baseHash)}
                                    disabled={isLoading}
                                    className="p-1 text-[#888] hover:text-orange-400 hover:bg-orange-400/10 rounded transition-colors"
                                    title="Restore to initial state"
                                >
                                    <RotateCcw className="w-3 h-3" />
                                </button>
                            </div>

                            {/* Checkpoint list */}
                            {checkpoints.map((cp, idx) => (
                                <div
                                    key={cp.hash}
                                    className="flex items-center justify-between px-2 py-1.5 rounded hover:bg-[#2a2d2e] group"
                                >
                                    <div className="flex items-center gap-2 min-w-0">
                                        <span className="text-xs text-[#ccc] truncate">
                                            {cp.message}
                                        </span>
                                        <span className="text-[10px] text-[#555] font-mono flex-shrink-0">
                                            {cp.hash.slice(0, 7)}
                                        </span>
                                    </div>
                                    <button
                                        onClick={() => handleRestore(cp.hash)}
                                        disabled={isLoading}
                                        className="p-1 text-[#888] hover:text-orange-400 hover:bg-orange-400/10 rounded transition-colors opacity-0 group-hover:opacity-100"
                                        title={`Restore to checkpoint ${idx + 1}`}
                                    >
                                        {isLoading ? (
                                            <Loader2 className="w-3 h-3 animate-spin" />
                                        ) : (
                                            <RotateCcw className="w-3 h-3" />
                                        )}
                                    </button>
                                </div>
                            ))}
                        </>
                    )}
                </div>
            )}
        </div>
    );
}

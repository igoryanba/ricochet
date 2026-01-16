import { useState } from 'react';
import { ChevronDown, ChevronUp, FileCode, CheckCircle, Loader2 } from 'lucide-react';

interface TaskProgressProps {
    taskName: string;
    summary: string;
    mode?: 'planning' | 'execution' | 'verification';
    steps: string[];
    filesEdited: string[];
    isActive?: boolean;
}

const modeConfig = {
    planning: { label: 'PLANNING', color: 'bg-blue-500/20 text-blue-400 border-blue-500/30' },
    execution: { label: 'EXECUTION', color: 'bg-green-500/20 text-green-400 border-green-500/30' },
    verification: { label: 'VERIFICATION', color: 'bg-purple-500/20 text-purple-400 border-purple-500/30' },
};

/**
 * TaskProgressCard — Antigravity-style structured task progress display.
 * Shows task header, summary, files edited, and collapsible progress updates.
 */
export function TaskProgressCard({
    taskName,
    summary,
    mode = 'execution',
    steps,
    filesEdited,
    isActive = true
}: TaskProgressProps) {
    const [expanded, setExpanded] = useState(false);
    const config = modeConfig[mode];

    return (
        <div className="rounded-xl border border-white/10 bg-gradient-to-br from-white/5 to-white/[0.02] backdrop-blur-sm overflow-hidden">
            {/* Header */}
            <div className="p-4 border-b border-white/5">
                <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                        {isActive ? (
                            <Loader2 className="w-4 h-4 text-blue-400 animate-spin" />
                        ) : (
                            <CheckCircle className="w-4 h-4 text-green-400" />
                        )}
                        <h3 className="font-semibold text-white">{taskName}</h3>
                    </div>
                    <span className={`px-2 py-0.5 text-[10px] font-bold uppercase rounded border ${config.color}`}>
                        {config.label}
                    </span>
                </div>
                <p className="text-sm text-white/70">{summary}</p>
            </div>

            {/* Files Edited */}
            {filesEdited.length > 0 && (
                <div className="px-4 py-2 border-b border-white/5 bg-white/[0.02]">
                    <div className="text-[10px] uppercase text-white/40 mb-1.5 font-medium">Files Edited</div>
                    <div className="flex flex-wrap gap-1.5">
                        {filesEdited.map((file, i) => {
                            const basename = file.split('/').pop() || file;
                            const ext = basename.split('.').pop();
                            return (
                                <span
                                    key={i}
                                    className="inline-flex items-center gap-1 px-2 py-0.5 text-xs bg-white/5 rounded-md text-white/80 border border-white/10"
                                >
                                    <FileCode className="w-3 h-3 text-white/40" />
                                    <span className="text-blue-400">{ext}</span>
                                    <span>{basename}</span>
                                </span>
                            );
                        })}
                    </div>
                </div>
            )}

            {/* Progress Updates */}
            {steps.length > 0 && (
                <div className="px-4 py-2">
                    <button
                        onClick={() => setExpanded(!expanded)}
                        className="flex items-center justify-between w-full text-left group"
                    >
                        <span className="text-[10px] uppercase text-white/40 font-medium">
                            Progress Updates ({steps.length})
                        </span>
                        <span className="text-xs text-white/30 group-hover:text-white/50 flex items-center gap-1">
                            {expanded ? 'Collapse' : 'Expand'}
                            {expanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                        </span>
                    </button>

                    {expanded && (
                        <ol className="mt-2 space-y-1 text-sm text-white/60">
                            {steps.map((step, i) => (
                                <li key={i} className="flex items-start gap-2">
                                    <span className="text-white/30 font-mono text-xs min-w-[1.5rem]">{i + 1}</span>
                                    <span>{step}</span>
                                </li>
                            ))}
                        </ol>
                    )}
                </div>
            )}
        </div>
    );
}

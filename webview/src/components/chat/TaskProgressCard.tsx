import { useState } from 'react';
import { ChevronDown, ChevronUp, FileCode, CheckCircle, Loader2, FileText, ArrowUpDown, ClipboardList } from 'lucide-react';

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
 * 
 * Structure matches Antigravity:
 * - Bold task name header
 * - Summary text
 * - Files Edited badges
 * - Numbered Progress Updates with Collapse all / Expand all
 */
export function TaskProgressCard({
    taskName,
    summary,
    mode = 'execution',
    steps,
    filesEdited,
    isActive = true
}: TaskProgressProps) {
    const [expanded, setExpanded] = useState(true); // Default expanded like Antigravity

    // Deduplicate files
    const uniqueFiles = [...new Set(filesEdited)];

    // Get file icon based on extension/type
    const getFileIcon = (filename: string) => {
        const name = filename.toLowerCase();
        if (name.includes('walkthrough')) return <ArrowUpDown className="w-3 h-3 text-purple-400" />;
        if (name.includes('task')) return <ClipboardList className="w-3 h-3 text-yellow-400" />;
        return <FileCode className="w-3 h-3 text-blue-400" />;
    };

    return (
        <div className="mb-4 rounded-lg border border-[#333] bg-[#1e1e1e] overflow-hidden shadow-sm">
            {/* Header - Task Name + Summary */}
            <div className="px-4 py-3 border-b border-[#333]">
                <div className="flex items-center gap-2 mb-1">
                    {isActive ? (
                        <Loader2 className="w-4 h-4 text-blue-400 animate-spin flex-shrink-0" />
                    ) : (
                        <CheckCircle className="w-4 h-4 text-green-500 flex-shrink-0" />
                    )}
                    <h3 className="font-semibold text-white text-sm flex-1">{taskName}</h3>
                    {/* Mode badge */}
                    <span className={`px-2 py-0.5 text-[9px] font-bold uppercase rounded ${modeConfig[mode].color}`}>
                        {modeConfig[mode].label}
                    </span>
                </div>
                {summary && (
                    <p className="text-[12px] text-[#8b949e] leading-relaxed pl-6">{summary}</p>
                )}
            </div>

            {/* Files Edited - Horizontal badges */}
            {uniqueFiles.length > 0 && (
                <div className="px-4 py-2 border-b border-[#333] bg-[#161b22]">
                    <div className="text-[10px] uppercase text-[#484f58] mb-2 font-medium tracking-wide">Files Edited</div>
                    <div className="flex flex-wrap gap-2">
                        {uniqueFiles.map((file, i) => {
                            const basename = file.split('/').pop() || file;
                            const ext = basename.split('.').pop();
                            return (
                                <span
                                    key={i}
                                    className="inline-flex items-center gap-1.5 px-2 py-1 text-[11px] bg-[#21262d] rounded-md text-[#8b949e] border border-[#30363d] hover:bg-[#30363d] transition-colors cursor-pointer"
                                >
                                    {getFileIcon(basename)}
                                    <span className="text-blue-400 font-mono">{ext}</span>
                                    <span className="text-[#c9d1d9]">{basename}</span>
                                </span>
                            );
                        })}
                    </div>
                </div>
            )}

            {/* Progress Updates - Numbered list with toggle */}
            {steps.length > 0 && (
                <div className="px-4 py-2">
                    <button
                        onClick={() => setExpanded(!expanded)}
                        className="flex items-center justify-between w-full text-left group py-1"
                    >
                        <span className="text-[10px] uppercase text-[#484f58] font-medium tracking-wide">
                            Progress Updates
                        </span>
                        <span className="text-[10px] text-[#6e7681] group-hover:text-[#8b949e] flex items-center gap-1 transition-colors">
                            {expanded ? 'Collapse all' : 'Expand all'}
                            {expanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                        </span>
                    </button>

                    {expanded && (
                        <ol className="mt-2 space-y-1.5">
                            {steps.map((step, i) => (
                                <li key={i} className="flex items-start gap-2 text-[12px]">
                                    <span className="text-[#6e7681] font-mono min-w-[1.25rem] text-right">{i + 1}</span>
                                    <span className="text-[#c9d1d9]">{step}</span>
                                </li>
                            ))}
                        </ol>
                    )}
                </div>
            )}
        </div>
    );
}

// ============================================================================
// ArtifactCard - For walkthrough, task, implementation_plan artifacts
// ============================================================================

interface ArtifactCardProps {
    type: 'walkthrough' | 'implementation_plan' | 'task' | 'other';
    title: string;
    summary: string;
    path?: string;
    onOpen?: () => void;
}

const artifactConfig = {
    walkthrough: {
        icon: <ArrowUpDown className="w-4 h-4 text-purple-400" />,
        label: 'Walkthrough'
    },
    implementation_plan: {
        icon: <FileText className="w-4 h-4 text-blue-400" />,
        label: 'Implementation Plan'
    },
    task: {
        icon: <ClipboardList className="w-4 h-4 text-yellow-400" />,
        label: 'Task'
    },
    other: {
        icon: <FileText className="w-4 h-4 text-gray-400" />,
        label: 'Document'
    }
};

/**
 * ArtifactCard — Antigravity-style artifact display card.
 * Shows artifact type icon, title, summary, and Open button.
 * Used for walkthrough, implementation_plan, task files.
 */
export function ArtifactCard({
    type,
    title,
    summary,
    onOpen
}: ArtifactCardProps) {
    const config = artifactConfig[type] || artifactConfig.other;

    return (
        <div className="my-3 rounded-lg border border-[#333] bg-[#1e1e1e] overflow-hidden">
            <div className="flex items-start gap-3 px-4 py-3">
                {/* Icon + Title + Summary */}
                <div className="flex-shrink-0 mt-0.5">
                    {config.icon}
                </div>
                <div className="flex-1 min-w-0">
                    <div className="text-[13px] font-medium text-[#c9d1d9]">{title}</div>
                    <p className="text-[11px] text-[#8b949e] mt-0.5 line-clamp-2">{summary}</p>
                </div>

                {/* Open button */}
                {onOpen && (
                    <button
                        onClick={onOpen}
                        className="flex-shrink-0 px-3 py-1 text-[11px] font-medium text-[#c9d1d9] bg-[#21262d] hover:bg-[#30363d] rounded-md border border-[#30363d] transition-colors"
                    >
                        Open
                    </button>
                )}
            </div>

            {/* Feedback buttons */}
            <div className="flex items-center justify-end gap-2 px-4 py-2 border-t border-[#333] bg-[#161b22]">
                <span className="text-[10px] text-[#6e7681]">Good</span>
                <button className="p-1 hover:bg-[#30363d] rounded transition-colors" title="Good">
                    <svg className="w-3.5 h-3.5 text-[#6e7681]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5" />
                    </svg>
                </button>
                <span className="text-[10px] text-[#6e7681]">Bad</span>
                <button className="p-1 hover:bg-[#30363d] rounded transition-colors" title="Bad">
                    <svg className="w-3.5 h-3.5 text-[#6e7681]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14H5.236a2 2 0 01-1.789-2.894l3.5-7A2 2 0 018.736 3h4.018a2 2 0 01.485.06l3.76.94m-7 10v5a2 2 0 002 2h.096c.5 0 .905-.405.905-.904 0-.715.211-1.413.608-2.008L17 13V4m-7 10h2m5-10h2a2 2 0 012 2v6a2 2 0 01-2 2h-2.5" />
                    </svg>
                </button>
            </div>
        </div>
    );
}

// ============================================================================
// InlineActivity - Lightweight activity indicator between text
// ============================================================================

interface InlineActivityProps {
    type: 'analyzed' | 'edited' | 'searched';
    filename: string;
    lineRange?: string;
    onView?: () => void;
}

/**
 * InlineActivity — Antigravity-style lightweight activity item.
 * Shows: 📄 Analyzed filename#L1-100 or 📝 Edited filename [View]
 */
export function InlineActivity({
    type,
    filename,
    lineRange,
    onView
}: InlineActivityProps) {
    const basename = filename.split('/').pop() || filename;

    const iconClass = type === 'edited'
        ? 'text-green-400'
        : type === 'searched'
            ? 'text-yellow-400'
            : 'text-blue-400';

    const label = type === 'edited' ? 'Edited' : type === 'searched' ? 'Searched' : 'Analyzed';

    return (
        <div className="flex items-center gap-2 py-1 text-[12px] text-[#8b949e]">
            <FileText className={`w-3.5 h-3.5 ${iconClass}`} />
            <span className="text-[#6e7681]">{label}</span>
            <span className="text-blue-400 font-mono">{basename}</span>
            {lineRange && (
                <span className="text-[#484f58] text-[10px]">#{lineRange}</span>
            )}
            {onView && (
                <button
                    onClick={onView}
                    className="ml-auto text-[11px] text-[#8b949e] hover:text-white transition-colors"
                >
                    View
                </button>
            )}
        </div>
    );
}

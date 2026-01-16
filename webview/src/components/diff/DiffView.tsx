import { useState, useMemo } from 'react';
import { Check, X, ChevronDown, ChevronUp, Plus, Minus, Edit, FileEdit } from 'lucide-react';

export interface DiffLine {
    type: 'add' | 'remove' | 'context';
    content: string;
    lineNumber?: number;
}

export interface FileDiff {
    path: string;
    operation: 'create' | 'modify' | 'delete';
    hunks: DiffLine[][];
}

interface DiffViewProps {
    diffs: FileDiff[];
    onApprove?: () => void;
    onReject?: () => void;
    onViewInVSCode?: (path: string) => void;
    isLoading?: boolean;
}

/**
 * DiffView - Shows file changes in unified diff format
 * Like VSCode's diff view but inline in chat
 */
export function DiffView({ diffs, onApprove, onReject, onViewInVSCode, isLoading }: DiffViewProps) {
    const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set(diffs.map(d => d.path)));

    const toggleFile = (path: string) => {
        setExpandedFiles(prev => {
            const next = new Set(prev);
            if (next.has(path)) {
                next.delete(path);
            } else {
                next.add(path);
            }
            return next;
        });
    };

    const stats = useMemo(() => {
        let additions = 0;
        let deletions = 0;
        diffs.forEach(diff => {
            diff.hunks.forEach(hunk => {
                hunk.forEach(line => {
                    if (line.type === 'add') additions++;
                    if (line.type === 'remove') deletions++;
                });
            });
        });
        return { additions, deletions };
    }, [diffs]);

    return (
        <div className="border border-[#333] rounded-lg overflow-hidden bg-[#1e1e1e]">
            {/* Header */}
            <div className="flex items-center justify-between px-3 py-2 bg-[#252526] border-b border-[#333]">
                <div className="flex items-center gap-3">
                    <span className="text-xs font-medium text-[#ccc]">
                        {diffs.length} file{diffs.length !== 1 ? 's' : ''} changed
                    </span>
                    <div className="flex items-center gap-2 text-xs">
                        <span className="text-green-400">+{stats.additions}</span>
                        <span className="text-red-400">-{stats.deletions}</span>
                    </div>
                </div>
                <div className="flex items-center gap-1">
                    {onReject && (
                        <button
                            onClick={onReject}
                            disabled={isLoading}
                            className="flex items-center gap-1 px-2 py-1 text-xs bg-red-500/20 text-red-400 hover:bg-red-500/30 rounded transition-colors disabled:opacity-50"
                        >
                            <X className="w-3 h-3" />
                            Reject
                        </button>
                    )}
                    {onApprove && (
                        <button
                            onClick={onApprove}
                            disabled={isLoading}
                            className="flex items-center gap-1 px-2 py-1 text-xs bg-green-500/20 text-green-400 hover:bg-green-500/30 rounded transition-colors disabled:opacity-50"
                        >
                            <Check className="w-3 h-3" />
                            Apply
                        </button>
                    )}
                </div>
            </div>

            {/* File list */}
            <div className="max-h-96 overflow-y-auto">
                {diffs.map((diff) => (
                    <div key={diff.path} className="border-b border-[#333] last:border-b-0">
                        {/* File header */}
                        <button
                            onClick={() => toggleFile(diff.path)}
                            className="w-full flex items-center gap-2 px-3 py-2 hover:bg-[#2a2d2e] transition-colors"
                        >
                            {expandedFiles.has(diff.path) ? (
                                <ChevronDown className="w-3 h-3 text-[#666]" />
                            ) : (
                                <ChevronUp className="w-3 h-3 text-[#666]" />
                            )}
                            <FileIcon operation={diff.operation} />
                            <span className="text-xs text-[#ccc] font-mono truncate">
                                {diff.path}
                            </span>
                            <span className={`text-[10px] px-1.5 py-0.5 rounded ${diff.operation === 'create' ? 'bg-green-500/20 text-green-400' :
                                diff.operation === 'delete' ? 'bg-red-500/20 text-red-400' :
                                    'bg-yellow-500/20 text-yellow-400'
                                }`}>
                                {diff.operation}
                            </span>
                            {onViewInVSCode && (
                                <button
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        onViewInVSCode(diff.path);
                                    }}
                                    className="ml-auto p-1 text-[#888] hover:text-[#ccc] hover:bg-white/5 rounded transition-colors"
                                    title="Open native diff in VSCode"
                                >
                                    <FileEdit className="w-3 h-3" />
                                </button>
                            )}
                        </button>

                        {/* Diff content */}
                        {expandedFiles.has(diff.path) && (
                            <div className="bg-[#1a1a1a] font-mono text-xs overflow-x-auto">
                                {diff.hunks.map((hunk, hunkIdx) => (
                                    <div key={hunkIdx} className="border-t border-[#333]">
                                        {hunk.map((line, lineIdx) => (
                                            <div
                                                key={lineIdx}
                                                className={`flex ${line.type === 'add' ? 'bg-green-500/10' :
                                                    line.type === 'remove' ? 'bg-red-500/10' :
                                                        ''
                                                    }`}
                                            >
                                                <span className="w-10 flex-shrink-0 px-2 text-right text-[#555] select-none border-r border-[#333]">
                                                    {line.lineNumber || ''}
                                                </span>
                                                <span className={`w-4 flex-shrink-0 text-center select-none ${line.type === 'add' ? 'text-green-400' :
                                                    line.type === 'remove' ? 'text-red-400' :
                                                        'text-[#555]'
                                                    }`}>
                                                    {line.type === 'add' ? '+' : line.type === 'remove' ? '-' : ' '}
                                                </span>
                                                <span className={`flex-1 px-2 whitespace-pre ${line.type === 'add' ? 'text-green-300' :
                                                    line.type === 'remove' ? 'text-red-300' :
                                                        'text-[#aaa]'
                                                    }`}>
                                                    {line.content}
                                                </span>
                                            </div>
                                        ))}
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
}

function FileIcon({ operation }: { operation: 'create' | 'modify' | 'delete' }) {
    switch (operation) {
        case 'create':
            return <Plus className="w-3 h-3 text-green-400" />;
        case 'delete':
            return <Minus className="w-3 h-3 text-red-400" />;
        default:
            return <Edit className="w-3 h-3 text-yellow-400" />;
    }
}

/**
 * Parse unified diff string into structured format
 */
export function parseDiff(diffText: string): FileDiff[] {
    const files: FileDiff[] = [];
    const hunkRegex = /^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@/;

    const fileParts = diffText.split(/^diff --git/m).filter(Boolean);

    for (const part of fileParts) {
        const lines = part.split('\n');
        const headerLine = lines[0];
        const pathMatch = headerLine.match(/a\/(.*) b\/(.*)/);

        if (!pathMatch) continue;

        const path = pathMatch[2];
        let operation: 'create' | 'modify' | 'delete' = 'modify';

        if (part.includes('new file mode')) operation = 'create';
        if (part.includes('deleted file mode')) operation = 'delete';

        const hunks: DiffLine[][] = [];
        let currentHunk: DiffLine[] = [];
        let lineNum = 0;

        for (const line of lines) {
            const hunkMatch = line.match(hunkRegex);
            if (hunkMatch) {
                if (currentHunk.length) hunks.push(currentHunk);
                currentHunk = [];
                lineNum = parseInt(hunkMatch[3], 10);
                continue;
            }

            if (line.startsWith('+') && !line.startsWith('+++')) {
                currentHunk.push({ type: 'add', content: line.slice(1), lineNumber: lineNum++ });
            } else if (line.startsWith('-') && !line.startsWith('---')) {
                currentHunk.push({ type: 'remove', content: line.slice(1) });
            } else if (line.startsWith(' ')) {
                currentHunk.push({ type: 'context', content: line.slice(1), lineNumber: lineNum++ });
            }
        }

        if (currentHunk.length) hunks.push(currentHunk);
        files.push({ path, operation, hunks });
    }

    return files;
}

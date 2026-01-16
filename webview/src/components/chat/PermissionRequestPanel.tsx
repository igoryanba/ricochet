import React from 'react';

interface PermissionRequestPanelProps {
    request: {
        id: string;
        question: string;
    };
    onResponse: (id: string, answer: string) => void;
    inline?: boolean; // When true, renders as chat message style
}

export const PermissionRequestPanel: React.FC<PermissionRequestPanelProps> = ({ request, onResponse, inline = false }) => {
    // Parse the question to extract the command description if possible
    // Usually format is "Mode: X\n\nDo you ...\n\nWrite to file: ..."
    const q = request?.question || '';
    const lines = q.split('\n');
    const title = lines[0]; // Mode: ...
    const description = lines.slice(2).join('\n').trim();

    const containerClass = inline
        ? "py-4 px-6 bg-vscode-sideBar-background/50 border-y border-yellow-500/20 animate-fade-in"
        : "mx-4 mb-4 p-4 rounded-lg bg-vscode-input-background border border-vscode-focusBorder shadow-lg animate-in fade-in slide-in-from-bottom-2";

    return (
        <div className={containerClass}>
            <div className="flex items-start gap-3">
                <div className="text-yellow-400 mt-1">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                </div>
                <div className="flex-1">
                    <h3 className="text-sm font-semibold text-vscode-fg mb-1">{title || 'Permission Required'}</h3>
                    <div className="text-xs text-vscode-fg/80 mb-3 font-mono whitespace-pre-wrap">
                        {description || request.question}
                    </div>

                    <div className="flex gap-2">
                        <button
                            onClick={() => onResponse(request.id, 'Yes')}
                            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs rounded-md transition-colors"
                        >
                            Allow
                        </button>
                        <button
                            onClick={() => onResponse(request.id, 'Always Allow')}
                            className="px-3 py-1.5 bg-vscode-button-secondaryBackground hover:bg-vscode-button-secondaryHoverBackground text-vscode-button-secondaryForeground text-xs rounded-md transition-colors"
                        >
                            Always Allow
                        </button>
                        <button
                            onClick={() => onResponse(request.id, 'No')}
                            className="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs rounded-md transition-colors"
                        >
                            Deny
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};

import { Radio } from 'lucide-react';
import { useLiveMode } from '@hooks/useLiveMode';

/**
 * Live Mode toggle switch with status indicator.
 * Shows connection status when enabled.
 */
export function LiveModeSwitch() {
    const { isLiveMode, status, isLoading, toggleLiveMode } = useLiveMode();

    return (
        <button
            onClick={toggleLiveMode}
            disabled={isLoading}
            className={`
        flex items-center gap-2 px-2 py-1 rounded-md text-xs font-medium
        transition-all duration-200
        ${isLiveMode
                    ? 'bg-live-mode/20 text-live-mode border border-live-mode/50 live-mode-active'
                    : 'bg-vscode-input-bg text-vscode-fg hover:bg-vscode-button-hover border border-transparent'
                }
        ${isLoading ? 'opacity-50 cursor-wait' : 'cursor-pointer'}
      `}
            title={isLiveMode ? 'Disable Live Mode' : 'Enable Live Mode'}
        >
            <Radio
                className={`w-3.5 h-3.5 ${isLiveMode ? 'animate-pulse' : ''}`}
            />
            <span>Live</span>
            {isLiveMode && status.connectedVia && (
                <span className="text-[10px] opacity-70">
                    via {status.connectedVia}
                </span>
            )}
        </button>
    );
}
